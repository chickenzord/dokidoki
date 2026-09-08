package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Ensure ClientWrapper and MockClient implement Client interface at compile time.
var _ Client = (*ClientWrapper)(nil)
var _ Client = (*MockClient)(nil)

func TestNewClient(t *testing.T) {
	// Test creating client with default environment
	cli, err := NewClient("")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer cli.Close()

	// Test creating client with custom host
	cliCustom, err := NewClient("tcp://127.0.0.1:2375")
	if err != nil {
		t.Fatalf("failed to create client with custom host: %v", err)
	}
	defer cliCustom.Close()
}

// MockClient is a mock implementation of Client for testing other packages.
type MockClient struct {
	PingFunc             func(ctx context.Context) (types.Ping, error)
	ListContainersFunc   func(ctx context.Context) ([]types.Container, error)
	InspectContainerFunc func(ctx context.Context, id string) (types.ContainerJSON, error)
	RestartContainerFunc func(ctx context.Context, id string, timeout *int) error
	StartContainerFunc   func(ctx context.Context, id string) error
	StopContainerFunc    func(ctx context.Context, id string, timeout *int) error
	CreateContainerFunc  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	WaitContainerFunc    func(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerLogsFunc    func(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error)
	RemoveContainerFunc  func(ctx context.Context, id string, options container.RemoveOptions) error
	PullImageFunc        func(ctx context.Context, imageRef string) (io.ReadCloser, error)
	ServerVersionFunc    func(ctx context.Context) (types.Version, error)
	InfoFunc             func(ctx context.Context) (system.Info, error)
	CloseFunc            func() error
}

func (m *MockClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return types.Ping{}, nil
}

func (m *MockClient) ListContainers(ctx context.Context) ([]types.Container, error) {
	if m.ListContainersFunc != nil {
		return m.ListContainersFunc(ctx)
	}
	return nil, nil
}

func (m *MockClient) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	if m.InspectContainerFunc != nil {
		return m.InspectContainerFunc(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func (m *MockClient) RestartContainer(ctx context.Context, id string, timeout *int) error {
	if m.RestartContainerFunc != nil {
		return m.RestartContainerFunc(ctx, id, timeout)
	}
	return nil
}

func (m *MockClient) StartContainer(ctx context.Context, id string) error {
	if m.StartContainerFunc != nil {
		return m.StartContainerFunc(ctx, id)
	}
	return nil
}

func (m *MockClient) StopContainer(ctx context.Context, id string, timeout *int) error {
	if m.StopContainerFunc != nil {
		return m.StopContainerFunc(ctx, id, timeout)
	}
	return nil
}

func (m *MockClient) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	if m.CreateContainerFunc != nil {
		return m.CreateContainerFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{}, nil
}

func (m *MockClient) WaitContainer(ctx context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	if m.WaitContainerFunc != nil {
		return m.WaitContainerFunc(ctx, id, condition)
	}
	resCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)
	resCh <- container.WaitResponse{StatusCode: 0}
	return resCh, errCh
}

func (m *MockClient) ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.ContainerLogsFunc != nil {
		return m.ContainerLogsFunc(ctx, id, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockClient) RemoveContainer(ctx context.Context, id string, options container.RemoveOptions) error {
	if m.RemoveContainerFunc != nil {
		return m.RemoveContainerFunc(ctx, id, options)
	}
	return nil
}

func (m *MockClient) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	if m.PullImageFunc != nil {
		return m.PullImageFunc(ctx, imageRef)
	}
	return nil, nil
}

func (m *MockClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.ServerVersionFunc != nil {
		return m.ServerVersionFunc(ctx)
	}
	return types.Version{}, nil
}

func (m *MockClient) Info(ctx context.Context) (system.Info, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc(ctx)
	}
	return system.Info{}, nil
}

func (m *MockClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestMockClient_ContainerAndImageMethods(t *testing.T) {
	ctx := context.Background()

	t.Run("RestartContainer", func(t *testing.T) {
		mock := &MockClient{}
		if err := mock.RestartContainer(ctx, "c1", nil); err != nil {
			t.Fatalf("expected nil error on default mock, got: %v", err)
		}

		called := false
		timeoutVal := 15
		expectedErr := errors.New("restart fail")
		mock.RestartContainerFunc = func(c context.Context, id string, timeout *int) error {
			called = true
			if id != "c1" {
				t.Errorf("expected id c1, got %s", id)
			}
			if timeout == nil || *timeout != 15 {
				t.Errorf("expected timeout 15, got %v", timeout)
			}
			return expectedErr
		}

		err := mock.RestartContainer(ctx, "c1", &timeoutVal)
		if !called {
			t.Error("expected RestartContainerFunc to be called")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("StartContainer", func(t *testing.T) {
		mock := &MockClient{}
		if err := mock.StartContainer(ctx, "c2"); err != nil {
			t.Fatalf("expected nil error on default mock, got: %v", err)
		}

		called := false
		expectedErr := errors.New("start fail")
		mock.StartContainerFunc = func(c context.Context, id string) error {
			called = true
			if id != "c2" {
				t.Errorf("expected id c2, got %s", id)
			}
			return expectedErr
		}

		err := mock.StartContainer(ctx, "c2")
		if !called {
			t.Error("expected StartContainerFunc to be called")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("StopContainer", func(t *testing.T) {
		mock := &MockClient{}
		if err := mock.StopContainer(ctx, "c3", nil); err != nil {
			t.Fatalf("expected nil error on default mock, got: %v", err)
		}

		called := false
		timeoutVal := 30
		expectedErr := errors.New("stop fail")
		mock.StopContainerFunc = func(c context.Context, id string, timeout *int) error {
			called = true
			if id != "c3" {
				t.Errorf("expected id c3, got %s", id)
			}
			if timeout == nil || *timeout != 30 {
				t.Errorf("expected timeout 30, got %v", timeout)
			}
			return expectedErr
		}

		err := mock.StopContainer(ctx, "c3", &timeoutVal)
		if !called {
			t.Error("expected StopContainerFunc to be called")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("PullImage", func(t *testing.T) {
		mock := &MockClient{}
		rc, err := mock.PullImage(ctx, "alpine:latest")
		if err != nil || rc != nil {
			t.Fatalf("expected nil rc and nil error on default mock, got %v, %v", rc, err)
		}

		called := false
		expectedErr := errors.New("pull fail")
		mockReader := io.NopCloser(strings.NewReader("fake stream"))
		mock.PullImageFunc = func(c context.Context, imageRef string) (io.ReadCloser, error) {
			called = true
			if imageRef != "nginx:alpine" {
				t.Errorf("expected imageRef nginx:alpine, got %s", imageRef)
			}
			return mockReader, expectedErr
		}

		r, err := mock.PullImage(ctx, "nginx:alpine")
		if !called {
			t.Error("expected PullImageFunc to be called")
		}
		if r != mockReader {
			t.Errorf("expected reader %v, got %v", mockReader, r)
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("CreateContainer", func(t *testing.T) {
		mock := &MockClient{}
		resp, err := mock.CreateContainer(ctx, nil, nil, nil, nil, "")
		if err != nil || resp.ID != "" {
			t.Fatalf("expected empty create response on default mock, got %v, %v", resp, err)
		}

		called := false
		mock.CreateContainerFunc = func(c context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
			called = true
			if containerName != "my-container" {
				t.Errorf("expected name my-container, got %s", containerName)
			}
			return container.CreateResponse{ID: "created-id-123"}, nil
		}

		res, err := mock.CreateContainer(ctx, nil, nil, nil, nil, "my-container")
		if !called || err != nil || res.ID != "created-id-123" {
			t.Errorf("unexpected create container response: %v, %v", res, err)
		}
	})

	t.Run("WaitContainer", func(t *testing.T) {
		mock := &MockClient{}
		resCh, errCh := mock.WaitContainer(ctx, "c1", container.WaitConditionNotRunning)
		select {
		case res := <-resCh:
			if res.StatusCode != 0 {
				t.Errorf("expected status 0 on default mock, got %d", res.StatusCode)
			}
		case err := <-errCh:
			t.Fatalf("unexpected error: %v", err)
		}

		mock.WaitContainerFunc = func(c context.Context, id string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
			rCh := make(chan container.WaitResponse, 1)
			eCh := make(chan error, 1)
			rCh <- container.WaitResponse{StatusCode: 42}
			return rCh, eCh
		}
		resCh2, _ := mock.WaitContainer(ctx, "c2", container.WaitConditionNotRunning)
		res2 := <-resCh2
		if res2.StatusCode != 42 {
			t.Errorf("expected status 42, got %d", res2.StatusCode)
		}
	})

	t.Run("ContainerLogs", func(t *testing.T) {
		mock := &MockClient{}
		rc, err := mock.ContainerLogs(ctx, "c1", container.LogsOptions{})
		if err != nil || rc == nil {
			t.Fatalf("expected reader on default mock, got %v, %v", rc, err)
		}
		_ = rc.Close()

		mock.ContainerLogsFunc = func(c context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("log content")), nil
		}
		rc2, err := mock.ContainerLogs(ctx, "c1", container.LogsOptions{ShowStdout: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rc2.Close()
		data, _ := io.ReadAll(rc2)
		if string(data) != "log content" {
			t.Errorf("expected 'log content', got %q", string(data))
		}
	})

	t.Run("RemoveContainer", func(t *testing.T) {
		mock := &MockClient{}
		if err := mock.RemoveContainer(ctx, "c1", container.RemoveOptions{}); err != nil {
			t.Fatalf("expected nil error on default mock, got: %v", err)
		}

		called := false
		mock.RemoveContainerFunc = func(c context.Context, id string, options container.RemoveOptions) error {
			called = true
			if id != "c1" || !options.Force {
				t.Errorf("unexpected args: %s, %+v", id, options)
			}
			return nil
		}
		err := mock.RemoveContainer(ctx, "c1", container.RemoveOptions{Force: true})
		if !called || err != nil {
			t.Errorf("expected successful RemoveContainerFunc call, got %v", err)
		}
	})
}
