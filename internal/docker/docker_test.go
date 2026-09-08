package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
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
}
