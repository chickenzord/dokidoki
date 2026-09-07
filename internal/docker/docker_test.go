package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
)

// Ensure ClientWrapper implements Client interface at compile time.
var _ Client = (*ClientWrapper)(nil)

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
