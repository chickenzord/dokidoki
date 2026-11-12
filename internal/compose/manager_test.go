package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockDockerForManager struct {
	containers []types.Container
	inspects   map[string]types.ContainerJSON
}

func (m *mockDockerForManager) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}
func (m *mockDockerForManager) ListContainers(ctx context.Context) ([]types.Container, error) {
	return m.containers, nil
}
func (m *mockDockerForManager) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if m.inspects != nil {
		if c, ok := m.inspects[id]; ok {
			return c, nil
		}
	}
	return types.ContainerJSON{}, nil
}
func (m *mockDockerForManager) RestartContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}
func (m *mockDockerForManager) StartContainer(ctx context.Context, id string) error {
	return nil
}
func (m *mockDockerForManager) StopContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}
func (m *mockDockerForManager) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{}, nil
}
func (m *mockDockerForManager) WaitContainer(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return nil, nil
}
func (m *mockDockerForManager) ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockDockerForManager) RemoveContainer(ctx context.Context, id string, options container.RemoveOptions) error {
	return nil
}
func (m *mockDockerForManager) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockDockerForManager) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{}, nil
}
func (m *mockDockerForManager) Info(ctx context.Context) (system.Info, error) {
	return system.Info{}, nil
}
func (m *mockDockerForManager) Close() error {
	return nil
}

func TestComposeDownloadURL(t *testing.T) {
	urlAMD64 := ComposeDownloadURL("v2.40.3", "linux", "amd64")
	expectedAMD64 := "https://github.com/docker/compose/releases/download/v2.40.3/docker-compose-linux-x86_64"
	if urlAMD64 != expectedAMD64 {
		t.Errorf("expected %s, got %s", expectedAMD64, urlAMD64)
	}

	urlARM64 := ComposeDownloadURL("2.40.3", "linux", "arm64")
	expectedARM64 := "https://github.com/docker/compose/releases/download/v2.40.3/docker-compose-linux-aarch64"
	if urlARM64 != expectedARM64 {
		t.Errorf("expected %s, got %s", expectedARM64, urlARM64)
	}
}

func TestManagerResolve(t *testing.T) {
	t.Run("bundled mode returns configured binary", func(t *testing.T) {
		mgr := NewManager(ManagerConfig{
			Bin:  "my-docker-compose",
			Mode: "bundled",
		}, nil)

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bin != "my-docker-compose" {
			t.Errorf("expected my-docker-compose, got %s", bin)
		}
	})

	t.Run("user explicit override uses configured binary", func(t *testing.T) {
		mgr := NewManager(ManagerConfig{
			Bin:  "/custom/path/to/docker-compose",
			Mode: "auto",
		}, nil)

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bin != "/custom/path/to/docker-compose" {
			t.Errorf("expected /custom/path/to/docker-compose, got %s", bin)
		}
	})

	t.Run("auto mode with no containers returns default bin", func(t *testing.T) {
		mockDocker := &mockDockerForManager{containers: []types.Container{}}
		mgr := NewManager(ManagerConfig{
			Bin:  "docker-compose",
			Mode: "auto",
		}, mockDocker)

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bin != "docker-compose" {
			t.Errorf("expected docker-compose, got %s", bin)
		}
	})

	t.Run("auto mode with detected version in cache uses cached binary", func(t *testing.T) {
		cacheDir := t.TempDir()
		cachedBinary := filepath.Join(cacheDir, "docker-compose-v2.20.0")
		if err := os.WriteFile(cachedBinary, []byte("#!/bin/sh\necho 2.20.0\n"), 0755); err != nil {
			t.Fatalf("failed to write fake cached binary: %v", err)
		}

		mockDocker := &mockDockerForManager{
			containers: []types.Container{
				{
					Labels: map[string]string{
						"com.docker.compose.version": "2.20.0",
					},
				},
			},
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "auto",
			CacheDir: cacheDir,
		}, mockDocker)

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bin != cachedBinary {
			t.Errorf("expected %s, got %s", cachedBinary, bin)
		}
	})

	t.Run("auto mode with detected version downloads when not cached", func(t *testing.T) {
		cacheDir := t.TempDir()

		// Mock HTTP release server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("#!/bin/sh\necho downloaded-compose\n"))
		}))
		defer server.Close()

		mockDocker := &mockDockerForManager{
			containers: []types.Container{
				{
					Labels: map[string]string{
						"com.docker.compose.version": "v2.30.0",
					},
				},
			},
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "auto",
			CacheDir: cacheDir,
		}, mockDocker)

		mgr.httpClient = server.Client()
		mgr.downloadURLFunc = func(tag string, goos, goarch string) string {
			return server.URL
		}

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedBin := filepath.Join(cacheDir, "docker-compose-v2.30.0")
		if bin != expectedBin {
			t.Errorf("expected %s, got %s", expectedBin, bin)
		}

		// Verify cached file exists and is executable
		fi, err := os.Stat(expectedBin)
		if err != nil {
			t.Fatalf("failed to stat downloaded binary: %v", err)
		}
		if (fi.Mode() & 0111) == 0 {
			t.Errorf("expected downloaded binary to be executable")
		}
	})

	t.Run("auto mode falls back to bundled on download failure", func(t *testing.T) {
		cacheDir := t.TempDir()

		badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer badServer.Close()

		mockDocker := &mockDockerForManager{
			containers: []types.Container{
				{
					Labels: map[string]string{
						"com.docker.compose.version": "v2.35.0",
					},
				},
			},
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "auto",
			CacheDir: cacheDir,
		}, mockDocker)

		mgr.httpClient = badServer.Client()
		mgr.downloadURLFunc = func(tag string, goos, goarch string) string {
			return badServer.URL
		}

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("expected no error on fallback, got %v", err)
		}
		if bin != "docker-compose" {
			t.Errorf("expected fallback to docker-compose, got %s", bin)
		}
	})

	t.Run("pinned mode uses cached or downloaded version", func(t *testing.T) {
		cacheDir := t.TempDir()
		cachedBinary := filepath.Join(cacheDir, "docker-compose-v2.25.0")
		if err := os.WriteFile(cachedBinary, []byte("#!/bin/sh\necho 2.25.0\n"), 0755); err != nil {
			t.Fatalf("failed to write fake cached binary: %v", err)
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "pinned",
			Version:  "2.25.0",
			CacheDir: cacheDir,
		}, nil)

		bin, err := mgr.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bin != cachedBinary {
			t.Errorf("expected %s, got %s", cachedBinary, bin)
		}
	})

	t.Run("Runner returns working runner", func(t *testing.T) {
		mgr := NewManager(ManagerConfig{
			Bin:  "docker-compose",
			Mode: "bundled",
		}, nil)

		r := mgr.Runner()
		if r == nil {
			t.Fatal("expected non-nil Runner")
		}
	})
}

func TestSemverUtils(t *testing.T) {
	if CompareSemver("2.20.0", "v2.40.3") >= 0 {
		t.Errorf("expected 2.20.0 < v2.40.3")
	}
	if CompareSemver("v2.40.3", "v2.4.0") <= 0 {
		t.Errorf("expected v2.40.3 > v2.4.0")
	}
	if CompareSemver("v2.40.3", "2.40.3") != 0 {
		t.Errorf("expected v2.40.3 == 2.40.3")
	}
	if CompareSemver("v2.40.3", "v2.40.3-rc1") <= 0 {
		t.Errorf("expected release > rc")
	}
	if CompareSemver("invalid", "v2.40.3") >= 0 {
		t.Errorf("expected invalid < valid semver")
	}
}

func TestInferProjectName(t *testing.T) {
	tmpDir := t.TempDir()
	stackDir := filepath.Join(tmpDir, "my-stack")
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// 1. Inferred from dir
	composePath1 := filepath.Join(stackDir, "compose.yaml")
	if err := os.WriteFile(composePath1, []byte("services:\n  app:\n    image: alpine\n"), 0644); err != nil {
		t.Fatalf("failed to write compose: %v", err)
	}
	if name := InferProjectName(composePath1); name != "my-stack" {
		t.Errorf("expected my-stack, got %s", name)
	}

	// 2. Inferred from compose name attribute
	composePath2 := filepath.Join(stackDir, "custom.yaml")
	if err := os.WriteFile(composePath2, []byte("name: custom-project\nservices:\n  app:\n    image: alpine\n"), 0644); err != nil {
		t.Fatalf("failed to write compose: %v", err)
	}
	if name := InferProjectName(composePath2); name != "custom-project" {
		t.Errorf("expected custom-project, got %s", name)
	}

	// Empty path
	if name := InferProjectName(""); name != "" {
		t.Errorf("expected empty name for empty path, got %s", name)
	}
}

func TestTargetVersionSelectionSteps(t *testing.T) {
	cacheDir := t.TempDir()
	bin220 := filepath.Join(cacheDir, "docker-compose-v2.20.0")
	_ = os.WriteFile(bin220, []byte("#!/bin/sh\n"), 0755)
	bin230 := filepath.Join(cacheDir, "docker-compose-v2.30.0")
	_ = os.WriteFile(bin230, []byte("#!/bin/sh\n"), 0755)
	bin240 := filepath.Join(cacheDir, "docker-compose-v2.40.0")
	_ = os.WriteFile(bin240, []byte("#!/bin/sh\n"), 0755)

	t.Run("Step 1: stack containers match returns highest version among stack containers", func(t *testing.T) {
		mockDocker := &mockDockerForManager{
			containers: []types.Container{
				{
					Labels: map[string]string{
						"com.docker.compose.project": "stack-a",
						"com.docker.compose.version": "2.20.0",
					},
				},
				{
					Labels: map[string]string{
						"com.docker.compose.project": "stack-a",
						"com.docker.compose.version": "v2.30.0",
					},
				},
				{
					Labels: map[string]string{
						"com.docker.compose.project": "other-stack",
						"com.docker.compose.version": "v2.40.0",
					},
				},
			},
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "auto",
			CacheDir: cacheDir,
		}, mockDocker)

		composePath := "/opt/stacks/stack-a/compose.yaml"
		bin := mgr.ResolveForStack(context.Background(), composePath, nil)
		if bin != bin230 {
			t.Errorf("expected stack-a highest version %s, got %s", bin230, bin)
		}
	})

	t.Run("Step 2: no stack containers, uses self container version", func(t *testing.T) {
		mockDocker := &mockDockerForManager{
			containers: []types.Container{
				{
					ID: "dokidoki-self-id",
					Labels: map[string]string{
						"com.docker.compose.version": "v2.30.0",
					},
				},
				{
					Labels: map[string]string{
						"com.docker.compose.project": "other-stack",
						"com.docker.compose.version": "v2.40.0",
					},
				},
			},
			inspects: map[string]types.ContainerJSON{
				"dokidoki-self-id": {
					ContainerJSONBase: &types.ContainerJSONBase{ID: "dokidoki-self-id"},
					Config: &container.Config{
						Labels: map[string]string{
							"com.docker.compose.version": "v2.30.0",
						},
					},
				},
			},
		}

		mgr := NewManager(ManagerConfig{
			Bin:             "docker-compose",
			Mode:            "auto",
			CacheDir:        cacheDir,
			SelfContainerID: "dokidoki-self-id",
		}, mockDocker)

		composePath := "/opt/stacks/new-stack/compose.yaml"
		bin := mgr.ResolveForStack(context.Background(), composePath, nil)
		if bin != bin230 {
			t.Errorf("expected self-container version %s, got %s", bin230, bin)
		}
	})

	t.Run("Step 3: no stack containers, no self container, uses highest semver across all host containers", func(t *testing.T) {
		mockDocker := &mockDockerForManager{
			containers: []types.Container{
				{
					Labels: map[string]string{
						"com.docker.compose.project": "some-stack",
						"com.docker.compose.version": "2.20.0",
					},
				},
				{
					Labels: map[string]string{
						"com.docker.compose.project": "other-stack",
						"com.docker.compose.version": "v2.40.0",
					},
				},
			},
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "auto",
			CacheDir: cacheDir,
		}, mockDocker)

		composePath := "/opt/stacks/brand-new-stack/compose.yaml"
		bin := mgr.ResolveForStack(context.Background(), composePath, nil)
		if bin != bin240 {
			t.Errorf("expected host-wide highest version %s, got %s", bin240, bin)
		}
	})

	t.Run("Step 4: no containers at all, falls back to bundled binary", func(t *testing.T) {
		mockDocker := &mockDockerForManager{
			containers: []types.Container{},
		}

		mgr := NewManager(ManagerConfig{
			Bin:      "docker-compose",
			Mode:     "auto",
			CacheDir: cacheDir,
		}, mockDocker)

		composePath := "/opt/stacks/empty-host/compose.yaml"
		bin := mgr.ResolveForStack(context.Background(), composePath, nil)
		if bin != "docker-compose" {
			t.Errorf("expected bundled fallback docker-compose, got %s", bin)
		}
	})
}

func TestWarmup(t *testing.T) {
	cacheDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#!/bin/sh\n"))
	}))
	defer server.Close()

	mockDocker := &mockDockerForManager{
		containers: []types.Container{
			{
				Labels: map[string]string{
					"com.docker.compose.version": "v2.31.0",
				},
			},
			{
				Labels: map[string]string{
					"com.docker.compose.version": "v2.32.0",
				},
			},
		},
		inspects: map[string]types.ContainerJSON{
			"self-id": {
				Config: &container.Config{
					Labels: map[string]string{
						"com.docker.compose.version": "v2.33.0",
					},
				},
			},
		},
	}

	mgr := NewManager(ManagerConfig{
		Bin:             "docker-compose",
		Mode:            "auto",
		CacheDir:        cacheDir,
		SelfContainerID: "self-id",
	}, mockDocker)
	mgr.httpClient = server.Client()
	mgr.downloadURLFunc = func(tag string, goos, goarch string) string {
		return server.URL
	}

	mgr.Warmup(context.Background())

	for _, v := range []string{"v2.31.0", "v2.32.0", "v2.33.0"} {
		expectedFile := filepath.Join(cacheDir, fmt.Sprintf("docker-compose-%s", v))
		if _, err := os.Stat(expectedFile); err != nil {
			t.Errorf("expected %s to be downloaded by Warmup: %v", expectedFile, err)
		}
	}
}

func TestProgressVisibilityAndSingleflight(t *testing.T) {
	cacheDir := t.TempDir()
	var downloadCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&downloadCount, 1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#!/bin/sh\n"))
	}))
	defer server.Close()

	mgr := NewManager(ManagerConfig{
		Bin:      "docker-compose",
		Mode:     "auto",
		CacheDir: cacheDir,
	}, nil)
	mgr.httpClient = server.Client()
	mgr.downloadURLFunc = func(tag string, goos, goarch string) string {
		return server.URL
	}

	// 1. Progress visibility
	var out bytes.Buffer
	bin, err := mgr.ensureComposeVersion(context.Background(), "v2.38.0", "docker-compose", &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedBin := filepath.Join(cacheDir, "docker-compose-v2.38.0")
	if bin != expectedBin {
		t.Errorf("expected %s, got %s", expectedBin, bin)
	}
	if !strings.Contains(out.String(), "[dokidoki] Preparing Docker Compose v2.38.0...") {
		t.Errorf("expected progress message, got %q", out.String())
	}

	// 2. Dedup concurrent downloads
	targetVer := "v2.39.0"
	var wg sync.WaitGroup
	concurrentWorkers := 5
	errs := make([]error, concurrentWorkers)
	bins := make([]string, concurrentWorkers)

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var buf bytes.Buffer
			b, e := mgr.ensureComposeVersion(context.Background(), targetVer, "docker-compose", &buf)
			bins[idx] = b
			errs[idx] = e
		}(i)
	}
	wg.Wait()

	for i := 0; i < concurrentWorkers; i++ {
		if errs[i] != nil {
			t.Errorf("worker %d returned error: %v", i, errs[i])
		}
		if bins[i] != filepath.Join(cacheDir, "docker-compose-v2.39.0") {
			t.Errorf("worker %d returned unexpected bin: %s", i, bins[i])
		}
	}

	// Download count should be 2 total (one for v2.38.0 and one for v2.39.0 despite 5 workers)
	if atomic.LoadInt32(&downloadCount) != 2 {
		t.Errorf("expected downloadCount 2, got %d", downloadCount)
	}
}
