package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

// Client is the interface wrapping Docker engine operations needed by Dokidoki.
type Client interface {
	Ping(ctx context.Context) (types.Ping, error)
	ListContainers(ctx context.Context) ([]types.Container, error)
	InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error)
	RestartContainer(ctx context.Context, id string, timeout *int) error
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string, timeout *int) error
	PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error)
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (system.Info, error)
	Close() error
}

// ClientWrapper wraps the official Docker SDK client.
type ClientWrapper struct {
	cli *client.Client
}

// New creates a new Docker client conforming to the Client interface.
// It uses client.WithAPIVersionNegotiation() and client.FromEnv.
// If dockerHost is explicitly specified, it overrides with client.WithHost(dockerHost).
func New(dockerHost string) (Client, error) {
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

// NewClient creates a new Docker client wrapper (backwards compatibility).
func NewClient(dockerHost string) (*ClientWrapper, error) {
	c, err := New(dockerHost)
	if err != nil {
		return nil, err
	}
	return c.(*ClientWrapper), nil
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

// RestartContainer stops and starts a container again.
func (c *ClientWrapper) RestartContainer(ctx context.Context, id string, timeout *int) error {
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: timeout})
}

// StartContainer starts a container.
func (c *ClientWrapper) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

// StopContainer stops a container.
func (c *ClientWrapper) StopContainer(ctx context.Context, id string, timeout *int) error {
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: timeout})
}

// PullImage requests the docker host to pull an image from a remote registry.
func (c *ClientWrapper) PullImage(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	return c.cli.ImagePull(ctx, imageRef, image.PullOptions{})
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
