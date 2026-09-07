package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

// Client is the interface wrapping Docker engine operations needed by Dokidoki.
type Client interface {
	Ping(ctx context.Context) (types.Ping, error)
	ListContainers(ctx context.Context) ([]types.Container, error)
	InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error)
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (system.Info, error)
	Close() error
}

// ClientWrapper wraps the official Docker SDK client.
type ClientWrapper struct {
	cli *client.Client
}

// NewClient creates a new Docker client wrapper.
// It uses client.WithAPIVersionNegotiation() and client.FromEnv.
// If dockerHost is explicitly specified, it overrides with client.WithHost(dockerHost).
func NewClient(dockerHost string) (*ClientWrapper, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if dockerHost != "" {
		opts = append(opts, client.WithHost(dockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	return &ClientWrapper{cli: cli}, nil
}

// Ping checks connectivity to the Docker daemon.
func (c *ClientWrapper) Ping(ctx context.Context) (types.Ping, error) {
	return c.cli.Ping(ctx)
}

// ListContainers returns all containers (including stopped/exited ones).
func (c *ClientWrapper) ListContainers(ctx context.Context) ([]types.Container, error) {
	return c.cli.ContainerList(ctx, container.ListOptions{All: true})
}

// InspectContainer returns low-level information about a container.
func (c *ClientWrapper) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	return c.cli.ContainerInspect(ctx, id)
}

// ServerVersion returns information about the Docker server version.
func (c *ClientWrapper) ServerVersion(ctx context.Context) (types.Version, error) {
	return c.cli.ServerVersion(ctx)
}

// Info returns system-wide information about the Docker daemon.
func (c *ClientWrapper) Info(ctx context.Context) (system.Info, error) {
	return c.cli.Info(ctx)
}

// Close closes the underlying transport.
func (c *ClientWrapper) Close() error {
	return c.cli.Close()
}
