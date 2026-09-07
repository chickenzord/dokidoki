package stacks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chickenzord/dokidoki/internal/model"
	"github.com/docker/docker/api/types"
)

func TestCalculateRollup(t *testing.T) {
	tests := []struct {
		name       string
		containers []model.ContainerSummary
		expected   model.ContainerRollup
	}{
		{
			name:       "empty containers",
			containers: []model.ContainerSummary{},
			expected:   model.ContainerRollup{},
		},
		{
			name: "various states with case insensitivity",
			containers: []model.ContainerSummary{
				{State: "running"},
				{State: "RUNNING"},
				{State: "exited"},
				{State: "Exited"},
				{State: "restarting"},
				{State: "paused"},
				{State: "dead"},
				{State: "created"}, // Not one of the 5 specific states, but increments Total
			},
			expected: model.ContainerRollup{
				Running:    2,
				Exited:     2,
				Restarting: 1,
				Paused:     1,
				Dead:       1,
				Total:      8,
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

	containers := []model.ContainerSummary{
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
	if s1.Name != "app-one" || s1.Source != model.StackSourceManaged || !s1.ComposePresent {
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
	if s2.Name != "app-two" || s2.Source != model.StackSourceManaged || !s2.ComposePresent {
		t.Errorf("unexpected s2 summary: %+v", s2)
	}
	if s2.Rollup != (model.ContainerRollup{}) {
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

func TestCategorize_ExternalStacks(t *testing.T) {
	// No stacks discovered on filesystem
	discovered := []DiscoveredStack{}

	containers := []model.ContainerSummary{
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
	if s.Source != model.StackSourceExternal {
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

	containers := []model.ContainerSummary{
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
				model.ComposeProjectLabel: "managed-active",
				model.ComposeServiceLabel: "worker",
			},
		},
		{
			ID:    "c_ext_1",
			Names: []string{"/coolify_proxy_1"},
			State: "running",
			Labels: map[string]string{
				model.ComposeProjectLabel: "coolify-proxy",
				model.ComposeServiceLabel: "proxy",
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
	if result.Stacks[0].Name != "coolify-proxy" || result.Stacks[0].Source != model.StackSourceExternal {
		t.Errorf("expected Stacks[0] to be coolify-proxy (external), got %+v", result.Stacks[0])
	}
	if result.Stacks[1].Name != "managed-active" || result.Stacks[1].Source != model.StackSourceManaged {
		t.Errorf("expected Stacks[1] to be managed-active (managed), got %+v", result.Stacks[1])
	}
	if result.Stacks[2].Name != "managed-idle" || result.Stacks[2].Source != model.StackSourceManaged {
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
			model.ComposeProjectLabel: "my-stack",
			model.ComposeServiceLabel: "redis",
			"custom.label":            "value",
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
				model.ComposeProjectLabel: "my-stack",
				model.ComposeServiceLabel: "dokidoki",
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
