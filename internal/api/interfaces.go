package api

import (
	"context"
	"io"

	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/stack"
)

// StackService defines the domain operations for managing stacks and containers.
type StackService interface {
	ListStacks(ctx context.Context, source string) ([]stack.Summary, error)
	GetStack(ctx context.Context, name string) (*stack.Detail, error)
	GetStackCompose(ctx context.Context, name string) (*stack.ComposeResponse, error)
	GetStackContainers(ctx context.Context, name string) ([]docker.Container, error)
	ListContainers(ctx context.Context, stackFilter string, grouped bool) (any, error)
	InspectContainer(ctx context.Context, id string) (*stack.EnrichedContainerInspect, error)
	GetStackFiles(ctx context.Context, name string) (*stack.StackFilesResponse, error)
	GetStackFile(ctx context.Context, name, filename string) (*stack.StackFile, error)
	CreateOrImportStack(ctx context.Context, req stack.CreateStackRequest) (*stack.Summary, error)
	GetContainerCompose(ctx context.Context, id string) (*stack.ContainerComposeResponse, error)
	RestartContainer(ctx context.Context, id string, force bool) (*stack.OperationResult, error)
	StartContainer(ctx context.Context, id string) (*stack.OperationResult, error)
	StopContainer(ctx context.Context, id string, force bool) (*stack.OperationResult, error)
	PullContainerImage(ctx context.Context, id string, out io.Writer) (*stack.OperationResult, error)
	ComposeUp(ctx context.Context, stackName string, out io.Writer) (*stack.OperationResult, error)
	ComposeDown(ctx context.Context, stackName string, out io.Writer) (*stack.OperationResult, error)
	ComposeRestart(ctx context.Context, stackName string, service string, out io.Writer) (*stack.OperationResult, error)
	ComposePull(ctx context.Context, stackName string, out io.Writer) (*stack.OperationResult, error)
}

// ClusterService defines the cluster coordination operations required by the API.
type ClusterService interface {
	ListNodes() []cluster.Node
	GetNode(id string) (*cluster.Node, bool)
	HandleHandshake(req cluster.HandshakeRequest) (*cluster.HandshakeResponse, error)
	HandleHeartbeat(msg cluster.HeartbeatMessage) error
	HandleLeave(nodeID string)
	RemoveNode(nodeID string) error
}
