package compose

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockDockerForManager struct {
	containers []types.Container
}

func (m *mockDockerForManager) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}
func (m *mockDockerForManager) ListContainers(ctx context.Context) ([]types.Container, error) {
	return m.containers, nil
}
func (m *mockDockerForManager) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
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
