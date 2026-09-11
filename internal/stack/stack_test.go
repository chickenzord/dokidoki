package stack

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	errdefs "github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestCalculateRollup(t *testing.T) {
	tests := []struct {
		name       string
		containers []docker.Container
		expected   Rollup
	}{
		{
			name:       "empty containers",
			containers: []docker.Container{},
			expected:   Rollup{},
		},
		{
			name: "various states with case insensitivity",
			containers: []docker.Container{
				{State: "running"},
				{State: "RUNNING"},
				{State: "exited"},
				{State: "Exited"},
				{State: "restarting"},
				{State: "paused"},
				{State: "dead"},
				{State: "created"}, // Not one of the 5 specific states, but increments Total
			},
			expected: Rollup{
				Running:    2,
				Exited:     2,
				Restarting: 1,
				Paused:     1,
				Dead:       1,
				Total:      8,
			},
		},
		{
			name: "completed init container with exit code 0",
			containers: []docker.Container{
				{State: "running"},
				{State: "running"},
				{State: "exited", Status: "Exited (0) 4 minutes ago"},
				{State: "exited", Status: "Exited (1) 2 minutes ago"},
			},
			expected: Rollup{
				Running:   2,
				Exited:    2,
				Completed: 1,
				Total:     4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CalculateRollup(tt.containers)
			if actual != tt.expected {
				t.Errorf("expected %+v, got %+v", tt.expected, actual)
			}
		})
	}
}

func TestCategorize_ManagedStacks(t *testing.T) {
	discovered := []DiscoveredStack{
		{
			Name:           "app-one",
			ComposePath:    "/opt/stacks/app-one/compose.yaml",
			ComposePresent: true,
		},
		{
			Name:           "app-two",
			ComposePath:    "/opt/stacks/app-two/compose.yaml",
			ComposePresent: true,
		},
	}

	containers := []docker.Container{
		{
			ID:      "c1",
			Name:    "app-one-web-1",
			Stack:   "app-one",
			Service: "web",
			State:   "running",
		},
		{
			ID:      "c2",
			Name:    "app-one-db-1",
			Stack:   "app-one",
			Service: "db",
			State:   "running",
		},
	}

	result := Categorize(discovered, containers)

	if len(result.Stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(result.Stacks))
	}

	// Stacks are sorted alphabetically: app-one, app-two
	s1 := result.Stacks[0]
	if s1.Name != "app-one" || s1.Source != string(SourceManaged) || !s1.ComposePresent {
		t.Errorf("unexpected s1 summary: %+v", s1)
	}
	if s1.Rollup.Running != 2 || s1.Rollup.Total != 2 {
		t.Errorf("unexpected s1 rollup: %+v", s1.Rollup)
	}
	if len(s1.Services) != 2 || s1.Services[0] != "db" || s1.Services[1] != "web" {
		t.Errorf("unexpected s1 services: %+v", s1.Services)
	}

	// app-two has 0 containers, rollup should be all 0s
	s2 := result.Stacks[1]
	if s2.Name != "app-two" || s2.Source != string(SourceManaged) || !s2.ComposePresent {
		t.Errorf("unexpected s2 summary: %+v", s2)
	}
	if s2.Rollup != (Rollup{}) {
		t.Errorf("expected zero rollup for app-two, got %+v", s2.Rollup)
	}
	if len(s2.Services) != 0 {
		t.Errorf("expected 0 services for app-two, got %+v", s2.Services)
	}

	// Check details
	d1, ok := result.GetStack("app-one")
	if !ok || len(d1.Containers) != 2 {
		t.Errorf("expected 2 containers in detail for app-one, got %v", d1)
	}
	d2, ok := result.GetStack("app-two")
	if !ok || len(d2.Containers) != 0 {
		t.Errorf("expected 0 containers in detail for app-two, got %v", d2)
	}
}

func TestCategorize_PendingImport(t *testing.T) {
	discovered := []DiscoveredStack{
		{
			Name:           "my-app",
			ComposePath:    "/opt/dokidoki/stacks/my-app/compose.yaml",
			ComposePresent: true,
		},
		{
			Name:           "clean-app",
			ComposePath:    "/opt/dokidoki/stacks/clean-app/compose.yaml",
			ComposePresent: true,
		},
	}

	containers := []docker.Container{
		{
			ID:      "c1",
			Name:    "my-app-web-1",
			Stack:   "my-app",
			Service: "web",
			State:   "running",
			Labels: map[string]string{
				"com.docker.compose.project.working_dir": "/opt/other/path/my-app",
			},
		},
		{
			ID:      "c2",
			Name:    "clean-app-web-1",
			Stack:   "clean-app",
			Service: "web",
			State:   "running",
			Labels: map[string]string{
				"com.docker.compose.project.working_dir": "/opt/dokidoki/stacks/clean-app",
			},
		},
	}

	result := Categorize(discovered, containers)
	myAppDetail, ok := result.GetStack("my-app")
	if !ok || !myAppDetail.PendingImport {
		t.Errorf("expected my-app to have PendingImport=true, got ok=%v PendingImport=%v", ok, myAppDetail.PendingImport)
	}
	cleanAppDetail, ok := result.GetStack("clean-app")
	if !ok || cleanAppDetail.PendingImport {
		t.Errorf("expected clean-app to have PendingImport=false, got ok=%v PendingImport=%v", ok, cleanAppDetail.PendingImport)
	}

	// Test config_files outside expectedStackDir
	containersConfigMismatch := []docker.Container{
		{
			ID:      "c3",
			Name:    "my-app-web-1",
			Stack:   "my-app",
			Service: "web",
			State:   "running",
			Labels: map[string]string{
				"com.docker.compose.project.config_files": "/opt/outside/docker-compose.yml",
			},
		},
	}
	resultConfig := Categorize(discovered, containersConfigMismatch)
	myAppDetail2, _ := resultConfig.GetStack("my-app")
	if !myAppDetail2.PendingImport {
		t.Errorf("expected my-app to have PendingImport=true when config_files differs")
	}
}

func TestCategorize_ExternalStacks(t *testing.T) {
	// No stacks discovered on filesystem
	discovered := []DiscoveredStack{}

	containers := []docker.Container{
		{
			ID:      "ext1",
			Name:    "external-nginx-1",
			Stack:   "external-proxy",
			Service: "nginx",
			State:   "running",
		},
		{
			ID:      "ext2",
			Name:    "external-certbot-1",
			Stack:   "external-proxy",
			Service: "certbot",
			State:   "exited",
		},
	}

	result := Categorize(discovered, containers)

	if len(result.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(result.Stacks))
	}

	s := result.Stacks[0]
	if s.Name != "external-proxy" {
		t.Errorf("expected stack name external-proxy, got %s", s.Name)
	}
	if s.Source != string(SourceExternal) {
		t.Errorf("expected source external, got %s", s.Source)
	}
	if s.ComposePresent {
		t.Errorf("expected ComposePresent=false for external stack")
	}
	if s.ComposePath != "" {
		t.Errorf("expected empty ComposePath for external stack, got %s", s.ComposePath)
	}
	if s.Rollup.Running != 1 || s.Rollup.Exited != 1 || s.Rollup.Total != 2 {
		t.Errorf("unexpected rollup for external stack: %+v", s.Rollup)
	}
	if len(s.Services) != 2 || s.Services[0] != "certbot" || s.Services[1] != "nginx" {
		t.Errorf("unexpected services for external stack: %+v", s.Services)
	}
}

func TestCategorize_StandaloneContainers(t *testing.T) {
	discovered := []DiscoveredStack{}

	containers := []docker.Container{
		{
			ID:    "stand1",
			Name:  "my-redis",
			Stack: "", // No stack
			State: "running",
		},
		{
			ID:    "stand2",
			Name:  "my-postgres",
			Stack: "  ", // Whitespace only
			State: "running",
		},
	}

	result := Categorize(discovered, containers)

	if len(result.Stacks) != 0 {
		t.Errorf("expected 0 stacks, got %d", len(result.Stacks))
	}
	if len(result.Standalone) != 2 {
		t.Fatalf("expected 2 standalone containers, got %d", len(result.Standalone))
	}
	if result.Standalone[0].ID != "stand1" || result.Standalone[1].ID != "stand2" {
		t.Errorf("unexpected standalone containers: %+v", result.Standalone)
	}
	if len(result.Grouped.Standalone) != 2 {
		t.Errorf("expected 2 standalone in Grouped, got %d", len(result.Grouped.Standalone))
	}
}

func TestCategorize_MixedScenario(t *testing.T) {
	discovered := []DiscoveredStack{
		{
			Name:           "managed-active",
			ComposePath:    "/opt/stacks/managed-active/compose.yaml",
			ComposePresent: true,
		},
		{
			Name:           "managed-idle",
			ComposePath:    "/opt/stacks/managed-idle/compose.yaml",
			ComposePresent: true,
		},
	}

	rawContainers := []types.Container{
		{
			ID:    "c_managed_1",
			Names: []string{"/managed-active_worker_1"},
			State: "running",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "managed-active",
				docker.ComposeServiceLabel: "worker",
			},
		},
		{
			ID:    "c_ext_1",
			Names: []string{"/coolify_proxy_1"},
			State: "running",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "coolify-proxy",
				docker.ComposeServiceLabel: "proxy",
			},
		},
		{
			ID:    "c_standalone_1",
			Names: []string{"/adhoc_busybox"},
			State: "exited",
			// No labels
		},
	}

	result := CategorizeRaw(discovered, rawContainers, "")

	// Should have 3 stacks: coolify-proxy (external), managed-active (managed), managed-idle (managed)
	if len(result.Stacks) != 3 {
		t.Fatalf("expected 3 stacks, got %d", len(result.Stacks))
	}

	// Alphabetical order
	if result.Stacks[0].Name != "coolify-proxy" || result.Stacks[0].Source != string(SourceExternal) {
		t.Errorf("expected Stacks[0] to be coolify-proxy (external), got %+v", result.Stacks[0])
	}
	if result.Stacks[1].Name != "managed-active" || result.Stacks[1].Source != string(SourceManaged) {
		t.Errorf("expected Stacks[1] to be managed-active (managed), got %+v", result.Stacks[1])
	}
	if result.Stacks[2].Name != "managed-idle" || result.Stacks[2].Source != string(SourceManaged) {
		t.Errorf("expected Stacks[2] to be managed-idle (managed), got %+v", result.Stacks[2])
	}

	// Check filters
	managed := result.ManagedStacks()
	if len(managed) != 2 {
		t.Errorf("expected 2 managed stacks, got %d", len(managed))
	}

	external := result.ExternalStacks()
	if len(external) != 1 || external[0].Name != "coolify-proxy" {
		t.Errorf("expected 1 external stack coolify-proxy, got %v", external)
	}

	// Check standalone
	if len(result.Standalone) != 1 || result.Standalone[0].Name != "adhoc_busybox" {
		t.Errorf("expected 1 standalone adhoc_busybox, got %v", result.Standalone)
	}
}

func TestScanner(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Stack with compose.yaml
	s1Dir := filepath.Join(tempDir, "stack1")
	if err := os.MkdirAll(s1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s1Dir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Stack with compose.yml
	s2Dir := filepath.Join(tempDir, "stack2")
	if err := os.MkdirAll(s2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s2Dir, "compose.yml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Stack with docker-compose.yaml
	s3Dir := filepath.Join(tempDir, "stack3")
	if err := os.MkdirAll(s3Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s3Dir, "docker-compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Stack with docker-compose.yml
	s4Dir := filepath.Join(tempDir, "stack4")
	if err := os.MkdirAll(s4Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s4Dir, "docker-compose.yml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// 5. Stack with multiple candidates - compose.yaml should win over docker-compose.yml
	s5Dir := filepath.Join(tempDir, "stack5")
	if err := os.MkdirAll(s5Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s5Dir, "compose.yaml"), []byte("preferred"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s5Dir, "docker-compose.yml"), []byte("fallback"), 0644); err != nil {
		t.Fatal(err)
	}

	// 6. Directory without compose file (should be ignored)
	emptyDir := filepath.Join(tempDir, "empty-dir")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 7. Hidden directory (should be ignored)
	hiddenDir := filepath.Join(tempDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "compose.yaml"), []byte("hidden"), 0644); err != nil {
		t.Fatal(err)
	}

	// 8. Regular file in stacks directory (should be ignored)
	if err := os.WriteFile(filepath.Join(tempDir, "some-file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tempDir)
	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(discovered) != 5 {
		t.Fatalf("expected 5 discovered stacks, got %d", len(discovered))
	}

	expectedNames := []string{"stack1", "stack2", "stack3", "stack4", "stack5"}
	for i, name := range expectedNames {
		if discovered[i].Name != name {
			t.Errorf("expected stack %d to be %s, got %s", i, name, discovered[i].Name)
		}
	}

	// Verify stack5 selected compose.yaml
	expectedS5Path := filepath.Join(s5Dir, "compose.yaml")
	if discovered[4].ComposePath != expectedS5Path {
		t.Errorf("expected stack5 path %s, got %s", expectedS5Path, discovered[4].ComposePath)
	}

	// Non-existent directory should return empty slice and nil error
	emptyScan, err := ScanDir("/non/existent/path/for/sure")
	if err != nil {
		t.Errorf("expected nil error for nonexistent dir, got %v", err)
	}
	if len(emptyScan) != 0 {
		t.Errorf("expected 0 stacks, got %d", len(emptyScan))
	}
}

func TestReadComposeFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "compose.yaml")
	content := "services:\n  web:\n    image: nginx\n"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Read existing file
	data, err := ReadComposeFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != content {
		t.Errorf("expected content %q, got %q", content, data)
	}

	// Read via GetComposeFileResponse
	resp, err := GetComposeFileResponse("web-stack", filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "web-stack" || resp.Path != filePath || resp.Content != content {
		t.Errorf("unexpected compose file response: %+v", resp)
	}

	// Empty path error
	if _, err := ReadComposeFile(""); err == nil {
		t.Error("expected error for empty path, got nil")
	}

	// Nonexistent file error
	if _, err := ReadComposeFile(filepath.Join(tempDir, "missing.yaml")); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestConvertContainer(t *testing.T) {
	c := types.Container{
		ID:      "abc123def456789",
		Names:   []string{"/my-service_1"},
		Image:   "redis:alpine",
		State:   "running",
		Status:  "Up 10 hours",
		Created: 1690000000,
		Ports: []types.Port{
			{
				IP:          "0.0.0.0",
				PrivatePort: 6379,
				PublicPort:  6379,
				Type:        "tcp",
			},
		},
		Labels: map[string]string{
			docker.ComposeProjectLabel: "my-stack",
			docker.ComposeServiceLabel: "redis",
			"custom.label":             "value",
		},
	}

	summary := ConvertContainer(c, "")

	if summary.ID != "abc123def456789" {
		t.Errorf("unexpected ID: %s", summary.ID)
	}
	if summary.Name != "my-service_1" {
		t.Errorf("expected leading slash removed, got %s", summary.Name)
	}
	if summary.Image != "redis:alpine" {
		t.Errorf("unexpected image: %s", summary.Image)
	}
	if summary.State != "running" || summary.Status != "Up 10 hours" {
		t.Errorf("unexpected state/status: %s / %s", summary.State, summary.Status)
	}
	if summary.Created != 1690000000 {
		t.Errorf("unexpected created: %d", summary.Created)
	}
	if len(summary.Ports) != 1 || summary.Ports[0].PrivatePort != 6379 || summary.Ports[0].PublicPort != 6379 {
		t.Errorf("unexpected ports: %+v", summary.Ports)
	}
	if summary.Stack != "my-stack" || summary.Service != "redis" {
		t.Errorf("unexpected stack/service: %s / %s", summary.Stack, summary.Service)
	}
	if summary.Labels["custom.label"] != "value" {
		t.Errorf("unexpected labels: %+v", summary.Labels)
	}
	if summary.IsSelf {
		t.Errorf("expected IsSelf to be false with empty selfContainerID")
	}

	// Test container with no name falling back to trimmed ID
	cNoName := types.Container{
		ID: "1234567890abcdef1234",
	}
	summaryNoName := ConvertContainer(cNoName, "")
	if summaryNoName.Name != "1234567890ab" {
		t.Errorf("expected fallback to 12 chars ID, got %s", summaryNoName.Name)
	}
}

func TestConvertContainer_IsSelf(t *testing.T) {
	fullID := "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1"
	shortID := "47040a455a73"
	otherID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	tests := []struct {
		name            string
		containerID     string
		selfContainerID string
		expectedIsSelf  bool
	}{
		{
			name:            "exact 64-char match",
			containerID:     fullID,
			selfContainerID: fullID,
			expectedIsSelf:  true,
		},
		{
			name:            "short container ID with full selfContainerID",
			containerID:     shortID,
			selfContainerID: fullID,
			expectedIsSelf:  true,
		},
		{
			name:            "full container ID with short selfContainerID",
			containerID:     fullID,
			selfContainerID: shortID,
			expectedIsSelf:  true,
		},
		{
			name:            "different ID",
			containerID:     otherID,
			selfContainerID: fullID,
			expectedIsSelf:  false,
		},
		{
			name:            "empty selfContainerID",
			containerID:     fullID,
			selfContainerID: "",
			expectedIsSelf:  false,
		},
		{
			name:            "empty containerID",
			containerID:     "",
			selfContainerID: fullID,
			expectedIsSelf:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := types.Container{ID: tt.containerID}
			summary := ConvertContainer(c, tt.selfContainerID)
			if summary.IsSelf != tt.expectedIsSelf {
				t.Errorf("expected IsSelf = %v, got %v", tt.expectedIsSelf, summary.IsSelf)
			}
		})
	}
}

func TestCategorizeContainers(t *testing.T) {
	selfID := "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1"
	managed := map[string]string{
		"my-stack": "/opt/stacks/my-stack/compose.yaml",
	}

	containers := []types.Container{
		{
			ID:    selfID,
			Names: []string{"/dokidoki"},
			State: "running",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "my-stack",
				docker.ComposeServiceLabel: "dokidoki",
			},
		},
		{
			ID:    "other123456789012",
			Names: []string{"/other-app"},
			State: "running",
		},
	}

	result := CategorizeContainers(containers, managed, selfID)
	if len(result.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(result.Stacks))
	}

	detail, ok := result.GetStack("my-stack")
	if !ok || len(detail.Containers) != 1 {
		t.Fatalf("expected 1 container in my-stack detail")
	}
	if !detail.Containers[0].IsSelf {
		t.Errorf("expected self container IsSelf=true")
	}

	if len(result.Standalone) != 1 {
		t.Fatalf("expected 1 standalone container")
	}
	if result.Standalone[0].IsSelf {
		t.Errorf("expected other container IsSelf=false")
	}
}

type mockDockerClient struct {
	listContainersFn   func(ctx context.Context) ([]types.Container, error)
	inspectContainerFn func(ctx context.Context, id string) (types.ContainerJSON, error)
	pingFn             func(ctx context.Context) (types.Ping, error)
	serverVersionFn    func(ctx context.Context) (types.Version, error)
	infoFn             func(ctx context.Context) (system.Info, error)
	createContainerFn  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	waitContainerFn    func(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	containerLogsFn    func(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error)
	removeContainerFn  func(ctx context.Context, id string, options container.RemoveOptions) error
}

func (m *mockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return types.Ping{}, nil
}

func (m *mockDockerClient) ListContainers(ctx context.Context) ([]types.Container, error) {
	if m.listContainersFn != nil {
		return m.listContainersFn(ctx)
	}
	return nil, nil
}

func (m *mockDockerClient) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if m.inspectContainerFn != nil {
		return m.inspectContainerFn(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.serverVersionFn != nil {
		return m.serverVersionFn(ctx)
	}
	return types.Version{}, nil
}

func (m *mockDockerClient) Info(ctx context.Context) (system.Info, error) {
	if m.infoFn != nil {
		return m.infoFn(ctx)
	}
	return system.Info{}, nil
}

func (m *mockDockerClient) RestartContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}

func (m *mockDockerClient) StartContainer(ctx context.Context, id string) error {
	return nil
}

func (m *mockDockerClient) StopContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}

func (m *mockDockerClient) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	if m.createContainerFn != nil {
		return m.createContainerFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{}, nil
}

func (m *mockDockerClient) WaitContainer(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	if m.waitContainerFn != nil {
		return m.waitContainerFn(ctx, id, condition)
	}
	resCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)
	resCh <- container.WaitResponse{StatusCode: 0}
	return resCh, errCh
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.containerLogsFn != nil {
		return m.containerLogsFn(ctx, id, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockDockerClient) RemoveContainer(ctx context.Context, id string, options container.RemoveOptions) error {
	if m.removeContainerFn != nil {
		return m.removeContainerFn(ctx, id, options)
	}
	return nil
}

func (m *mockDockerClient) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("pulled")), nil
}

func (m *mockDockerClient) Close() error {
	return nil
}

func setupTestService(t *testing.T, m *mockDockerClient, selfID string) (*Service, string) {
	tempDir := t.TempDir()

	// Create managed stack directory with compose.yaml
	managedDir := filepath.Join(tempDir, "my-app")
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		t.Fatalf("failed to create stack dir: %v", err)
	}
	composeContent := "services:\n  web:\n    image: nginx:latest\n"
	if err := os.WriteFile(filepath.Join(managedDir, "compose.yaml"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("failed to write compose.yaml: %v", err)
	}

	scanner := NewScanner(tempDir)
	svc := NewService(scanner, m, selfID)
	return svc, tempDir
}

func TestService_ListStacks(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyStacks", func(t *testing.T) {
		tempDir := t.TempDir()
		svc := NewService(NewScanner(tempDir), &mockDockerClient{}, "")
		stacks, err := svc.ListStacks(ctx, "all")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stacks == nil {
			t.Fatal("expected non-nil slice")
		}
		if len(stacks) != 0 {
			t.Fatalf("expected 0 stacks, got %d", len(stacks))
		}
	})

	t.Run("ManagedAndExternalFilters", func(t *testing.T) {
		m := &mockDockerClient{
			listContainersFn: func(ctx context.Context) ([]types.Container, error) {
				return []types.Container{
					{
						ID:    "c1",
						Names: []string{"/my-app-web-1"},
						State: "running",
						Labels: map[string]string{
							docker.ComposeProjectLabel: "my-app",
							docker.ComposeServiceLabel: "web",
						},
					},
					{
						ID:    "c2",
						Names: []string{"/ext-app-api-1"},
						State: "running",
						Labels: map[string]string{
							docker.ComposeProjectLabel: "ext-app",
							docker.ComposeServiceLabel: "api",
						},
					},
				}, nil
			},
		}

		svc, _ := setupTestService(t, m, "")

		// Source: all
		all, err := svc.ListStacks(ctx, "all")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("expected 2 stacks, got %d", len(all))
		}

		// Source: managed
		managed, err := svc.ListStacks(ctx, "managed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(managed) != 1 || managed[0].Name != "my-app" {
			t.Fatalf("expected 1 managed stack 'my-app', got %+v", managed)
		}

		// Source: external
		ext, err := svc.ListStacks(ctx, "external")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ext) != 1 || ext[0].Name != "ext-app" {
			t.Fatalf("expected 1 external stack 'ext-app', got %+v", ext)
		}

		// Invalid source
		_, err = svc.ListStacks(ctx, "unknown-source")
		if !errors.Is(err, ErrInvalidSource) {
			t.Fatalf("expected ErrInvalidSource, got %v", err)
		}
	})
}

func TestService_GetStack(t *testing.T) {
	ctx := context.Background()
	m := &mockDockerClient{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return []types.Container{
				{
					ID:    "c1",
					Names: []string{"/my-app-web-1"},
					State: "running",
					Labels: map[string]string{
						docker.ComposeProjectLabel: "my-app",
						docker.ComposeServiceLabel: "web",
					},
				},
			}, nil
		},
	}

	svc, _ := setupTestService(t, m, "")

	t.Run("Found", func(t *testing.T) {
		detail, err := svc.GetStack(ctx, "my-app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if detail.Name != "my-app" {
			t.Errorf("expected name 'my-app', got %s", detail.Name)
		}
		if len(detail.Containers) != 1 {
			t.Errorf("expected 1 container, got %d", len(detail.Containers))
		}
		if len(detail.Services) != 1 || detail.Services[0] != "web" {
			t.Errorf("expected services ['web'], got %+v", detail.Services)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := svc.GetStack(ctx, "non-existent")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestService_GetStackCompose(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService(t, &mockDockerClient{}, "")

	t.Run("Found", func(t *testing.T) {
		resp, err := svc.GetStackCompose(ctx, "my-app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Name != "my-app" {
			t.Errorf("expected name 'my-app', got %s", resp.Name)
		}
		if resp.Content == "" {
			t.Errorf("expected non-empty compose content")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := svc.GetStackCompose(ctx, "non-existent")
		if !errors.Is(err, ErrComposeNotFound) {
			t.Fatalf("expected ErrComposeNotFound, got %v", err)
		}
	})
}

func TestService_GetStackContainers(t *testing.T) {
	ctx := context.Background()
	m := &mockDockerClient{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return []types.Container{
				{
					ID:    "c1",
					Names: []string{"/my-app-web-1"},
					State: "running",
					Labels: map[string]string{
						docker.ComposeProjectLabel: "my-app",
						docker.ComposeServiceLabel: "web",
					},
				},
			}, nil
		},
	}

	svc, _ := setupTestService(t, m, "")

	t.Run("Found", func(t *testing.T) {
		containers, err := svc.GetStackContainers(ctx, "my-app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(containers) != 1 {
			t.Fatalf("expected 1 container, got %d", len(containers))
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := svc.GetStackContainers(ctx, "non-existent")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestService_ListContainers(t *testing.T) {
	ctx := context.Background()
	m := &mockDockerClient{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return []types.Container{
				{
					ID:    "c1",
					Names: []string{"/my-app-web-1"},
					State: "running",
					Labels: map[string]string{
						docker.ComposeProjectLabel: "my-app",
						docker.ComposeServiceLabel: "web",
					},
				},
				{
					ID:    "standalone-c2",
					Names: []string{"/standalone-app"},
					State: "running",
				},
			}, nil
		},
	}

	svc, _ := setupTestService(t, m, "")

	t.Run("Flat", func(t *testing.T) {
		res, err := svc.ListContainers(ctx, "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		containers, ok := res.([]docker.Container)
		if !ok {
			t.Fatalf("expected []docker.Container, got %T", res)
		}
		if len(containers) != 2 {
			t.Fatalf("expected 2 containers, got %d", len(containers))
		}
	})

	t.Run("Grouped", func(t *testing.T) {
		res, err := svc.ListContainers(ctx, "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		grouped, ok := res.(Grouped)
		if !ok {
			t.Fatalf("expected Grouped, got %T", res)
		}
		if len(grouped.Stacks["my-app"]) != 1 {
			t.Errorf("expected 1 container in my-app stack group")
		}
		if len(grouped.Standalone) != 1 {
			t.Errorf("expected 1 standalone container")
		}
	})

	t.Run("FilterByStack", func(t *testing.T) {
		res, err := svc.ListContainers(ctx, "my-app", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		containers, ok := res.([]docker.Container)
		if !ok {
			t.Fatalf("expected []docker.Container, got %T", res)
		}
		if len(containers) != 1 || containers[0].ID != "c1" {
			t.Fatalf("expected container c1, got %+v", containers)
		}

		// Unknown stack returns empty slice
		res, err = svc.ListContainers(ctx, "unknown-stack", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		containers, ok = res.([]docker.Container)
		if !ok {
			t.Fatalf("expected []docker.Container, got %T", res)
		}
		if containers == nil || len(containers) != 0 {
			t.Fatalf("expected empty non-nil slice, got %+v", containers)
		}
	})
}

func TestService_InspectContainer(t *testing.T) {
	ctx := context.Background()
	selfID := "self-cid-123456"

	m := &mockDockerClient{
		inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			switch id {
			case "c1":
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   "c1",
						Name: "/my-app-web-1",
					},
					Config: &container.Config{
						Labels: map[string]string{
							docker.ComposeProjectLabel: "my-app",
							docker.ComposeServiceLabel: "web",
						},
					},
				}, nil
			case "c-self":
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   selfID,
						Name: "/dokidoki-agent",
					},
				}, nil
			case "c-standalone":
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   "c-standalone",
						Name: "/solo",
					},
				}, nil
			default:
				return types.ContainerJSON{}, errdefs.NotFound(errors.New("container not found"))
			}
		},
	}

	svc, _ := setupTestService(t, m, selfID)

	t.Run("ManagedContainer", func(t *testing.T) {
		enriched, err := svc.InspectContainer(ctx, "c1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enriched.StackName != "my-app" || enriched.ServiceName != "web" || enriched.Source != "managed" {
			t.Fatalf("unexpected enriched data: %+v", enriched)
		}
		if enriched.IsSelf {
			t.Errorf("expected is_self=false")
		}
	})

	t.Run("SelfContainer", func(t *testing.T) {
		enriched, err := svc.InspectContainer(ctx, "c-self")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !enriched.IsSelf {
			t.Errorf("expected is_self=true")
		}
	})

	t.Run("StandaloneContainer", func(t *testing.T) {
		enriched, err := svc.InspectContainer(ctx, "c-standalone")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enriched.Source != "standalone" {
			t.Errorf("expected source 'standalone', got %s", enriched.Source)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := svc.InspectContainer(ctx, "nonexistent")
		if !errors.Is(err, ErrContainerNotFound) {
			t.Fatalf("expected ErrContainerNotFound, got %v", err)
		}
	})
}

func TestService_GetStackFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("ManagedStack", func(t *testing.T) {
		tempDir := t.TempDir()
		stackDir := filepath.Join(tempDir, "managed-app")
		if err := os.MkdirAll(stackDir, 0755); err != nil {
			t.Fatalf("failed to mkdir: %v", err)
		}

		_ = os.WriteFile(filepath.Join(stackDir, "compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
		_ = os.WriteFile(filepath.Join(stackDir, ".env"), []byte("PORT=80\n"), 0644)
		_ = os.WriteFile(filepath.Join(stackDir, ".env.production"), []byte("PORT=8080\n"), 0644)
		_ = os.WriteFile(filepath.Join(stackDir, "config.json"), []byte("{\"debug\": true}"), 0644)
		_ = os.WriteFile(filepath.Join(stackDir, ".hidden_ignore"), []byte("ignored"), 0644)

		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		resp, err := svc.GetStackFiles(ctx, "managed-app", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Stack != "managed-app" {
			t.Errorf("expected stack name 'managed-app', got %s", resp.Stack)
		}
		if len(resp.Files) != 4 {
			t.Fatalf("expected 4 files (excluding .hidden_ignore), got %d", len(resp.Files))
		}

		// Files should be sorted: compose first, then .env, then others
		composeFile := resp.Files[0]
		if composeFile.Name != "compose.yaml" || !composeFile.IsCompose || composeFile.IsEnv {
			t.Errorf("expected compose.yaml as first file, got %+v", composeFile)
		}
		if !strings.Contains(composeFile.Content, "image: nginx") {
			t.Errorf("expected compose file content, got %s", composeFile.Content)
		}

		envFile := resp.Files[1]
		if !envFile.IsEnv || envFile.IsCompose {
			t.Errorf("expected .env file with IsEnv=true, got %+v", envFile)
		}
	})

	t.Run("ExternalStack_NoForceRead", func(t *testing.T) {
		m := &mockDockerClient{
			listContainersFn: func(ctx context.Context) ([]types.Container, error) {
				return []types.Container{
					{
						ID:    "ext-c1",
						Names: []string{"/ext_web_1"},
						Labels: map[string]string{
							docker.ComposeProjectLabel:              "external-app",
							docker.ComposeServiceLabel:              "web",
							"com.docker.compose.project.working_dir": "/host/external/path",
						},
					},
				}, nil
			},
		}

		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, m, "")

		resp, err := svc.GetStackFiles(ctx, "external-app", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Dir != "/host/external/path" {
			t.Errorf("expected dir /host/external/path, got %s", resp.Dir)
		}
		if len(resp.Files) != 0 {
			t.Fatalf("expected 0 files when forceRead=false, got %d", len(resp.Files))
		}
	})

	t.Run("ExternalStack_ForceRead_BareMetal", func(t *testing.T) {
		extDir := t.TempDir()
		realComposePath := filepath.Join(extDir, "docker-compose.yml")
		_ = os.WriteFile(realComposePath, []byte("services:\n  ext:\n    image: alpine\n"), 0644)
		_ = os.WriteFile(filepath.Join(extDir, ".env"), []byte("FOO=BAR\n"), 0644)

		m := &mockDockerClient{
			listContainersFn: func(ctx context.Context) ([]types.Container, error) {
				return []types.Container{
					{
						ID:    "ext-c2",
						Names: []string{"/ext2_web_1"},
						Labels: map[string]string{
							docker.ComposeProjectLabel:              "accessible-ext",
							docker.ComposeServiceLabel:              "web",
							"com.docker.compose.project.working_dir": extDir,
						},
					},
				}, nil
			},
		}

		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, m, "") // bare-metal

		resp, err := svc.GetStackFiles(ctx, "accessible-ext", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Dir != extDir {
			t.Errorf("expected dir %s, got %s", extDir, resp.Dir)
		}
		if len(resp.Files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(resp.Files))
		}
		if !strings.Contains(resp.Files[0].Content, "image: alpine") {
			t.Errorf("expected compose file content, got %s", resp.Files[0].Content)
		}
	})

	t.Run("ExternalStack_ForceRead_Containerized_Success", func(t *testing.T) {
		removedContainerID := ""
		m := &mockDockerClient{
			listContainersFn: func(ctx context.Context) ([]types.Container, error) {
				return []types.Container{
					{
						ID:    "ext-c3",
						Names: []string{"/ext3_web_1"},
						Labels: map[string]string{
							docker.ComposeProjectLabel:              "ext-containerized",
							docker.ComposeServiceLabel:              "web",
							"com.docker.compose.project.working_dir": "/host/my-stack",
						},
					},
				}, nil
			},
			inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				if id == "self-cid-123" {
					return types.ContainerJSON{
						Config: &container.Config{
							Image:      "dokidoki:v0.1.0",
							Entrypoint: []string{"dokidoki"},
						},
					}, nil
				}
				return types.ContainerJSON{}, ErrContainerNotFound
			},
			createContainerFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
				if config.Image != "dokidoki:v0.1.0" {
					t.Errorf("expected image dokidoki:v0.1.0, got %s", config.Image)
				}
				if len(config.Cmd) < 2 || config.Cmd[0] != "read-files" || config.Cmd[1] != "/mnt/target" {
					t.Errorf("unexpected cmd: %+v", config.Cmd)
				}
				if len(hostConfig.Binds) != 1 || hostConfig.Binds[0] != "/host/my-stack:/mnt/target:ro" {
					t.Errorf("unexpected binds: %+v", hostConfig.Binds)
				}
				return container.CreateResponse{ID: "temp-helper-cid"}, nil
			},
			waitContainerFn: func(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
				resCh := make(chan container.WaitResponse, 1)
				errCh := make(chan error, 1)
				resCh <- container.WaitResponse{StatusCode: 0}
				return resCh, errCh
			},
			containerLogsFn: func(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
				filesJSON := `[{"name":"compose.yaml","path":"/mnt/target/compose.yaml","size":25,"content":"services:\n  web:\n    image: nginx","isCompose":true,"isEnv":false}]`
				return io.NopCloser(strings.NewReader(filesJSON)), nil
			},
			removeContainerFn: func(ctx context.Context, id string, options container.RemoveOptions) error {
				removedContainerID = id
				return nil
			},
		}

		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, m, "self-cid-123")

		resp, err := svc.GetStackFiles(ctx, "ext-containerized", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(resp.Files))
		}
		if resp.Files[0].Name != "compose.yaml" || !resp.Files[0].IsCompose {
			t.Errorf("unexpected file: %+v", resp.Files[0])
		}
		if resp.Files[0].Path != "/host/my-stack/compose.yaml" {
			t.Errorf("expected path to be rewritten to hostDir, got %s", resp.Files[0].Path)
		}
		if removedContainerID != "temp-helper-cid" {
			t.Errorf("expected temp container to be cleaned up, got removedContainerID=%q", removedContainerID)
		}
	})

	t.Run("ExternalStack_ForceRead_Containerized_ErrorExit", func(t *testing.T) {
		removedContainerID := ""
		m := &mockDockerClient{
			listContainersFn: func(ctx context.Context) ([]types.Container, error) {
				return []types.Container{
					{
						ID:    "ext-c4",
						Names: []string{"/ext4_web_1"},
						Labels: map[string]string{
							docker.ComposeProjectLabel:              "ext-fail",
							docker.ComposeServiceLabel:              "web",
							"com.docker.compose.project.working_dir": "/host/fail-stack",
						},
					},
				}, nil
			},
			inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				return types.ContainerJSON{
					Config: &container.Config{Image: "dokidoki:v0.1.0"},
				}, nil
			},
			createContainerFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
				return container.CreateResponse{ID: "fail-helper-cid"}, nil
			},
			waitContainerFn: func(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
				resCh := make(chan container.WaitResponse, 1)
				errCh := make(chan error, 1)
				resCh <- container.WaitResponse{StatusCode: 1}
				return resCh, errCh
			},
			containerLogsFn: func(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("permission denied")), nil
			},
			removeContainerFn: func(ctx context.Context, id string, options container.RemoveOptions) error {
				removedContainerID = id
				return nil
			},
		}

		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, m, "self-cid-123")

		_, err := svc.GetStackFiles(ctx, "ext-fail", true)
		if err == nil {
			t.Fatal("expected error on non-zero container exit, got nil")
		}
		if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("expected error details containing status code and logs, got %v", err)
		}
		if removedContainerID != "fail-helper-cid" {
			t.Errorf("expected container to be cleaned up on error, got %q", removedContainerID)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		_, err := svc.GetStackFiles(ctx, "unknown-stack", false)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestService_GetStackFile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	stackDir := filepath.Join(tempDir, "my-app")
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	_ = os.WriteFile(filepath.Join(stackDir, "compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
	_ = os.WriteFile(filepath.Join(stackDir, ".env"), []byte("KEY=VALUE\n"), 0644)

	m := &mockDockerClient{
		listContainersFn: func(ctx context.Context) ([]types.Container, error) {
			return []types.Container{
				{
					ID:    "ext-c1",
					Names: []string{"/ext_web_1"},
					Labels: map[string]string{
						docker.ComposeProjectLabel:              "external-app",
						docker.ComposeServiceLabel:              "web",
						"com.docker.compose.project.config_files": "/opt/outside/compose.yaml",
					},
				},
			}, nil
		},
	}

	scanner := NewScanner(tempDir)
	svc := NewService(scanner, m, "")

	t.Run("FoundManaged", func(t *testing.T) {
		f, err := svc.GetStackFile(ctx, "my-app", "compose.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Name != "compose.yaml" || !f.IsCompose || !strings.Contains(f.Content, "image: nginx") {
			t.Errorf("unexpected file result: %+v", f)
		}
	})

	t.Run("FoundEnv", func(t *testing.T) {
		f, err := svc.GetStackFile(ctx, "my-app", ".env")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Name != ".env" || !f.IsEnv || !strings.Contains(f.Content, "KEY=VALUE") {
			t.Errorf("unexpected file result: %+v", f)
		}
	})

	t.Run("PathTraversalBlocked", func(t *testing.T) {
		_, err := svc.GetStackFile(ctx, "my-app", "../../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path traversal, got nil")
		}
		if !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("expected path traversal error message, got %v", err)
		}

		_, err = svc.GetStackFile(ctx, "../../etc", "passwd")
		if err == nil {
			t.Fatal("expected error for stack name path traversal, got nil")
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		_, err := svc.GetStackFile(ctx, "my-app", "nonexistent.yaml")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ExternalStack_StubbedCompose", func(t *testing.T) {
		f, err := svc.GetStackFile(ctx, "external-app", "compose.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Name != "compose.yaml" || !f.IsCompose {
			t.Errorf("expected stubbed compose.yaml, got %+v", f)
		}
		if !strings.Contains(f.Content, "# External stack detected: external-app") {
			t.Errorf("expected stub content, got %s", f.Content)
		}

		// Other files on external stack should not be found
		_, err = svc.GetStackFile(ctx, "external-app", ".env")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound for non-compose on external stack, got %v", err)
		}
	})
}

func TestService_CreateOrImportStack(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateWithFiles", func(t *testing.T) {
		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		req := CreateStackRequest{
			Name: "new-custom-stack",
			Files: []CreateStackFile{
				{
					Name:    "compose.yaml",
					Content: "services:\n  redis:\n    image: redis:alpine\n",
				},
				{
					Name:    ".env",
					Content: "PORT=6379\n",
				},
			},
		}

		summary, err := svc.CreateOrImportStack(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if summary.Name != "new-custom-stack" || summary.Source != string(SourceManaged) || !summary.ComposePresent {
			t.Errorf("unexpected summary: %+v", summary)
		}

		// Verify files written
		composeData, err := os.ReadFile(filepath.Join(tempDir, "new-custom-stack", "compose.yaml"))
		if err != nil {
			t.Fatalf("failed to read created compose file: %v", err)
		}
		if !strings.Contains(string(composeData), "image: redis:alpine") {
			t.Errorf("unexpected file content: %s", string(composeData))
		}

		envData, err := os.ReadFile(filepath.Join(tempDir, "new-custom-stack", ".env"))
		if err != nil {
			t.Fatalf("failed to read created .env file: %v", err)
		}
		if !strings.Contains(string(envData), "PORT=6379") {
			t.Errorf("unexpected .env content: %s", string(envData))
		}
	})

	t.Run("CreateWithoutCompose_WritesDefaultCompose", func(t *testing.T) {
		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		req := CreateStackRequest{
			Name: "starter-stack",
			Files: []CreateStackFile{
				{
					Name:    ".env",
					Content: "KEY=VALUE\n",
				},
			},
		}

		summary, err := svc.CreateOrImportStack(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if summary.Name != "starter-stack" || summary.Source != string(SourceManaged) {
			t.Errorf("unexpected summary: %+v", summary)
		}

		data, err := os.ReadFile(filepath.Join(tempDir, "starter-stack", "compose.yaml"))
		if err != nil {
			t.Fatalf("failed to read created compose file: %v", err)
		}
		if !strings.Contains(string(data), "nginx:alpine") {
			t.Errorf("expected starter template, got %s", string(data))
		}
	})

	t.Run("PathTraversalInFileName", func(t *testing.T) {
		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		traversalReqs := []CreateStackRequest{
			{Name: "valid-name", Files: []CreateStackFile{{Name: "../evil", Content: "bad"}}},
			{Name: "valid-name", Files: []CreateStackFile{{Name: "/etc/passwd", Content: "bad"}}},
			{Name: "valid-name", Files: []CreateStackFile{{Name: "sub\\file", Content: "bad"}}},
			{Name: "valid-name", Files: []CreateStackFile{{Name: "sub/file", Content: "bad"}}},
			{Name: "valid-name", Files: []CreateStackFile{{Name: "", Content: "bad"}}},
			{Name: "valid-name", Files: []CreateStackFile{{Name: ".", Content: "bad"}}},
		}

		for _, req := range traversalReqs {
			_, err := svc.CreateOrImportStack(ctx, req)
			if err == nil {
				t.Errorf("expected error for file name %q, got nil", req.Files[0].Name)
			}
		}
	})

	t.Run("EmptyFiles", func(t *testing.T) {
		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		_, err := svc.CreateOrImportStack(ctx, CreateStackRequest{Name: "no-files", Files: []CreateStackFile{}})
		if err == nil {
			t.Error("expected error for empty files slice, got nil")
		}
	})

	t.Run("InvalidName", func(t *testing.T) {
		tempDir := t.TempDir()
		scanner := NewScanner(tempDir)
		svc := NewService(scanner, &mockDockerClient{}, "")

		_, err := svc.CreateOrImportStack(ctx, CreateStackRequest{Name: "", Files: []CreateStackFile{{Name: "compose.yaml", Content: "test"}}})
		if err == nil {
			t.Error("expected error for empty name")
		}

		_, err = svc.CreateOrImportStack(ctx, CreateStackRequest{Name: "../traversal", Files: []CreateStackFile{{Name: "compose.yaml", Content: "test"}}})
		if err == nil {
			t.Error("expected error for path traversal name")
		}

		_, err = svc.CreateOrImportStack(ctx, CreateStackRequest{Name: "bad name with spaces", Files: []CreateStackFile{{Name: "compose.yaml", Content: "test"}}})
		if err == nil {
			t.Error("expected error for invalid characters")
		}
	})
}

func TestService_GetContainerCompose(t *testing.T) {
	ctx := context.Background()

	m := &mockDockerClient{
		inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			if id == "redis-container" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID:   "redis-container",
						Name: "/my_redis",
						HostConfig: &container.HostConfig{
							RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
							PortBindings: nat.PortMap{
								"6379/tcp": []nat.PortBinding{{HostPort: "6379"}},
							},
							Binds: []string{"/host/data:/data"},
						},
					},
					Config: &container.Config{
						Image: "redis:7.0-alpine",
						Cmd:   []string{"redis-server", "--appendonly", "yes"},
						Env:   []string{"REDIS_PASSWORD=secret"},
						Labels: map[string]string{
							docker.ComposeProjectLabel: "redis-stack",
							docker.ComposeServiceLabel: "redis",
						},
					},
				}, nil
			}
			return types.ContainerJSON{}, errdefs.NotFound(errors.New("no such container"))
		},
	}

	tempDir := t.TempDir()
	scanner := NewScanner(tempDir)
	svc := NewService(scanner, m, "")

	t.Run("Success", func(t *testing.T) {
		resp, err := svc.GetContainerCompose(ctx, "redis-container")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.StackName != "redis-stack" {
			t.Errorf("expected stack name redis-stack, got %s", resp.StackName)
		}

		content := resp.Content
		if !strings.Contains(content, "image: redis:7.0-alpine") {
			t.Errorf("missing image in compose: %s", content)
		}
		if !strings.Contains(content, "redis-server --appendonly yes") {
			t.Errorf("missing command in compose: %s", content)
		}
		if !strings.Contains(content, "restart: unless-stopped") {
			t.Errorf("missing restart in compose: %s", content)
		}
		if !strings.Contains(content, "6379:6379") {
			t.Errorf("missing port in compose: %s", content)
		}
		if !strings.Contains(content, "REDIS_PASSWORD=secret") {
			t.Errorf("missing env in compose: %s", content)
		}
		if !strings.Contains(content, "/host/data:/data") {
			t.Errorf("missing volume bind in compose: %s", content)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := svc.GetContainerCompose(ctx, "nonexistent")
		if !errors.Is(err, ErrContainerNotFound) {
			t.Fatalf("expected ErrContainerNotFound, got %v", err)
		}
	})
}

type mockComposeRunner struct {
	upFn      func(ctx context.Context, path string, out io.Writer, extraArgs ...string) error
	downFn    func(ctx context.Context, path string, out io.Writer) error
	restartFn func(ctx context.Context, path string, service string, out io.Writer) error
	pullFn    func(ctx context.Context, path string, out io.Writer) error
}

func (m *mockComposeRunner) Up(ctx context.Context, path string, out io.Writer, extraArgs ...string) error {
	if m.upFn != nil {
		return m.upFn(ctx, path, out, extraArgs...)
	}
	if out != nil {
		_, _ = out.Write([]byte("up success\n"))
	}
	return nil
}

func (m *mockComposeRunner) Down(ctx context.Context, path string, out io.Writer) error {
	if m.downFn != nil {
		return m.downFn(ctx, path, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("down success\n"))
	}
	return nil
}

func (m *mockComposeRunner) Restart(ctx context.Context, path string, service string, out io.Writer) error {
	if m.restartFn != nil {
		return m.restartFn(ctx, path, service, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("restart success\n"))
	}
	return nil
}

func (m *mockComposeRunner) Pull(ctx context.Context, path string, out io.Writer) error {
	if m.pullFn != nil {
		return m.pullFn(ctx, path, out)
	}
	if out != nil {
		_, _ = out.Write([]byte("pull success\n"))
	}
	return nil
}

func TestService_ContainerOperations(t *testing.T) {
	ctx := context.Background()
	selfID := "self-container-123"

	mockDocker := &mockDockerClient{
		inspectContainerFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			if id == "app-container" {
				return types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{ID: "app-container", Name: "/app"},
					Config:            &container.Config{Image: "redis:alpine"},
				}, nil
			}
			return types.ContainerJSON{}, errdefs.NotFound(errors.New("no such container"))
		},
	}

	svc := NewService(nil, mockDocker, selfID)

	t.Run("RestartContainer_SelfSafeguard", func(t *testing.T) {
		// Attempting restart on self container without force should fail
		_, err := svc.RestartContainer(ctx, selfID, false)
		if !errors.Is(err, ErrSelfContainer) {
			t.Fatalf("expected ErrSelfContainer, got %v", err)
		}

		// With force should succeed
		res, err := svc.RestartContainer(ctx, selfID, true)
		if err != nil {
			t.Fatalf("expected success with force, got %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("StopContainer_SelfSafeguard", func(t *testing.T) {
		_, err := svc.StopContainer(ctx, selfID, false)
		if !errors.Is(err, ErrSelfContainer) {
			t.Fatalf("expected ErrSelfContainer, got %v", err)
		}

		res, err := svc.StopContainer(ctx, selfID, true)
		if err != nil {
			t.Fatalf("expected success with force, got %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("StartContainer", func(t *testing.T) {
		res, err := svc.StartContainer(ctx, "app-container")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("PullContainerImage_Stream", func(t *testing.T) {
		var out strings.Builder
		res, err := svc.PullContainerImage(ctx, "app-container", &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
		if out.String() != "pulled" {
			t.Errorf("expected streamed output 'pulled', got %q", out.String())
		}
	})
}

func TestService_ComposeOperations(t *testing.T) {
	ctx := context.Background()
	mockDocker := &mockDockerClient{}
	mockRunner := &mockComposeRunner{}

	svc, tempDir := setupTestService(t, mockDocker, "")
	svc.composeRunner = mockRunner

	// Create a test stack directory with compose.yaml
	stackDir := filepath.Join(tempDir, "mystack")
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(stackDir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte("services:\n  app:\n    image: alpine\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("ComposeUp_Stream", func(t *testing.T) {
		var out strings.Builder
		res, err := svc.ComposeUp(ctx, "mystack", &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
		if !strings.Contains(out.String(), "up success") {
			t.Errorf("expected 'up success' in stream, got %q", out.String())
		}
	})

	t.Run("ComposeDown", func(t *testing.T) {
		res, err := svc.ComposeDown(ctx, "mystack", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("ComposeRestart", func(t *testing.T) {
		res, err := svc.ComposeRestart(ctx, "mystack", "app", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("ComposeRestart_PendingImportDelegatesToUp", func(t *testing.T) {
		var upCalled bool
		var extraPassed []string
		mockR := &mockComposeRunner{
			upFn: func(ctx context.Context, path string, out io.Writer, extraArgs ...string) error {
				upCalled = true
				extraPassed = extraArgs
				return nil
			},
		}
		mockDocker := &mockDockerClient{
			listContainersFn: func(ctx context.Context) ([]types.Container, error) {
				return []types.Container{
					{
						ID:    "c-pending",
						Names: []string{"/mystack-web-1"},
						State: "running",
						Labels: map[string]string{
							"com.docker.compose.project":             "mystack",
							"com.docker.compose.project.working_dir": "/external/dir/mystack",
						},
					},
				}, nil
			},
		}
		svcWithPending := NewService(NewScanner(tempDir), mockDocker, "", WithComposeRunner(mockR))
		res, err := svcWithPending.ComposeRestart(ctx, "mystack", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
		if !upCalled {
			t.Errorf("expected ComposeRestart on pending import to call Up")
		}
		var hasForceRecreate bool
		for _, arg := range extraPassed {
			if arg == "--force-recreate" {
				hasForceRecreate = true
			}
		}
		if !hasForceRecreate {
			t.Errorf("expected --force-recreate to be passed to Up on pending import stack, got %v", extraPassed)
		}
	})

	t.Run("ComposePull", func(t *testing.T) {
		res, err := svc.ComposePull(ctx, "mystack", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("StackNotFound", func(t *testing.T) {
		_, err := svc.ComposeUp(ctx, "unknown-stack", nil)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestService_ContainerLogs(t *testing.T) {
	ctx := context.Background()

	t.Run("nil docker client returns ErrContainerNotFound", func(t *testing.T) {
		svc := NewService(nil, nil, "")
		_, err := svc.ContainerLogs(ctx, "cid-1", container.LogsOptions{})
		if !errors.Is(err, ErrContainerNotFound) {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})

	t.Run("not found error maps to ErrContainerNotFound", func(t *testing.T) {
		m := &mockDockerClient{
			containerLogsFn: func(ctx context.Context, id string, opts container.LogsOptions) (io.ReadCloser, error) {
				return nil, errdefs.NotFound(errors.New("no such container"))
			},
		}
		svc := NewService(nil, m, "")
		_, err := svc.ContainerLogs(ctx, "missing", container.LogsOptions{})
		if !errors.Is(err, ErrContainerNotFound) {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})

	t.Run("'no such container' error message maps to ErrContainerNotFound", func(t *testing.T) {
		m := &mockDockerClient{
			containerLogsFn: func(ctx context.Context, id string, opts container.LogsOptions) (io.ReadCloser, error) {
				return nil, errors.New("Error: No such container: abc123")
			},
		}
		svc := NewService(nil, m, "")
		_, err := svc.ContainerLogs(ctx, "abc123", container.LogsOptions{})
		if !errors.Is(err, ErrContainerNotFound) {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})

	t.Run("other error propagated as-is", func(t *testing.T) {
		want := errors.New("docker daemon unreachable")
		m := &mockDockerClient{
			containerLogsFn: func(ctx context.Context, id string, opts container.LogsOptions) (io.ReadCloser, error) {
				return nil, want
			},
		}
		svc := NewService(nil, m, "")
		_, err := svc.ContainerLogs(ctx, "cid-1", container.LogsOptions{})
		if !errors.Is(err, want) {
			t.Errorf("expected propagated error %v, got %v", want, err)
		}
	})

	t.Run("success returns the reader", func(t *testing.T) {
		want := io.NopCloser(strings.NewReader("hello"))
		m := &mockDockerClient{
			containerLogsFn: func(ctx context.Context, id string, opts container.LogsOptions) (io.ReadCloser, error) {
				return want, nil
			},
		}
		svc := NewService(nil, m, "")
		rc, err := svc.ContainerLogs(ctx, "cid-1", container.LogsOptions{Follow: true, Tail: "100"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rc != want {
			t.Errorf("expected returned reader to match mock")
		}
	})
}
