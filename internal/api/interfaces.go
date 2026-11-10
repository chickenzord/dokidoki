package api

import (
	"context"
	"io"
	"time"

	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/stack"
	"github.com/chickenzord/dokidoki/internal/task"
	"github.com/docker/docker/api/types/container"
)

// TaskRunner defines the task runner methods needed by the API.
type TaskRunner interface {
	StartTask(ctx context.Context, cfg task.Config) (*task.TaskInfo, error)
	GetTask(ctx context.Context, id string) (*task.TaskInfo, error)
	TailTaskLogs(ctx context.Context, id string, out io.Writer, follow bool) error
	ListTasks(ctx context.Context) ([]*task.TaskInfo, error)
	PruneTasks(ctx context.Context, olderThan time.Duration) (int, error)
}

// StackService defines the domain operations for managing stacks and containers.
type StackService interface {
	ListStacks(ctx context.Context, source string) ([]stack.Summary, error)
	GetStack(ctx context.Context, name string) (*stack.Detail, error)
	GetStackCompose(ctx context.Context, name string) (*stack.ComposeResponse, error)
	GetStackContainers(ctx context.Context, name string) ([]docker.Container, error)
	ListContainers(ctx context.Context, stackFilter string, grouped bool) (any, error)
	InspectContainer(ctx context.Context, id string) (*stack.EnrichedContainerInspect, error)
	GetStackFiles(ctx context.Context, name string, forceRead bool) (*stack.StackFilesResponse, error)
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
	ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error)
}

// ClusterService defines the cluster coordination operations required by the API.
type ClusterService interface {
	Self() cluster.Node
	ListNodes() []cluster.Node
	GetNode(id string) (*cluster.Node, bool)
	AddPeer(ctx context.Context, address string) (*cluster.Node, error)
	HandleHandshake(req cluster.HandshakeRequest) (*cluster.HandshakeResponse, error)
	HandleHeartbeat(msg cluster.HeartbeatMessage) error
	HandleLeave(nodeID string)
	RemoveNode(nodeID string, broadcast ...bool) error
	LeaveCluster(ctx context.Context) error
}
