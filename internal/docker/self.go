package docker

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strings"
)

var (
	procMountinfoPath = "/proc/self/mountinfo"
	procCgroupPath    = "/proc/self/cgroup"

	cgroupV2ScopeRegex = regexp.MustCompile(`docker-([0-9a-fA-F]{64})\.scope`)
	cgroupDockerRegex  = regexp.MustCompile(`(?:^|/)docker(?:/containers)?/([0-9a-fA-F]{64})(?:[^0-9a-fA-F]|$)`)
	mountinfoRegex     = regexp.MustCompile(`(?:^|/)docker/containers/([0-9a-fA-F]{64})(?:[^0-9a-fA-F]|$)`)
)

// isHexID verifies whether s consists of exactly 12 or 64 valid hex characters.
func isHexID(s string) bool {
	if len(s) != 12 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// extractContainerIDFromCgroup extracts a 64-char hex container ID from cgroup content.
// It handles cgroup v1 (.../docker/<64-char-hex>...) and cgroup v2 (...docker-<64-char-hex>.scope... or .../docker/<64-char-hex>...).
func extractContainerIDFromCgroup(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if match := cgroupV2ScopeRegex.FindStringSubmatch(line); len(match) > 1 {
			id := strings.ToLower(match[1])
			if isHexID(id) && len(id) == 64 {
				return id
			}
		}
		if match := cgroupDockerRegex.FindStringSubmatch(line); len(match) > 1 {
			id := strings.ToLower(match[1])
			if isHexID(id) && len(id) == 64 {
				return id
			}
		}
	}
	return ""
}

// extractContainerIDFromMountinfo extracts a 64-char hex container ID from container mount paths in mountinfo content.
// Extracts from paths like /docker/containers/<id>/... or /var/lib/docker/containers/<id>/...
func extractContainerIDFromMountinfo(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if match := mountinfoRegex.FindStringSubmatch(line); len(match) > 1 {
			id := strings.ToLower(match[1])
			if isHexID(id) && len(id) == 64 {
				return id
			}
		}
	}
	return ""
}

// IdentifySelf detects whether Dokidoki is running inside a Docker container
// and returns its full 64-character container ID, or empty string "" if running bare-metal on host.
func IdentifySelf(ctx context.Context, cli Client) string {
	// 1. Checks /proc/self/mountinfo using extractContainerIDFromMountinfo
	if data, err := os.ReadFile(procMountinfoPath); err == nil {
		if id := extractContainerIDFromMountinfo(string(data)); id != "" {
			// 3. If a 64-char ID is extracted, verifies it with cli.InspectContainer(ctx, id). If valid, returns the ID.
			if cli != nil {
				if inspect, err := cli.InspectContainer(ctx, id); err == nil && inspect.ID != "" {
					return inspect.ID
				}
			}
		}
	}

	// 2. Checks /proc/self/cgroup using extractContainerIDFromCgroup
	if data, err := os.ReadFile(procCgroupPath); err == nil {
		if id := extractContainerIDFromCgroup(string(data)); id != "" {
			// 3. If a 64-char ID is extracted, verifies it with cli.InspectContainer(ctx, id). If valid, returns the ID.
			if cli != nil {
				if inspect, err := cli.InspectContainer(ctx, id); err == nil && inspect.ID != "" {
					return inspect.ID
				}
			}
		}
	}

	// 4. If not found, checks os.Hostname(). If it is a 12 or 64 hex char ID, checks with cli.InspectContainer(ctx, hostname). If valid, returns inspect.ID.
	if hostname, err := os.Hostname(); err == nil {
		hostname = strings.TrimSpace(hostname)
		if isHexID(hostname) && cli != nil {
			if inspect, err := cli.InspectContainer(ctx, hostname); err == nil && inspect.ID != "" {
				return inspect.ID
			}
		}
	}

	// 5. Checks environment variable DOKIDOKI_CONTAINER_ID if set.
	if envID := strings.TrimSpace(os.Getenv("DOKIDOKI_CONTAINER_ID")); envID != "" {
		if cli != nil {
			if inspect, err := cli.InspectContainer(ctx, envID); err == nil && inspect.ID != "" {
				return inspect.ID
			}
		} else if isHexID(envID) {
			return envID
		}
	}

	// 6. If none match, returns "" (indicating Dokidoki is running bare-metal on host OS).
	return ""
}
