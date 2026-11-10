package task

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockDockerClient struct {
	createContainerFunc func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	startContainerFunc  func(ctx context.Context, id string) error
	inspectContainerFunc func(ctx context.Context, id string) (types.ContainerJSON, error)
}

func (m *mockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockDockerClient) ListContainers(ctx context.Context) ([]types.Container, error) {
	return nil, nil
}

func (m *mockDockerClient) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if m.inspectContainerFunc != nil {
		return m.inspectContainerFunc(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func (m *mockDockerClient) RestartContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}

func (m *mockDockerClient) StartContainer(ctx context.Context, id string) error {
	if m.startContainerFunc != nil {
		return m.startContainerFunc(ctx, id)
	}
	return nil
}

func (m *mockDockerClient) StopContainer(ctx context.Context, id string, timeout *int) error {
	return nil
}

func (m *mockDockerClient) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	if m.createContainerFunc != nil {
		return m.createContainerFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "helper-c1"}, nil
}

func (m *mockDockerClient) WaitContainer(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return nil, nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDockerClient) RemoveContainer(ctx context.Context, id string, options container.RemoveOptions) error {
	return nil
}

func (m *mockDockerClient) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{}, nil
}

func (m *mockDockerClient) Info(ctx context.Context) (system.Info, error) {
	return system.Info{}, nil
}

func (m *mockDockerClient) Close() error {
	return nil
}

func TestContainerTaskRunner_StartAndStatus(t *testing.T) {
	tmpDir := t.TempDir()

	var capturedConfig *container.Config
	var capturedHostConfig *container.HostConfig
	var capturedName string

	mockCli := &mockDockerClient{
		createContainerFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
			capturedConfig = config
			capturedHostConfig = hostConfig
			capturedName = containerName
			return container.CreateResponse{ID: "task-cid-123"}, nil
		},
		startContainerFunc: func(ctx context.Context, id string) error {
			if id != "task-cid-123" {
				t.Errorf("unexpected start id: %s", id)
			}
			return nil
		},
	}

	runner := NewContainerTaskRunner(mockCli, tmpDir, WithDockerSocket("/custom/docker.sock"), WithDefaultImage("docker:24-cli"))

	ctx := context.Background()
	task, err := runner.StartTask(ctx, Config{
		ID:          "task-1",
		Description: "Self-update task",
		Command:     []string{"docker", "compose", "-f", "compose.yaml", "up", "-d"},
		WorkingDir:  filepath.Join(tmpDir, "dokidoki"),
	})

	if err != nil {
		t.Fatalf("unexpected StartTask error: %v", err)
	}

	if task.ID != "task-1" {
		t.Errorf("expected ID task-1, got %s", task.ID)
	}
	if task.Status != StatusRunning {
		t.Errorf("expected status running, got %s", task.Status)
	}
	if capturedName != "dokidoki-task-task-1" {
		t.Errorf("expected container name dokidoki-task-task-1, got %s", capturedName)
	}
	if capturedConfig.Image != "docker:24-cli" {
		t.Errorf("expected default image docker:24-cli, got %s", capturedConfig.Image)
	}
	if capturedConfig.WorkingDir != filepath.Join(tmpDir, "dokidoki") {
		t.Errorf("expected working dir %s, got %s", filepath.Join(tmpDir, "dokidoki"), capturedConfig.WorkingDir)
	}
	if !capturedHostConfig.AutoRemove {
		t.Errorf("expected AutoRemove=true")
	}

	// Verify mounts
	hasSocket := false
	hasStacks := false
	for _, bind := range capturedHostConfig.Binds {
		if bind == "/custom/docker.sock:/var/run/docker.sock" {
			hasSocket = true
		}
		if bind == fmt.Sprintf("%s:%s", filepath.Clean(tmpDir), filepath.Clean(tmpDir)) {
			hasStacks = true
		}
	}
	if !hasSocket {
		t.Errorf("missing socket bind mount: %v", capturedHostConfig.Binds)
	}
	if !hasStacks {
		t.Errorf("missing stacks bind mount: %v", capturedHostConfig.Binds)
	}

	// Check status reading while running
	fetched, err := runner.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if fetched.Status != StatusRunning {
		t.Errorf("expected status running, got %s", fetched.Status)
	}

	// Simulate container completion: write exitcode
	exitCodeFile := filepath.Join(runner.TasksDir(), "task-1.status.exitcode")
	if err := os.WriteFile(exitCodeFile, []byte("0\n"), 0644); err != nil {
		t.Fatalf("failed to write exitcode: %v", err)
	}

	fetchedCompleted, err := runner.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if fetchedCompleted.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", fetchedCompleted.Status)
	}
	if fetchedCompleted.ExitCode == nil || *fetchedCompleted.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %v", fetchedCompleted.ExitCode)
	}
}

func TestContainerTaskRunner_TailLogs(t *testing.T) {
	tmpDir := t.TempDir()
	mockCli := &mockDockerClient{}
	runner := NewContainerTaskRunner(mockCli, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	task, err := runner.StartTask(ctx, Config{
		ID:      "task-tail",
		Command: []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("StartTask failed: %v", err)
	}

	// Write some logs asynchronously
	go func() {
		time.Sleep(50 * time.Millisecond)
		logPath := runner.taskLogPath(task.ID)
		_ = os.WriteFile(logPath, []byte("Line 1\nLine 2\n"), 0644)

		time.Sleep(50 * time.Millisecond)
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
		if f != nil {
			_, _ = f.WriteString("Line 3\n")
			_ = f.Close()
		}

		time.Sleep(50 * time.Millisecond)
		exitFile := runner.taskStatusPath(task.ID) + ".exitcode"
		_ = os.WriteFile(exitFile, []byte("0\n"), 0644)
	}()

	var buf bytes.Buffer
	if err := runner.TailTaskLogs(ctx, task.ID, &buf, true); err != nil {
		t.Fatalf("TailTaskLogs error: %v", err)
	}

	output := buf.String()
	if output != "Line 1\nLine 2\nLine 3\n" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestContainerTaskRunner_ListAndPrune(t *testing.T) {
	tmpDir := t.TempDir()
	mockCli := &mockDockerClient{}
	runner := NewContainerTaskRunner(mockCli, tmpDir)

	ctx := context.Background()

	t1, _ := runner.StartTask(ctx, Config{ID: "task-old"})
	t2, _ := runner.StartTask(ctx, Config{ID: "task-new"})

	// Mark both complete
	_ = os.WriteFile(runner.taskStatusPath(t1.ID)+".exitcode", []byte("0\n"), 0644)
	_ = os.WriteFile(runner.taskStatusPath(t2.ID)+".exitcode", []byte("0\n"), 0644)
	_ = os.WriteFile(runner.taskLogPath(t1.ID), []byte("log1"), 0644)
	_ = os.WriteFile(runner.taskLogPath(t2.ID), []byte("log2"), 0644)

	// Backdate t1
	oldInfo, _ := runner.GetTask(ctx, t1.ID)
	oldTime := time.Now().Add(-2 * time.Hour)
	oldInfo.CreatedAt = oldTime
	oldInfo.FinishedAt = &oldTime
	_ = runner.saveTaskStatus(oldInfo)

	list, err := runner.ListTasks(ctx)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}

	// Prune tasks older than 1 hour
	pruned, err := runner.PruneTasks(ctx, 1*time.Hour)
	if err != nil {
		t.Fatalf("PruneTasks failed: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 task pruned, got %d", pruned)
	}

	// Check that t1 files are gone
	if _, err := runner.GetTask(ctx, t1.ID); !os.IsNotExist(err) {
		t.Errorf("expected t1 to not exist after pruning, got %v", err)
	}
	// Check that t2 remains
	if _, err := runner.GetTask(ctx, t2.ID); err != nil {
		t.Errorf("expected t2 to still exist, got %v", err)
	}
}
