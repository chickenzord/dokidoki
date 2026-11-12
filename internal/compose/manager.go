package compose

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/docker"
	"gopkg.in/yaml.v3"
)

// DefaultBundledVersion is the default bundled static Docker Compose version.
const DefaultBundledVersion = "v2.40.3"

// DefaultDownloadTimeout is the HTTP timeout when downloading docker-compose.
const DefaultDownloadTimeout = 30 * time.Second

// ManagerConfig holds settings for resolving the docker compose binary.
type ManagerConfig struct {
	Bin             string // configured binary (e.g. "docker-compose" or "/usr/local/bin/docker-compose")
	DockerHost      string // docker daemon host URL
	Mode            string // "auto", "bundled", "pinned"
	Version         string // "auto" or specific version like "v2.40.3"
	CacheDir        string // cache directory for downloaded binaries
	SelfContainerID string // container ID of Dokidoki itself if containerized
}

type singleflightCall struct {
	wg  sync.WaitGroup
	val string
	err error
}

// Manager resolves the appropriate compose binary and provides a dynamic Runner.
type Manager struct {
	cfg             ManagerConfig
	dockerCli       docker.Client
	httpClient      *http.Client
	downloadURLFunc func(tag string, goos, goarch string) string

	mu          sync.Mutex
	resolvedBin string
	resolved    bool

	dlMu    sync.Mutex
	dlCalls map[string]*singleflightCall
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
		dlCalls:    make(map[string]*singleflightCall),
	}
}

// Resolve resolves the fallback/global compose binary to be used.
// It caches the result for subsequent calls.
func (m *Manager) Resolve(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.resolved {
		return m.resolvedBin, nil
	}

	bin := m.resolveBinary(ctx)
	m.resolvedBin = bin
	m.resolved = true
	return bin, nil
}

// Runner returns a dynamic Runner that resolves the compose binary per operation.
func (m *Manager) Runner() Runner {
	return NewMultiVersionRunner(m, m.cfg.DockerHost)
}

// Warmup scans host containers and self-container for distinct compose versions
// and downloads missing binaries in the background.
func (m *Manager) Warmup(ctx context.Context) {
	mode := strings.ToLower(strings.TrimSpace(m.cfg.Mode))
	if mode == "bundled" {
		return
	}

	versions := make(map[string]struct{})

	// If pinned, warm up the pinned version
	if mode == "pinned" {
		targetVersion := strings.TrimSpace(m.cfg.Version)
		if targetVersion == "" || targetVersion == "auto" {
			targetVersion = DefaultBundledVersion
		}
		versions[normalizeVersion(targetVersion)] = struct{}{}
	} else {
		// If auto with explicit version
		if m.cfg.Version != "" && m.cfg.Version != "auto" {
			versions[normalizeVersion(m.cfg.Version)] = struct{}{}
		}

		// Self-container version
		if selfVer := m.detectSelfComposeVersion(ctx); selfVer != "" {
			versions[normalizeVersion(selfVer)] = struct{}{}
		}

		// Host containers versions
		if m.dockerCli != nil {
			if containers, err := m.dockerCli.ListContainers(ctx); err == nil {
				for _, c := range containers {
					if v, ok := c.Labels[docker.ComposeVersionLabel]; ok && strings.TrimSpace(v) != "" {
						versions[normalizeVersion(v)] = struct{}{}
					}
				}
			}
		}
	}

	fallbackBin := m.bundledBin()
	for v := range versions {
		if ctx.Err() != nil {
			return
		}
		if _, err := m.ensureComposeVersion(ctx, v, fallbackBin, nil); err != nil {
			slog.Debug("Pre-warm download skipped or failed", "version", v, "error", err)
		}
	}
}

// ResolveForStack resolves the compose binary specifically for the given compose file / stack.
// out is an optional progress writer for user visibility.
func (m *Manager) ResolveForStack(ctx context.Context, composePath string, out io.Writer) string {
	bundledBin := m.bundledBin()

	mode := strings.ToLower(strings.TrimSpace(m.cfg.Mode))
	if mode == "bundled" {
		return bundledBin
	}

	if !m.isStandardDefaultBin(bundledBin) {
		return bundledBin
	}

	if mode == "pinned" {
		targetVersion := strings.TrimSpace(m.cfg.Version)
		if targetVersion == "" || targetVersion == "auto" {
			targetVersion = DefaultBundledVersion
		}
		if binPath, err := m.ensureComposeVersion(ctx, targetVersion, bundledBin, out); err == nil && binPath != "" {
			return binPath
		}
		return bundledBin
	}

	// Auto mode with explicit version configured
	if m.cfg.Version != "" && m.cfg.Version != "auto" {
		if binPath, err := m.ensureComposeVersion(ctx, m.cfg.Version, bundledBin, out); err == nil && binPath != "" {
			return binPath
		}
		return bundledBin
	}

	// Multi-step target version selection:
	// Step 1: Infer project name and find stack containers' highest semver
	projectName := InferProjectName(composePath)
	if projectName != "" && m.dockerCli != nil {
		if v := m.findHighestVersionForProject(ctx, projectName); v != "" {
			if binPath, err := m.ensureComposeVersion(ctx, v, bundledBin, out); err == nil && binPath != "" {
				return binPath
			}
			return bundledBin
		}
	}

	// Step 2: Check Dokidoki's self-container label com.docker.compose.version
	if selfVer := m.detectSelfComposeVersion(ctx); selfVer != "" {
		if binPath, err := m.ensureComposeVersion(ctx, selfVer, bundledBin, out); err == nil && binPath != "" {
			return binPath
		}
		return bundledBin
	}

	// Step 3: Highest semver across ALL containers on host
	if allVer := m.findHighestVersionAll(ctx); allVer != "" {
		if binPath, err := m.ensureComposeVersion(ctx, allVer, bundledBin, out); err == nil && binPath != "" {
			return binPath
		}
		return bundledBin
	}

	// Step 4: If none found, use bundled binary (DefaultBundledVersion).
	return bundledBin
}

// resolveBinary resolves a default compose binary.
func (m *Manager) resolveBinary(ctx context.Context) string {
	return m.ResolveForStack(ctx, "", nil)
}

func (m *Manager) bundledBin() string {
	bundledBin := m.cfg.Bin
	if strings.TrimSpace(bundledBin) == "" {
		bundledBin = DefaultBin
	}
	return bundledBin
}

func (m *Manager) isStandardDefaultBin(bundledBin string) bool {
	return bundledBin == "docker-compose" ||
		bundledBin == "/usr/local/bin/docker-compose" ||
		bundledBin == "docker compose"
}

// detectSelfComposeVersion inspects Dokidoki's container to check com.docker.compose.version.
func (m *Manager) detectSelfComposeVersion(ctx context.Context) string {
	if m.dockerCli == nil || m.cfg.SelfContainerID == "" {
		return ""
	}

	inspect, err := m.dockerCli.InspectContainer(ctx, m.cfg.SelfContainerID)
	if err != nil {
		return ""
	}

	if inspect.Config != nil && inspect.Config.Labels != nil {
		if v, ok := inspect.Config.Labels[docker.ComposeVersionLabel]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}

	return ""
}

// findHighestVersionForProject finds the highest semver among containers with com.docker.compose.project == project.
func (m *Manager) findHighestVersionForProject(ctx context.Context, project string) string {
	if m.dockerCli == nil || project == "" {
		return ""
	}

	containers, err := m.dockerCli.ListContainers(ctx)
	if err != nil {
		return ""
	}

	var highest string
	for _, c := range containers {
		if c.Labels[docker.ComposeProjectLabel] == project {
			if v, ok := c.Labels[docker.ComposeVersionLabel]; ok && strings.TrimSpace(v) != "" {
				val := strings.TrimSpace(v)
				if highest == "" || CompareSemver(val, highest) > 0 {
					highest = val
				}
			}
		}
	}
	return highest
}

// findHighestVersionAll finds highest semver across all containers on host.
func (m *Manager) findHighestVersionAll(ctx context.Context) string {
	if m.dockerCli == nil {
		return ""
	}

	containers, err := m.dockerCli.ListContainers(ctx)
	if err != nil {
		return ""
	}

	var highest string
	for _, c := range containers {
		if v, ok := c.Labels[docker.ComposeVersionLabel]; ok && strings.TrimSpace(v) != "" {
			val := strings.TrimSpace(v)
			if highest == "" || CompareSemver(val, highest) > 0 {
				highest = val
			}
		}
	}
	return highest
}

// ensureComposeVersion checks if version is cached, or downloads it using singleflight dedup.
func (m *Manager) ensureComposeVersion(ctx context.Context, version string, fallbackBin string, out io.Writer) (string, error) {
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

	// Dedup concurrent downloads using singleflight
	m.dlMu.Lock()
	if call, ok := m.dlCalls[tag]; ok {
		m.dlMu.Unlock()
		call.wg.Wait()
		if call.err == nil && call.val != "" {
			return call.val, nil
		}
		return fallbackBin, call.err
	}

	call := new(singleflightCall)
	call.wg.Add(1)
	m.dlCalls[tag] = call
	m.dlMu.Unlock()

	defer func() {
		m.dlMu.Lock()
		delete(m.dlCalls, tag)
		m.dlMu.Unlock()
	}()

	binPath, err := m.downloadComposeBinary(ctx, tag, cachedBinPath, fallbackBin, out)
	call.val = binPath
	call.err = err
	call.wg.Done()

	return binPath, err
}

func (m *Manager) downloadComposeBinary(ctx context.Context, tag, cachedBinPath, fallbackBin string, out io.Writer) (string, error) {
	// Re-check cache in case another caller just downloaded it
	if info, err := os.Stat(cachedBinPath); err == nil && !info.IsDir() && (info.Mode()&0111) != 0 {
		return cachedBinPath, nil
	}

	if out != nil {
		fmt.Fprintf(out, "[dokidoki] Preparing Docker Compose %s...\n", tag)
	}

	downloadURL := ComposeDownloadURL(tag, runtime.GOOS, runtime.GOARCH)
	if m.downloadURLFunc != nil {
		downloadURL = m.downloadURLFunc(tag, runtime.GOOS, runtime.GOARCH)
	}
	if downloadURL == "" {
		slog.Warn("Unsupported OS/architecture for Docker Compose download", "os", runtime.GOOS, "arch", runtime.GOARCH)
		return fallbackBin, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	cacheDir := filepath.Dir(cachedBinPath)
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

// Semver represents a parsed semantic version.
type Semver struct {
	Major int
	Minor int
	Patch int
	Pre   string
}

// ParseSemver parses a semver string (with or without leading 'v').
func ParseSemver(v string) (Semver, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return Semver{}, false
	}

	parts := strings.SplitN(v, "-", 2)
	pre := ""
	if len(parts) == 2 {
		pre = parts[1]
	}

	nums := strings.Split(parts[0], ".")
	if len(nums) == 0 {
		return Semver{}, false
	}

	var maj, min, patch int
	var err error

	maj, err = strconv.Atoi(nums[0])
	if err != nil {
		return Semver{}, false
	}

	if len(nums) > 1 {
		min, err = strconv.Atoi(nums[1])
		if err != nil {
			return Semver{}, false
		}
	}

	if len(nums) > 2 {
		patch, err = strconv.Atoi(nums[2])
		if err != nil {
			return Semver{}, false
		}
	}

	return Semver{Major: maj, Minor: min, Patch: patch, Pre: pre}, true
}

// CompareSemver compares two semver strings.
// Returns 1 if a > b, -1 if a < b, 0 if a == b.
func CompareSemver(a, b string) int {
	sa, oka := ParseSemver(a)
	sb, okb := ParseSemver(b)

	if !oka && !okb {
		return strings.Compare(a, b)
	}
	if !oka {
		return -1
	}
	if !okb {
		return 1
	}

	if sa.Major != sb.Major {
		if sa.Major > sb.Major {
			return 1
		}
		return -1
	}
	if sa.Minor != sb.Minor {
		if sa.Minor > sb.Minor {
			return 1
		}
		return -1
	}
	if sa.Patch != sb.Patch {
		if sa.Patch > sb.Patch {
			return 1
		}
		return -1
	}

	if sa.Pre == "" && sb.Pre != "" {
		return 1
	}
	if sa.Pre != "" && sb.Pre == "" {
		return -1
	}
	return strings.Compare(sa.Pre, sb.Pre)
}

type composeFileHeader struct {
	Name string `yaml:"name"`
}

// InferProjectName derives the Compose project name from compose file content or directory name.
func InferProjectName(composePath string) string {
	if composePath == "" {
		return ""
	}

	// Try reading compose file content
	if data, err := os.ReadFile(composePath); err == nil && len(data) > 0 {
		var hdr composeFileHeader
		if err := yaml.Unmarshal(data, &hdr); err == nil && strings.TrimSpace(hdr.Name) != "" {
			return strings.TrimSpace(hdr.Name)
		}
	}

	// Fallback to parent directory name
	dir := filepath.Dir(composePath)
	base := filepath.Base(dir)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return base
}
