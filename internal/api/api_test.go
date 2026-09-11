package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/compose"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/logger"
	"github.com/chickenzord/dokidoki/internal/stack"
	"github.com/chickenzord/dokidoki/web"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	errdefs "github.com/docker/docker/errdefs"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockDocker struct {
	pingFn             func(ctx context.Context) (types.Ping, error)
	listContainersFn   func(ctx context.Context) ([]types.Container, error)
	inspectContainerFn func(ctx context.Context, id string) (types.ContainerJSON, error)
	serverVersionFn    func(ctx context.Context) (types.Version, error)
	infoFn             func(ctx context.Context) (system.Info, error)
	createContainerFn  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	waitContainerFn    func(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	containerLogsFn    func(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error)
	removeContainerFn  func(ctx context.Context, id string, options container.RemoveOptions) error
}

func (m *mockDocker) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return types.Ping{}, nil
}

func (m *mockDocker) ListContainers(ctx context.Context) ([]types.Container, error) {
	if m.listContainersFn != nil {
		return m.listContainersFn(ctx)
	}
	return nil, nil
}

func (m *mockDocker) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if m.inspectContainerFn != nil {
		return m.inspectContainerFn(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func (m *mockDocker) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.serverVersionFn != nil {
		return m.serverVersionFn(ctx)
	}
	return types.Version{Version: "27.5.1", APIVersion: "1.47", Os: "linux", Arch: "amd64"}, nil
}

func (m *mockDocker) Info(ctx context.Context) (system.Info, error) {
	if m.infoFn != nil {
		return m.infoFn(ctx)
	}
	return system.Info{Containers: 2, ContainersRunning: 1, ContainersPaused: 0, ContainersStopped: 1, OperatingSystem: "Linux"}, nil
}

func (m *mockDocker) RestartContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}

func (m *mockDocker) StartContainer(ctx context.Context, id string) error {
	return nil
}

func (m *mockDocker) StopContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}

func (m *mockDocker) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	if m.createContainerFn != nil {
		return m.createContainerFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{}, nil
}

func (m *mockDocker) WaitContainer(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	if m.waitContainerFn != nil {
		return m.waitContainerFn(ctx, id, condition)
	}
	resCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)
	resCh <- container.WaitResponse{StatusCode: 0}
	return resCh, errCh
}

func (m *mockDocker) ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.containerLogsFn != nil {
		return m.containerLogsFn(ctx, id, options)
	}
	return io.NopCloser(strings.NewReader("stdout: hello\nstderr: world\n")), nil
}

func (m *mockDocker) RemoveContainer(ctx context.Context, id string, options container.RemoveOptions) error {
	if m.removeContainerFn != nil {
		return m.removeContainerFn(ctx, id, options)
	}
	return nil
}

func (m *mockDocker) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("pulled")), nil
}

func (m *mockDocker) Close() error {
	return nil
}

func setupTestServer(t *testing.T, m *mockDocker) (http.Handler, string) {
	return setupTestServerWithSelf(t, m, "")
}

func setupTestServerWithSelf(t *testing.T, m *mockDocker, selfContainerID string) (http.Handler, string) {
	tempDir := t.TempDir()

	// Create a managed stack directory with compose.yaml
	managedDir := filepath.Join(tempDir, "web-stack")
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		t.Fatalf("failed to create stack dir: %v", err)
	}
	composeContent := "services:\n  web:\n    image: nginx:latest\n"
	if err := os.WriteFile(filepath.Join(managedDir, "compose.yaml"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("failed to write compose.yaml: %v", err)
	}

	cfg := &config.Config{
		Bind:      "127.0.0.1",
		Port:      8080,
		StacksDir: tempDir,
	}

	scanner := stack.NewScanner(tempDir)
	stackSvc := stack.NewService(scanner, m, selfContainerID)
	server := NewServer(cfg, m, stackSvc, nil, selfContainerID)
	return server.Routes(), tempDir
}

func sampleContainers() []types.Container {
	return []types.Container{
		{
			ID:    "cid-1",
			Names: []string{"/web-stack_web_1"},
			Image: "nginx:latest",
			State: "running",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "web-stack",
				docker.ComposeServiceLabel: "web",
			},
		},
		{
			ID:    "cid-2",
			Names: []string{"/external_db_1"},
			Image: "postgres:latest",
			State: "running",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "external-stack",
				docker.ComposeServiceLabel: "db",
			},
		},
		{
			ID:    "cid-3",
			Names: []string{"/standalone-app"},
			Image: "alpine:latest",
			State: "exited",
		},
	}
}

func TestListStacks(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, _ := setupTestServer(t, m)

	// 1. Default (all)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stacksAll []stack.Summary
	if err := json.Unmarshal(w.Body.Bytes(), &stacksAll); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(stacksAll) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(stacksAll))
	}

	// 2. Managed only
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks?source=managed", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stacksManaged []stack.Summary
	if err := json.Unmarshal(w.Body.Bytes(), &stacksManaged); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(stacksManaged) != 1 || stacksManaged[0].Name != "web-stack" {
		t.Fatalf("expected 1 managed stack 'web-stack', got %+v", stacksManaged)
	}

	// 3. External only
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks?source=external", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stacksExt []stack.Summary
	if err := json.Unmarshal(w.Body.Bytes(), &stacksExt); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(stacksExt) != 1 || stacksExt[0].Name != "external-stack" {
		t.Fatalf("expected 1 external stack 'external-stack', got %+v", stacksExt)
	}

	// 4. Invalid source parameter
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks?source=foo", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid source, got %d", w.Code)
	}
}

func TestGetStack(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, _ := setupTestServer(t, m)

	// Existing stack
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var detail stack.Detail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if detail.Name != "web-stack" || detail.Source != "managed" {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	// Non-existent stack
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/not-exist", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var errResp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "stack not found" {
		t.Fatalf("expected 'stack not found', got %q", errResp["error"])
	}
}

func TestGetStackCompose(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, _ := setupTestServer(t, m)

	// 1. JSON default
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/compose", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp stack.ComposeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Name != "web-stack" || resp.Content == "" {
		t.Fatalf("unexpected compose response: %+v", resp)
	}

	// 2. YAML accept header
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/compose", nil)
	req.Header.Set("Accept", "text/yaml")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/yaml; charset=utf-8" {
		t.Fatalf("expected Content-Type text/yaml; charset=utf-8, got %q", ct)
	}
	if w.Body.String() != "services:\n  web:\n    image: nginx:latest\n" {
		t.Fatalf("unexpected raw compose: %s", w.Body.String())
	}

	// 3. Stack without compose file (external-stack)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/external-stack/compose", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for external stack compose, got %d", w.Code)
	}
}

func TestGetStackContainers(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, _ := setupTestServer(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/containers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var containers []docker.Container
	if err := json.Unmarshal(w.Body.Bytes(), &containers); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(containers) != 1 || containers[0].ID != "cid-1" {
		t.Fatalf("unexpected containers: %+v", containers)
	}

	// Non-existent stack
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/unknown/containers", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListContainers(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, _ := setupTestServer(t, m)

	// 1. Flat all
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var flat []docker.Container
	if err := json.Unmarshal(w.Body.Bytes(), &flat); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(flat) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(flat))
	}

	// 2. Filtered by stack
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers?stack=web-stack", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var filtered []docker.Container
	if err := json.Unmarshal(w.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Stack != "web-stack" {
		t.Fatalf("expected 1 container for web-stack, got %+v", filtered)
	}

	// 3. Grouped
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers?grouped=true", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var grouped stack.Grouped
	if err := json.Unmarshal(w.Body.Bytes(), &grouped); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(grouped.Stacks) != 2 || len(grouped.Standalone) != 1 {
		t.Fatalf("unexpected grouped containers: %+v", grouped)
	}

	// 4. Verify is_self with selfContainerID set
	handlerWithSelf, _ := setupTestServerWithSelf(t, m, "cid-1")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	w = httptest.NewRecorder()
	handlerWithSelf.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var flatWithSelf []docker.Container
	if err := json.Unmarshal(w.Body.Bytes(), &flatWithSelf); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	for _, c := range flatWithSelf {
		if c.ID == "cid-1" && !c.IsSelf {
			t.Errorf("expected container cid-1 to have is_self=true")
		}
		if c.ID != "cid-1" && c.IsSelf {
			t.Errorf("expected container %s to have is_self=false", c.ID)
		}
	}
}

func TestInspectContainer(t *testing.T) {
	m := &mockDocker{
		inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			if id == "cid-1" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   "cid-1",
						Name: "/web-stack_web_1",
					},
					Config: &container.Config{
						Labels: map[string]string{
							docker.ComposeProjectLabel: "web-stack",
							docker.ComposeServiceLabel: "web",
						},
					},
				}, nil
			}
			if id == "cid-2" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   "cid-2",
						Name: "/standalone-app",
					},
				}, nil
			}
			return types.ContainerJSON{}, errdefs.NotFound(errors.New("container not found"))
		},
	}
	handler, _ := setupTestServer(t, m)

	// Managed container inspection (bare-metal / no selfContainerID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers/cid-1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var enriched EnrichedContainerInspect
	if err := json.Unmarshal(w.Body.Bytes(), &enriched); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if enriched.StackName != "web-stack" || enriched.ServiceName != "web" || enriched.Source != "managed" {
		t.Fatalf("unexpected enrichment: %+v", enriched)
	}
	if enriched.IsSelf {
		t.Errorf("expected is_self=false when no selfContainerID configured")
	}

	// Inspection with selfContainerID set to cid-1
	handlerWithSelf, _ := setupTestServerWithSelf(t, m, "cid-1")

	// 1. Inspect self container
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers/cid-1", nil)
	w = httptest.NewRecorder()
	handlerWithSelf.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &enriched)
	if !enriched.IsSelf {
		t.Errorf("expected is_self=true for self container cid-1")
	}

	// 2. Inspect non-self container
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers/cid-2", nil)
	w = httptest.NewRecorder()
	handlerWithSelf.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &enriched)
	if enriched.IsSelf {
		t.Errorf("expected is_self=false for non-self container cid-2")
	}
	if enriched.Source != "standalone" {
		t.Fatalf("expected source 'standalone', got %q", enriched.Source)
	}

	// Not found container
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers/cid-missing", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHostInfo(t *testing.T) {
	m := &mockDocker{
		serverVersionFn: func(ctx context.Context) (types.Version, error) {
			return types.Version{Version: "27.5.1", APIVersion: "1.47", Os: "linux", Arch: "amd64"}, nil
		},
		infoFn: func(ctx context.Context) (system.Info, error) {
			return system.Info{Containers: 5, ContainersRunning: 3, ContainersPaused: 1, ContainersStopped: 1, OperatingSystem: "Alpine Linux"}, nil
		},
	}
	handler, _ := setupTestServer(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/host", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var hostInfo HostInfo
	if err := json.Unmarshal(w.Body.Bytes(), &hostInfo); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if hostInfo.DokidokiVersion != "0.1.0" || hostInfo.Docker.EngineVersion != "27.5.1" || hostInfo.Docker.Containers != 5 {
		t.Fatalf("unexpected host info: %+v", hostInfo)
	}
}

func TestHostPing(t *testing.T) {
	// Success ping
	mSuccess := &mockDocker{
		pingFn: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{APIVersion: "1.47"}, nil
		},
	}
	handler, _ := setupTestServer(t, mSuccess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/host/ping", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", resp["status"])
	}

	// Failure ping
	mFail := &mockDocker{
		pingFn: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{}, errors.New("cannot connect to docker")
		},
	}
	handlerFail, _ := setupTestServer(t, mFail)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/host/ping", nil)
	w = httptest.NewRecorder()
	handlerFail.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "error" || resp["error"] != "cannot connect to docker" {
		t.Fatalf("unexpected failure response: %+v", resp)
	}
}

func TestCORS(t *testing.T) {
	m := &mockDocker{}
	handler, _ := setupTestServer(t, m)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/host", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("expected 200 or 204 for OPTIONS, got %d", w.Code)
	}
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" && origin != "http://example.com" {
		t.Fatalf("expected CORS allow origin, got %q", origin)
	}
}

func setupClusterTestServer(t *testing.T, token string) (http.Handler, *cluster.Manager) {
	m := &mockDocker{}
	tempDir := t.TempDir()

	cfg := &config.Config{
		Bind:         "127.0.0.1",
		Port:         8080,
		StacksDir:    tempDir,
		ClusterToken: token,
	}

	self := cluster.Node{
		ID:        "node-self",
		Name:      "self-node",
		Addresses: []string{"http://127.0.0.1:8080"},
		Version:   "0.1.0",
		Status:    cluster.StatusAlive,
	}

	cm := cluster.NewManager(self, token, 30*time.Second, nil, tempDir)
	scanner := stack.NewScanner(tempDir)
	stackSvc := stack.NewService(scanner, m, "")
	server := NewServer(cfg, m, stackSvc, cm, "")
	return server.Routes(), cm
}

func TestClusterNodes(t *testing.T) {
	t.Run("NilClusterManager", func(t *testing.T) {
		m := &mockDocker{}
		handler, _ := setupTestServer(t, m)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var nodes []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if len(nodes) != 0 {
			t.Fatalf("expected 0 nodes, got %d", len(nodes))
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes/any-id", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		var errResp map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		if errResp["error"] != "node not found" {
			t.Fatalf("expected 'node not found', got %q", errResp["error"])
		}
	})

	t.Run("WithClusterManager", func(t *testing.T) {
		handler, cm := setupClusterTestServer(t, "test-token")

		// Register a remote peer via handshake
		_, _ = cm.HandleHandshake(cluster.HandshakeRequest{
			NodeID:       "node-peer-1",
			Name:         "peer-1",
			Addresses:    []string{"http://192.168.1.10:8080"},
			Version:      "0.1.0",
			ClusterToken: "test-token",
		})

		// 1. List nodes
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var nodes []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if len(nodes) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(nodes))
		}

		// 2. Get existing self node
		req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes/node-self", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for self node, got %d", w.Code)
		}
		var selfNode cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &selfNode); err != nil {
			t.Fatal(err)
		}
		if selfNode.ID != "node-self" || !selfNode.IsSelf {
			t.Fatalf("unexpected self node: %+v", selfNode)
		}

		// 3. Get existing peer node
		req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes/node-peer-1", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for peer node, got %d", w.Code)
		}
		var peerNode cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &peerNode); err != nil {
			t.Fatal(err)
		}
		if peerNode.ID != "node-peer-1" || peerNode.Name != "peer-1" {
			t.Fatalf("unexpected peer node: %+v", peerNode)
		}

		// 4. Get non-existent node
		req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes/non-existent", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		var errResp map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		if errResp["error"] != "node not found" {
			t.Fatalf("expected 'node not found', got %q", errResp["error"])
		}
	})
}

func TestClusterHandshake(t *testing.T) {
	handler, _ := setupClusterTestServer(t, "valid-token")

	// 1. Successful handshake
	hsReq := cluster.HandshakeRequest{
		NodeID:       "node-incoming",
		Name:         "node-incoming",
		Addresses:    []string{"http://10.0.0.5:8080"},
		Version:      "0.1.0",
		ClusterToken: "valid-token",
	}
	body, _ := json.Marshal(hsReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/handshake", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var hsResp cluster.HandshakeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &hsResp); err != nil {
		t.Fatalf("failed to unmarshal HandshakeResponse: %v", err)
	}
	if hsResp.NodeID != "node-self" {
		t.Fatalf("expected response NodeID 'node-self', got %q", hsResp.NodeID)
	}

	// 2. Handshake with invalid token -> 401
	hsReq.ClusterToken = "wrong-token"
	body, _ = json.Marshal(hsReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/handshake", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", w.Code)
	}

	// 3. Handshake with invalid JSON -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/handshake", bytes.NewReader([]byte("not-json")))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", w.Code)
	}

	// 4. Handshake with empty node_id -> 400
	hsReq.NodeID = ""
	hsReq.ClusterToken = "valid-token"
	body, _ = json.Marshal(hsReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/handshake", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty node_id, got %d", w.Code)
	}
}

func TestClusterHeartbeat(t *testing.T) {
	handler, _ := setupClusterTestServer(t, "hb-token")

	// 1. Successful heartbeat
	hbMsg := cluster.HeartbeatMessage{
		NodeID:       "node-peer-hb",
		Addresses:    []string{"http://10.0.0.6:8080"},
		ClusterToken: "hb-token",
	}
	body, _ := json.Marshal(hbMsg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %q", resp["status"])
	}

	// 2. Heartbeat with invalid token -> 401
	hbMsg.ClusterToken = "invalid-token"
	body, _ = json.Marshal(hbMsg)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/heartbeat", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", w.Code)
	}

	// 3. Heartbeat with invalid JSON -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/heartbeat", bytes.NewReader([]byte("corrupt")))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for corrupt JSON, got %d", w.Code)
	}
}

func TestClusterLeave(t *testing.T) {
	handler, cm := setupClusterTestServer(t, "")

	// Handshake to add a peer
	_, _ = cm.HandleHandshake(cluster.HandshakeRequest{
		NodeID:    "node-to-leave",
		Name:      "leaving-node",
		Addresses: []string{"http://10.0.0.9:8080"},
	})
	if _, ok := cm.GetNode("node-to-leave"); !ok {
		t.Fatal("expected peer to be present before leave")
	}

	// 1. Leave with JSON body
	leaveBody, _ := json.Marshal(map[string]string{"node_id": "node-to-leave"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/leave", bytes.NewReader(leaveBody))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %q", resp["status"])
	}

	peer, ok := cm.GetNode("node-to-leave")
	if !ok {
		t.Fatal("expected peer to still exist after leave")
	}
	if peer.Status != cluster.StatusOffline {
		t.Fatalf("expected peer status to be offline after leave, got %s", peer.Status)
	}

	// 2. Missing node_id -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/leave", bytes.NewReader([]byte("{}")))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty node_id, got %d", w.Code)
	}

	// 3. Manual DELETE /api/v1/nodes/{id}
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/node-to-leave", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on DELETE /api/v1/nodes/node-to-leave, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := cm.GetNode("node-to-leave"); ok {
		t.Fatal("expected peer to be permanently removed after DELETE")
	}

	// 4. Delete non-existent node -> 404
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/non-existent", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent node, got %d", w.Code)
	}
}

func TestWebUIHandler(t *testing.T) {
	m := &mockDocker{
		pingFn: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{APIVersion: "1.47"}, nil
		},
	}

	fakeFS := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<!doctype html><title>SPA</title>")},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{margin:0}")},
	}

	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "web-stack"), 0755); err != nil {
		t.Fatalf("failed to create stack dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "web-stack", "compose.yaml"), []byte("services:\n  web:\n    image: nginx:latest\n"), 0644); err != nil {
		t.Fatalf("failed to write compose.yaml: %v", err)
	}

	cfg := &config.Config{Bind: "127.0.0.1", Port: 8080, StacksDir: tempDir}
	scanner := stack.NewScanner(tempDir)
	stackSvc := stack.NewService(scanner, m, "")
	server := NewServer(cfg, m, stackSvc, nil, "").WithWebUI(web.HandlerFromFS(fakeFS))
	handler := server.Routes()

	// 1. Root / serves the SPA entry
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected non-empty body for /")
	}

	// 2. Non-API route falls back to SPA root
	req = httptest.NewRequest(http.MethodGet, "/stacks/my-custom-stack", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA route, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected non-empty fallback body")
	}

	// 3. Static asset served
	req = httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /assets/app.js, got %d", w.Code)
	}

	// 4. API route /api/v1/host/ping has precedence over SPA
	req = httptest.NewRequest(http.MethodGet, "/api/v1/host/ping", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for API ping, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp)
	}

	// 5. Unknown /api/v1 route returns 404 and does not fall back to web UI
	req = httptest.NewRequest(http.MethodGet, "/api/v1/not-a-real-route", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown API route, got %d", w.Code)
	}
}

func TestHeartbeatLogDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	lvlVar := &slog.LevelVar{}
	lvlVar.Set(slog.LevelInfo)
	logger.Setup(&buf, lvlVar, "text", false)
	defer logger.Setup(os.Stdout, slog.LevelInfo, "text", true)

	handler, _ := setupClusterTestServer(t, "")

	hbMsg := cluster.HeartbeatMessage{
		NodeID:    "node-remote-hb",
		Addresses: []string{"http://10.0.0.9:8080"},
	}
	body, _ := json.Marshal(hbMsg)

	// 1. With LevelInfo (default), heartbeat request should NOT be logged
	lvlVar.Set(slog.LevelInfo)
	buf.Reset()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(buf.String(), "/api/v1/cluster/heartbeat") {
		t.Errorf("heartbeat was logged at info level: %q", buf.String())
	}

	// Non-heartbeat request should be logged at info level
	buf.Reset()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "[INFO]") || !strings.Contains(buf.String(), "/api/v1/nodes") {
		t.Errorf("expected non-heartbeat request to be logged at info level, got: %q", buf.String())
	}

	// 2. With LevelDebug, heartbeat request SHOULD be logged at debug level
	lvlVar.Set(slog.LevelDebug)
	buf.Reset()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/heartbeat", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(buf.String(), "[DEBUG]") || !strings.Contains(buf.String(), "/api/v1/cluster/heartbeat") {
		t.Errorf("expected heartbeat to be logged at debug level, got: %q", buf.String())
	}
}

func TestCreateStack(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, tempDir := setupTestServer(t, m)

	// 1. Success with content
	payload := `{"name": "new-api-stack", "files": [{"name": "compose.yaml", "content": "services:\n  app:\n    image: node:18\n"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
	var summary stack.Summary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if summary.Name != "new-api-stack" || summary.Source != "managed" {
		t.Errorf("unexpected summary: %+v", summary)
	}

	// Verify file was written
	composePath := filepath.Join(tempDir, "new-api-stack", "compose.yaml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if !strings.Contains(string(data), "image: node:18") {
		t.Errorf("unexpected file content: %s", string(data))
	}

	// 2. Empty name -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(`{"name": ""}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d", w.Code)
	}

	// 3. Invalid body -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(`{invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", w.Code)
	}

	// 4. Empty files -> 400
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(`{"name": "no-files", "files": []}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty files, got %d", w.Code)
	}
}

func TestUpdateStack(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, tempDir := setupTestServer(t, m)

	// Update existing web-stack
	payload := `{"files": [{"name": "compose.yaml", "content": "services:\n  web:\n    image: nginx:1.25-alpine\n"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/stacks/web-stack", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var summary stack.Summary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if summary.Name != "web-stack" || summary.Source != "managed" {
		t.Errorf("unexpected summary: %+v", summary)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, "web-stack", "compose.yaml"))
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if !strings.Contains(string(data), "image: nginx:1.25-alpine") {
		t.Errorf("unexpected updated content: %s", string(data))
	}

	// Update non-existent stack -> 404
	req = httptest.NewRequest(http.MethodPut, "/api/v1/stacks/nonexistent-stack", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent stack, got %d", w.Code)
	}

	// Update external stack -> 400
	req = httptest.NewRequest(http.MethodPut, "/api/v1/stacks/external-stack", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for external stack, got %d", w.Code)
	}
}

func TestGetStackFiles(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, tempDir := setupTestServer(t, m)

	// Add an env file to web-stack
	_ = os.WriteFile(filepath.Join(tempDir, "web-stack", ".env"), []byte("PORT=80\n"), 0644)

	// 1. Success GET
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/files", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp stack.StackFilesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Stack != "web-stack" || len(resp.Files) != 2 {
		t.Fatalf("unexpected files response: %+v", resp)
	}

	// 2. External stack without read query parameter -> empty files
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/external-stack/files", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for external stack files, got %d", w.Code)
	}
	var extResp stack.StackFilesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &extResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(extResp.Files) != 0 {
		t.Fatalf("expected 0 files without read=true on external stack, got %d", len(extResp.Files))
	}

	// 3. Not found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/unknown-stack/files", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown stack files, got %d", w.Code)
	}
}

func TestGetStackFile(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
	}
	handler, _ := setupTestServer(t, m)

	// 1. Success GET JSON
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/files/compose.yaml", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var file stack.StackFile
	if err := json.Unmarshal(w.Body.Bytes(), &file); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if file.Name != "compose.yaml" || !file.IsCompose || !strings.Contains(file.Content, "image: nginx") {
		t.Fatalf("unexpected stack file: %+v", file)
	}

	// 2. Success GET raw text via Accept
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/files/compose.yaml", nil)
	req.Header.Set("Accept", "text/plain")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "image: nginx") {
		t.Fatalf("unexpected plain text body: %s", w.Body.String())
	}

	// 3. File not found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks/web-stack/files/missing.txt", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing file, got %d", w.Code)
	}
}

func TestGetContainerCompose(t *testing.T) {
	m := &mockDocker{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return sampleContainers(), nil
		},
		inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			if id == "cid-1" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   "cid-1",
						Name: "/web-stack_web_1",
						HostConfig: &container.HostConfig{
							RestartPolicy: container.RestartPolicy{Name: "always"},
						},
					},
					Config: &container.Config{
						Image: "nginx:latest",
						Labels: map[string]string{
							docker.ComposeProjectLabel: "web-stack",
							docker.ComposeServiceLabel: "web",
						},
					},
				}, nil
			}
			return types.ContainerJSON{}, errdefs.NotFound(errors.New("no such container"))
		},
	}
	handler, _ := setupTestServer(t, m)

	// 1. Success JSON
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers/cid-1/compose", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp stack.ContainerComposeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.StackName != "web-stack" || !strings.Contains(resp.Content, "image: nginx:latest") {
		t.Fatalf("unexpected compose response: %+v", resp)
	}

	// 2. Accept: text/yaml
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers/cid-1/compose", nil)
	req.Header.Set("Accept", "text/yaml")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/yaml") {
		t.Errorf("expected Content-Type text/yaml, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "image: nginx:latest") {
		t.Errorf("unexpected yaml content: %s", w.Body.String())
	}

	// 3. Not Found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/containers/missing-id/compose", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing container, got %d", w.Code)
	}
}

type mockComposeRunner struct {
	upFn      func(ctx context.Context, path string, out io.Writer) error
	downFn    func(ctx context.Context, path string, out io.Writer) error
	restartFn func(ctx context.Context, path string, service string, out io.Writer) error
	pullFn    func(ctx context.Context, path string, out io.Writer) error
}

func (m *mockComposeRunner) Up(ctx context.Context, path string, out io.Writer, extraArgs ...string) error {
	if m.upFn != nil {
		return m.upFn(ctx, path, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("up streamed output\n"))
	}
	return nil
}

func (m *mockComposeRunner) Down(ctx context.Context, path string, out io.Writer) error {
	if m.downFn != nil {
		return m.downFn(ctx, path, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("down streamed output\n"))
	}
	return nil
}

func (m *mockComposeRunner) Restart(ctx context.Context, path string, service string, out io.Writer) error {
	if m.restartFn != nil {
		return m.restartFn(ctx, path, service, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("restart streamed output\n"))
	}
	return nil
}

func (m *mockComposeRunner) Pull(ctx context.Context, path string, out io.Writer) error {
	if m.pullFn != nil {
		return m.pullFn(ctx, path, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("pull streamed output\n"))
	}
	return nil
}

func setupTestServerWithRunner(t *testing.T, m *mockDocker, runner compose.Runner, selfContainerID string) (http.Handler, string) {
	tempDir := t.TempDir()

	managedDir := filepath.Join(tempDir, "web-stack")
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		t.Fatalf("failed to create stack dir: %v", err)
	}
	composeContent := "services:\n  web:\n    image: nginx:latest\n"
	if err := os.WriteFile(filepath.Join(managedDir, "compose.yaml"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("failed to write compose.yaml: %v", err)
	}

	cfg := &config.Config{
		Bind:      "127.0.0.1",
		Port:      8080,
		StacksDir: tempDir,
	}

	scanner := stack.NewScanner(tempDir)
	stackSvc := stack.NewService(scanner, m, selfContainerID, stack.WithComposeRunner(runner))
	server := NewServer(cfg, m, stackSvc, nil, selfContainerID)
	return server.Routes(), tempDir
}

func TestContainerOperationHandlers(t *testing.T) {
	m := &mockDocker{
		inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			if id == "cid-1" || id == "self-cid" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{ID: id, Name: "/test"},
					Config:            &container.Config{Image: "nginx:alpine"},
				}, nil
			}
			return types.ContainerJSON{}, errdefs.NotFound(errors.New("no such container"))
		},
	}
	handler, _ := setupTestServerWithSelf(t, m, "self-cid")

	t.Run("RestartContainer_Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/cid-1/restart", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var res stack.OperationResult
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("RestartContainer_SelfSafeguard", func(t *testing.T) {
		// Without force: 400 Bad Request
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/self-cid/restart", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for self without force, got %d: %s", w.Code, w.Body.String())
		}

		// With ?force=true: 200 OK
		req = httptest.NewRequest(http.MethodPost, "/api/v1/containers/self-cid/restart?force=true", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for self with force, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("StartContainer_Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/cid-1/start", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("StopContainer_SelfSafeguard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/self-cid/stop", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for self without force, got %d", w.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/containers/self-cid/stop?force=true", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for self with force, got %d", w.Code)
		}
	})

	t.Run("PullContainer_JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/cid-1/pull", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var res stack.OperationResult
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("PullContainer_Streaming", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containers/cid-1/pull?stream=true", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Errorf("expected Content-Type text/plain, got %s", ct)
		}
		if !strings.Contains(w.Body.String(), "pulled") {
			t.Errorf("expected 'pulled' in stream, got %q", w.Body.String())
		}
	})
}

func TestStackComposeOperationHandlers(t *testing.T) {
	m := &mockDocker{}
	runner := &mockComposeRunner{}
	handler, _ := setupTestServerWithRunner(t, m, runner, "")

	t.Run("ComposeUp_JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/web-stack/up", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var res stack.OperationResult
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("ComposeUp_Streaming", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/web-stack/up?stream=true", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Errorf("expected Content-Type text/plain, got %s", ct)
		}
		if !strings.Contains(w.Body.String(), "up streamed output") {
			t.Errorf("expected 'up streamed output' in stream, got %q", w.Body.String())
		}
	})

	t.Run("ComposeDown_Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/web-stack/down", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("ComposeRestart_WithService", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/web-stack/restart?service=web", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("ComposePull_Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/web-stack/pull", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/nonexistent-stack/up", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for missing stack, got %d", w.Code)
		}
	})
}

type mockClusterService struct {
	nodes []cluster.Node
}

func (m *mockClusterService) ListNodes() []cluster.Node {
	return m.nodes
}

func (m *mockClusterService) GetNode(id string) (*cluster.Node, bool) {
	for _, n := range m.nodes {
		if n.ID == id {
			return &n, true
		}
	}
	return nil, false
}

func (m *mockClusterService) HandleHandshake(req cluster.HandshakeRequest) (*cluster.HandshakeResponse, error) {
	return nil, nil
}

func (m *mockClusterService) HandleHeartbeat(msg cluster.HeartbeatMessage) error {
	return nil
}

func (m *mockClusterService) HandleLeave(nodeID string) {}

func (m *mockClusterService) RemoveNode(nodeID string) error {
	return nil
}

func TestHandleListNodes(t *testing.T) {
	nodes := []cluster.Node{
		{ID: "node-1", Name: "Node 1", Status: cluster.StatusAlive},
		{ID: "node-2", Name: "Node 2", Status: cluster.StatusSuspect},
		{ID: "node-3", Name: "Node 3", Status: cluster.StatusOffline},
	}
	mockCluster := &mockClusterService{nodes: nodes}
	cfg := &config.Config{Bind: "127.0.0.1", Port: 8080}
	server := NewServer(cfg, &mockDocker{}, nil, mockCluster, "")
	handler := server.Routes()

	t.Run("AllNodes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result) != 3 {
			t.Fatalf("expected 3 nodes, got %d", len(result))
		}
	})

	t.Run("StatusFilter_Alive", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes?status=alive", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result) != 1 || result[0].ID != "node-1" {
			t.Fatalf("expected 1 alive node (node-1), got %+v", result)
		}
	})

	t.Run("StatusFilter_Active", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes?status=active", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 active nodes (alive & suspect), got %d", len(result))
		}
	})

	t.Run("StatusFilter_Offline", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes?status=offline", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result) != 1 || result[0].ID != "node-3" {
			t.Fatalf("expected 1 offline node (node-3), got %+v", result)
		}
	})

	t.Run("NilClusterService", func(t *testing.T) {
		nilServer := NewServer(cfg, &mockDocker{}, nil, nil, "")
		nilHandler := nilServer.Routes()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		w := httptest.NewRecorder()
		nilHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result []cluster.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 nodes, got %d", len(result))
		}
	})
}




