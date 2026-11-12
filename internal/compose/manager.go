package compose

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/docker"
)

// DefaultBundledVersion is the default bundled static Docker Compose version.
const DefaultBundledVersion = "v2.40.3"

// DefaultDownloadTimeout is the HTTP timeout when downloading docker-compose.
const DefaultDownloadTimeout = 15 * time.Second

// ManagerConfig holds settings for resolving the docker compose binary.
type ManagerConfig struct {
	Bin        string // configured binary (e.g. "docker-compose" or "/usr/local/bin/docker-compose")
	DockerHost string // docker daemon host URL
	Mode       string // "auto", "bundled", "pinned"
	Version    string // "auto" or specific version like "v2.40.3"
	CacheDir   string // cache directory for downloaded binaries
}

// Manager resolves the appropriate compose binary and provides a Runner.
type Manager struct {
	cfg        ManagerConfig
	dockerCli  docker.Client
	httpClient *http.Client
	downloadURLFunc func(tag string, goos, goarch string) string

	mu          sync.Mutex
	resolvedBin string
	resolved    bool
	runner      *CommandRunner
}

// NewManager creates a new Manager instance.
func NewManager(cfg ManagerConfig, dockerCli docker.Client) *Manager {
	if cfg.Mode == "" {
		cfg.Mode = "auto"
	}
	if cfg.Version == "" {
		cfg.Version = "auto"
	}
	return &Manager{
		cfg:        cfg,
		dockerCli:  dockerCli,
		httpClient: &http.Client{Timeout: DefaultDownloadTimeout},
	}
}

// Resolve resolves the compose binary to be used.
// It caches the result for subsequent calls unless re-resolved.
func (m *Manager) Resolve(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.resolved {
		return m.resolvedBin, nil
	}

	bin := m.resolveBinary(ctx)
	m.resolvedBin = bin
	m.resolved = true
	m.runner = NewRunner(bin, m.cfg.DockerHost)
	return bin, nil
}

// Runner returns a Runner backed by the resolved binary.
// If not yet resolved, Resolve is called with a background context.
func (m *Manager) Runner() Runner {
	m.mu.Lock()
	if m.resolved && m.runner != nil {
		r := m.runner
		m.mu.Unlock()
		return r
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = m.Resolve(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runner
}

// resolveBinary implements the compose binary resolution logic.
func (m *Manager) resolveBinary(ctx context.Context) string {
	bundledBin := m.cfg.Bin
	if strings.TrimSpace(bundledBin) == "" {
		bundledBin = DefaultBin
	}

	// If user explicitly overrode ComposeBin (not "docker-compose", "/usr/local/bin/docker-compose", or "docker compose"),
	// or if mode is "bundled", use configured binary.
	mode := strings.ToLower(strings.TrimSpace(m.cfg.Mode))
	if mode == "bundled" {
		return bundledBin
	}

	isStandardDefault := bundledBin == "docker-compose" ||
		bundledBin == "/usr/local/bin/docker-compose" ||
		bundledBin == "docker compose"

	if !isStandardDefault {
		return bundledBin
	}

	// If mode is pinned to a specific version
	if mode == "pinned" {
		targetVersion := strings.TrimSpace(m.cfg.Version)
		if targetVersion == "" || targetVersion == "auto" {
			targetVersion = DefaultBundledVersion
		}
		if binPath, err := m.ensureComposeVersion(ctx, targetVersion, bundledBin); err == nil && binPath != "" {
			return binPath
		}
		return bundledBin
	}

	// Auto mode:
	// If ComposeVersion is explicitly set to a specific version (not "auto"), use that version
	if m.cfg.Version != "" && m.cfg.Version != "auto" {
		if binPath, err := m.ensureComposeVersion(ctx, m.cfg.Version, bundledBin); err == nil && binPath != "" {
			return binPath
		}
		return bundledBin
	}

	// Detect host compose version from existing containers
	detectedVersion := m.detectHostComposeVersion(ctx)
	if detectedVersion == "" {
		return bundledBin
	}

	bundledVer := m.detectBundledVersion(ctx, bundledBin)
	if bundledVer != "" && normalizeVersion(detectedVersion) == normalizeVersion(bundledVer) {
		return bundledBin
	}

	// Version detected and differs from bundled version: attempt download/cache
	if binPath, err := m.ensureComposeVersion(ctx, detectedVersion, bundledBin); err == nil && binPath != "" {
		return binPath
	}

	return bundledBin
}

// detectHostComposeVersion looks for label com.docker.compose.version on running/stopped containers.
func (m *Manager) detectHostComposeVersion(ctx context.Context) string {
	if m.dockerCli == nil {
		return ""
	}

	containers, err := m.dockerCli.ListContainers(ctx)
	if err != nil {
		slog.Debug("Failed to list containers for compose version detection", "error", err)
		return ""
	}

	for _, c := range containers {
		if v, ok := c.Labels[docker.ComposeVersionLabel]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// detectBundledVersion attempts to get the version of the bundled binary by running `<bin> version --short`
// or parsing `<bin> version`.
func (m *Manager) detectBundledVersion(ctx context.Context, bin string) string {
	parts := strings.Fields(bin)
	if len(parts) == 0 {
		return ""
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, parts[0], append(parts[1:], "version", "--short")...)
	out, err := cmd.Output()
	if err == nil {
		ver := strings.TrimSpace(string(out))
		if ver != "" {
			return ver
		}
	}

	// Fallback to standard version command
	cmd = exec.CommandContext(checkCtx, parts[0], append(parts[1:], "version")...)
	out, err = cmd.Output()
	if err == nil {
		// e.g. "Docker Compose version v2.40.3"
		fields := strings.Fields(string(out))
		for _, f := range fields {
			if strings.HasPrefix(f, "v") && len(f) > 1 {
				return f
			}
		}
	}

	return ""
}

// ensureComposeVersion checks if version is cached, or downloads it from GitHub releases.
func (m *Manager) ensureComposeVersion(ctx context.Context, version string, fallbackBin string) (string, error) {
	normVer := normalizeVersion(version)
	tag := "v" + normVer

	cacheDir := m.cfg.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "dokidoki", "bin")
	}

	cachedBinPath := filepath.Join(cacheDir, fmt.Sprintf("docker-compose-%s", tag))
	if info, err := os.Stat(cachedBinPath); err == nil && !info.IsDir() && (info.Mode()&0111) != 0 {
		return cachedBinPath, nil
	}

	// Download from GitHub releases
	downloadURL := ComposeDownloadURL(tag, runtime.GOOS, runtime.GOARCH)
	if m.downloadURLFunc != nil {
		downloadURL = m.downloadURLFunc(tag, runtime.GOOS, runtime.GOARCH)
	}
	if downloadURL == "" {
		slog.Warn("Unsupported OS/architecture for Docker Compose download", "os", runtime.GOOS, "arch", runtime.GOARCH)
		return fallbackBin, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		slog.Warn("Failed to create compose cache directory", "dir", cacheDir, "error", err)
		return fallbackBin, err
	}

	slog.Info("Downloading Docker Compose binary", "version", tag, "url", downloadURL)
	tmpPath := cachedBinPath + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		slog.Warn("Failed to create download request for Docker Compose", "url", downloadURL, "error", err)
		return fallbackBin, err
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		slog.Warn("Failed to download Docker Compose release; falling back to bundled binary", "url", downloadURL, "error", err)
		return fallbackBin, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("Docker Compose download returned non-200 status; falling back to bundled binary", "status", resp.StatusCode, "url", downloadURL)
		return fallbackBin, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		slog.Warn("Failed to create temporary compose binary file", "path", tmpPath, "error", err)
		return fallbackBin, err
	}

	_, copyErr := io.Copy(f, resp.Body)
	_ = f.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		slog.Warn("Failed to save downloaded compose binary", "error", copyErr)
		return fallbackBin, copyErr
	}

	if err := os.Rename(tmpPath, cachedBinPath); err != nil {
		_ = os.Remove(tmpPath)
		slog.Warn("Failed to finalize downloaded compose binary", "error", err)
		return fallbackBin, err
	}

	_ = os.Chmod(cachedBinPath, 0755)
	slog.Info("Successfully downloaded and cached Docker Compose", "version", tag, "path", cachedBinPath)
	return cachedBinPath, nil
}

// ComposeDownloadURL returns the GitHub release URL for the specified tag, OS, and GOARCH.
func ComposeDownloadURL(tag string, goos, goarch string) string {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	var arch string
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	case "arm":
		arch = "armv7"
	case "ppc64le":
		arch = "ppc64le"
	case "riscv64":
		arch = "riscv64"
	case "s390x":
		arch = "s390x"
	default:
		arch = goarch
	}

	var ext string
	if goos == "windows" {
		ext = ".exe"
	}

	return fmt.Sprintf("https://github.com/docker/compose/releases/download/%s/docker-compose-%s-%s%s", tag, goos, arch, ext)
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}
