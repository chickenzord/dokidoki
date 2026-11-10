package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
)

const (
	// DefaultHelperImage is the default container image used for helper tasks.
	DefaultHelperImage = "docker:cli"

	// TasksDirName is the directory inside the stacks dir where tasks are persisted.
	TasksDirName = ".dokidoki/tasks"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// TaskInfo holds the metadata and state of an out-of-process task.
type TaskInfo struct {
	ID          string     `json:"id"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Config configures a task execution.
type Config struct {
	ID          string
	Description string
	Image       string
	Command     []string
	WorkingDir  string
	Env         []string
	Binds       []string
}

// Runner defines the interface for running and managing out-of-process helper tasks.
type Runner interface {
	StartTask(ctx context.Context, cfg Config) (*TaskInfo, error)
	GetTask(ctx context.Context, id string) (*TaskInfo, error)
	TailTaskLogs(ctx context.Context, id string, out io.Writer, follow bool) error
	ListTasks(ctx context.Context) ([]*TaskInfo, error)
	PruneTasks(ctx context.Context, olderThan time.Duration) (int, error)
}

// ContainerTaskRunner runs tasks using detached helper containers.
type ContainerTaskRunner struct {
	dockerCli    docker.Client
	stacksDir    string
	dockerSocket string
	defaultImage string
	mu           sync.Mutex
}

var _ Runner = (*ContainerTaskRunner)(nil)

// Option is a functional option for ContainerTaskRunner.
type Option func(*ContainerTaskRunner)

// WithDockerSocket overrides the Docker socket path mounted into helper containers.
func WithDockerSocket(socketPath string) Option {
	return func(r *ContainerTaskRunner) {
		r.dockerSocket = socketPath
	}
}

// WithDefaultImage overrides the default helper image.
func WithDefaultImage(image string) Option {
	return func(r *ContainerTaskRunner) {
		r.defaultImage = image
	}
}

// NewContainerTaskRunner creates a new ContainerTaskRunner.
func NewContainerTaskRunner(dockerCli docker.Client, stacksDir string, opts ...Option) *ContainerTaskRunner {
	socket := os.Getenv("DOCKER_SOCKET")
	if socket == "" {
		socket = "/var/run/docker.sock"
	}

	image := os.Getenv("DOKIDOKI_TASK_IMAGE")
	if image == "" {
		image = DefaultHelperImage
	}

	r := &ContainerTaskRunner{
		dockerCli:    dockerCli,
		stacksDir:    stacksDir,
		dockerSocket: socket,
		defaultImage: image,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// TasksDir returns the absolute path to the tasks directory.
func (r *ContainerTaskRunner) TasksDir() string {
	return filepath.Join(r.stacksDir, TasksDirName)
}

func (r *ContainerTaskRunner) taskStatusPath(id string) string {
	return filepath.Join(r.TasksDir(), fmt.Sprintf("%s.status", id))
}

func (r *ContainerTaskRunner) taskLogPath(id string) string {
	return filepath.Join(r.TasksDir(), fmt.Sprintf("%s.log", id))
}

// ensureTasksDir ensures that the tasks storage directory exists.
func (r *ContainerTaskRunner) ensureTasksDir() error {
	dir := r.TasksDir()
	return os.MkdirAll(dir, 0755)
}

// saveTaskStatus writes task status metadata to disk as JSON.
func (r *ContainerTaskRunner) saveTaskStatus(info *TaskInfo) error {
	if err := r.ensureTasksDir(); err != nil {
		return err
	}
	path := r.taskStatusPath(info.ID)
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// StartTask launches a detached helper container to execute the given task.
func (r *ContainerTaskRunner) StartTask(ctx context.Context, cfg Config) (*TaskInfo, error) {
	if r.dockerCli == nil {
		return nil, errors.New("docker client unavailable")
	}

	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}

	if err := r.ensureTasksDir(); err != nil {
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}

	image := cfg.Image
	if image == "" {
		image = r.defaultImage
	}

	now := time.Now().UTC()
	info := &TaskInfo{
		ID:          cfg.ID,
		Description: cfg.Description,
		Status:      StatusPending,
		CreatedAt:   now,
	}

	if err := r.saveTaskStatus(info); err != nil {
		return nil, fmt.Errorf("failed to initialize task status: %w", err)
	}

	// Prepare container binds
	// 1. Docker socket
	// 2. Stacks directory
	binds := make([]string, 0, len(cfg.Binds)+2)
	if r.dockerSocket != "" {
		binds = append(binds, fmt.Sprintf("%s:/var/run/docker.sock", r.dockerSocket))
	}
	cleanStacksDir := filepath.Clean(r.stacksDir)
	binds = append(binds, fmt.Sprintf("%s:%s", cleanStacksDir, cleanStacksDir))
	binds = append(binds, cfg.Binds...)

	// Container name
	containerName := fmt.Sprintf("dokidoki-task-%s", cfg.ID)

	// Wrap command to write logs and exit status inside the container.
	// Container entrypoint/cmd runs sh:
	// status file path and log file path inside container
	statusFile := r.taskStatusPath(cfg.ID)
	logFile := r.taskLogPath(cfg.ID)

	// Shell quote helper
	shellCmd := escapeShellCommand(cfg.Command)
	script := fmt.Sprintf(`(%s) > %s 2>&1
EXIT_CODE=$?
echo $EXIT_CODE > %s.exitcode
exit $EXIT_CODE`, shellCmd, escapeShellArg(logFile), escapeShellArg(statusFile))

	env := append([]string{}, cfg.Env...)

	workingDir := cfg.WorkingDir
	if workingDir == "" {
		workingDir = cleanStacksDir
	}

	createResp, err := r.dockerCli.CreateContainer(ctx, &container.Config{
		Image:        image,
		Entrypoint:   []string{"sh", "-c"},
		Cmd:          []string{script},
		WorkingDir:   workingDir,
		Env:          env,
		AttachStdout: false,
		AttachStderr: false,
	}, &container.HostConfig{
		Binds:      binds,
		AutoRemove: true,
	}, nil, nil, containerName)

	if err != nil && (strings.Contains(strings.ToLower(err.Error()), "no such image") || strings.Contains(strings.ToLower(err.Error()), "not found")) {
		// Attempt to pull the image and retry creation once
		if rc, pullErr := r.dockerCli.PullImage(ctx, image); pullErr == nil && rc != nil {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()

			createResp, err = r.dockerCli.CreateContainer(ctx, &container.Config{
				Image:        image,
				Entrypoint:   []string{"sh", "-c"},
				Cmd:          []string{script},
				WorkingDir:   workingDir,
				Env:          env,
				AttachStdout: false,
				AttachStderr: false,
			}, &container.HostConfig{
				Binds:      binds,
				AutoRemove: true,
			}, nil, nil, containerName)
		}
	}

	if err != nil {
		info.Status = StatusFailed
		info.Error = fmt.Sprintf("failed to create task container: %v", err)
		nowFinish := time.Now().UTC()
		info.FinishedAt = &nowFinish
		_ = r.saveTaskStatus(info)
		return nil, fmt.Errorf("failed to create helper container: %w", err)
	}

	if err := r.dockerCli.StartContainer(ctx, createResp.ID); err != nil {
		info.Status = StatusFailed
		info.Error = fmt.Sprintf("failed to start task container: %v", err)
		nowFinish := time.Now().UTC()
		info.FinishedAt = &nowFinish
		_ = r.saveTaskStatus(info)
		return nil, fmt.Errorf("failed to start helper container: %w", err)
	}

	started := time.Now().UTC()
	info.Status = StatusRunning
	info.StartedAt = &started
	_ = r.saveTaskStatus(info)

	return info, nil
}

// GetTask returns the current status and metadata of a task.
func (r *ContainerTaskRunner) GetTask(ctx context.Context, id string) (*TaskInfo, error) {
	statusPath := r.taskStatusPath(id)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	var info TaskInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse task status: %w", err)
	}

	// If task is recorded as running/pending, check if container has finished by checking the .exitcode file
	if info.Status == StatusRunning || info.Status == StatusPending {
		exitCodeFile := statusPath + ".exitcode"
		if exitData, err := os.ReadFile(exitCodeFile); err == nil {
			var exitCode int
			trimmed := strings.TrimSpace(string(exitData))
			if _, scanErr := fmt.Sscanf(trimmed, "%d", &exitCode); scanErr == nil {
				info.ExitCode = &exitCode
				now := time.Now().UTC()
				info.FinishedAt = &now
				if exitCode == 0 {
					info.Status = StatusCompleted
				} else {
					info.Status = StatusFailed
					info.Error = fmt.Sprintf("task exited with code %d", exitCode)
				}
				_ = r.saveTaskStatus(&info)
			}
		}
	}

	return &info, nil
}

// TailTaskLogs reads or streams logs of a task to the provided writer.
// If follow is true, it continues tailing until the task finishes or ctx is cancelled.
func (r *ContainerTaskRunner) TailTaskLogs(ctx context.Context, id string, out io.Writer, follow bool) error {
	logPath := r.taskLogPath(id)

	var file *os.File
	var err error

	// Wait up to 5 seconds for the log file to appear
	waitDeadline := time.Now().Add(5 * time.Second)
	for {
		file, err = os.Open(logPath)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) || time.Now().After(waitDeadline) || !follow {
			if os.IsNotExist(err) {
				return os.ErrNotExist
			}
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer file.Close()

	buf := make([]byte, 4096)
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				return readErr
			}

			if !follow {
				break
			}

			// Check if task is still running
			taskInfo, getErr := r.GetTask(ctx, id)
			if getErr == nil && (taskInfo.Status == StatusCompleted || taskInfo.Status == StatusFailed) {
				// Read any final bytes that might have been written
				for {
					finalN, finalErr := file.Read(buf)
					if finalN > 0 {
						_, _ = out.Write(buf[:finalN])
					}
					if finalErr != nil {
						break
					}
				}
				break
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
		}
	}

	return nil
}

// ListTasks returns all tasks ordered by creation time descending.
func (r *ContainerTaskRunner) ListTasks(ctx context.Context) ([]*TaskInfo, error) {
	dir := r.TasksDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TaskInfo{}, nil
		}
		return nil, err
	}

	var tasks []*TaskInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".status") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".status")
		info, err := r.GetTask(ctx, id)
		if err == nil && info != nil {
			tasks = append(tasks, info)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// PruneTasks removes task status and log files older than the specified duration.
func (r *ContainerTaskRunner) PruneTasks(ctx context.Context, olderThan time.Duration) (int, error) {
	tasks, err := r.ListTasks(ctx)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	pruned := 0

	for _, t := range tasks {
		if t.Status == StatusRunning || t.Status == StatusPending {
			continue
		}
		finishedOrCreated := t.CreatedAt
		if t.FinishedAt != nil {
			finishedOrCreated = *t.FinishedAt
		}
		if finishedOrCreated.Before(cutoff) {
			_ = os.Remove(r.taskStatusPath(t.ID))
			_ = os.Remove(r.taskStatusPath(t.ID) + ".exitcode")
			_ = os.Remove(r.taskLogPath(t.ID))
			pruned++
		}
	}

	return pruned, nil
}

// escapeShellArg escapes a single string argument for POSIX shell.
func escapeShellArg(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// escapeShellCommand formats an argv slice into a single shell command string.
func escapeShellCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = escapeShellArg(arg)
	}
	return strings.Join(escaped, " ")
}
