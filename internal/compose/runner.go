package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

// DefaultBin is the default compose command binary used if none is specified.
const DefaultBin = "docker-compose"

// TerminalSize defines the pseudo-terminal window dimensions.
type TerminalSize struct {
	Cols uint16
	Rows uint16
}

type terminalSizeContextKey struct{}

// ContextWithTerminalSize returns a context annotated with PTY window dimensions.
func ContextWithTerminalSize(ctx context.Context, size TerminalSize) context.Context {
	return context.WithValue(ctx, terminalSizeContextKey{}, size)
}

// TerminalSizeFromContext extracts PTY window dimensions from context, or default (80x24) if not found.
func TerminalSizeFromContext(ctx context.Context) (TerminalSize, bool) {
	if size, ok := ctx.Value(terminalSizeContextKey{}).(TerminalSize); ok && size.Cols > 0 && size.Rows > 0 {
		return size, true
	}
	return TerminalSize{Cols: 80, Rows: 24}, false
}

// Runner defines the interface for executing docker compose operations.
type Runner interface {
	Up(ctx context.Context, composePath string, out io.Writer, extraArgs ...string) error
	Down(ctx context.Context, composePath string, out io.Writer) error
	Restart(ctx context.Context, composePath string, service string, out io.Writer) error
	Pull(ctx context.Context, composePath string, out io.Writer) error
}

// CommandRunner executes compose commands via a fixed CLI binary.
type CommandRunner struct {
	bin        string // e.g. "docker compose" or "docker-compose"
	dockerHost string
	locks      map[string]*sync.Mutex
	locksMu    sync.Mutex
}

// Ensure CommandRunner implements Runner at compile time.
var _ Runner = (*CommandRunner)(nil)

// NewRunner creates a new CommandRunner instance.
// If bin is empty, it defaults to "docker-compose".
func NewRunner(bin string, dockerHost string) *CommandRunner {
	if bin == "" {
		bin = DefaultBin
	}
	return &CommandRunner{
		bin:        bin,
		dockerHost: dockerHost,
		locks:      make(map[string]*sync.Mutex),
	}
}

// getLock returns the mutex associated with the given path, creating one if necessary.
func (r *CommandRunner) getLock(composePath string) *sync.Mutex {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	if r.locks == nil {
		r.locks = make(map[string]*sync.Mutex)
	}

	key := filepath.Clean(composePath)
	mu, ok := r.locks[key]
	if !ok {
		mu = &sync.Mutex{}
		r.locks[key] = mu
	}
	return mu
}

// buildCommand prepares the exec.Cmd for the given composePath and extra arguments.
func (r *CommandRunner) buildCommand(ctx context.Context, composePath string, extraArgs ...string) *exec.Cmd {
	bin := r.bin
	if strings.TrimSpace(bin) == "" {
		bin = DefaultBin
	}

	return buildExecCommand(ctx, bin, r.dockerHost, composePath, extraArgs...)
}

func buildExecCommand(ctx context.Context, bin, dockerHost, composePath string, extraArgs ...string) *exec.Cmd {
	parts := strings.Fields(bin)
	if len(parts) == 0 {
		parts = strings.Fields(DefaultBin)
	}
	cmdName := parts[0]
	baseArgs := parts[1:]

	cmdArgs := make([]string, 0, len(baseArgs)+2+len(extraArgs))
	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs, "-f", composePath)
	cmdArgs = append(cmdArgs, extraArgs...)

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Dir = filepath.Dir(composePath)
	cmd.WaitDelay = 500 * time.Millisecond

	env := os.Environ()
	if dockerHost != "" {
		updated := false
		for i, e := range env {
			if strings.HasPrefix(e, "DOCKER_HOST=") {
				env[i] = "DOCKER_HOST=" + dockerHost
				updated = true
				break
			}
		}
		if !updated {
			env = append(env, "DOCKER_HOST="+dockerHost)
		}
	}
	cmd.Env = env

	return cmd
}

// run executes a compose command with the specified arguments, serializing
// runs targeting the same composePath using a per-path mutex.
func (r *CommandRunner) run(ctx context.Context, composePath string, out io.Writer, extraArgs ...string) error {
	mu := r.getLock(composePath)
	mu.Lock()
	defer mu.Unlock()

	cmd := r.buildCommand(ctx, composePath, extraArgs...)
	return executeCommand(ctx, cmd, out)
}

func executeCommand(ctx context.Context, cmd *exec.Cmd, out io.Writer) error {
	var buf bytes.Buffer
	var writer io.Writer = &buf
	if out != nil {
		writer = io.MultiWriter(out, &buf)
	}

	// Only use PTY if streaming output is requested AND an explicit terminal screen size was provided.
	// Otherwise, fallback to standard pipe-based subprocess spawning.
	if termSize, ok := TerminalSizeFromContext(ctx); ok && termSize.Cols > 0 && termSize.Rows > 0 && out != nil {
		ptmx, ptyErr := pty.StartWithSize(cmd, &pty.Winsize{
			Cols: termSize.Cols,
			Rows: termSize.Rows,
		})
		if ptyErr == nil {
			defer func() {
				_ = ptmx.Close()
			}()

			// Copy ptmx output in real-time. On Linux, EIO is returned when the child process exits.
			_, _ = io.Copy(writer, ptmx)

			if err := cmd.Wait(); err != nil {
				output := strings.TrimSpace(buf.String())
				if ctxErr := ctx.Err(); ctxErr != nil {
					if output != "" {
						return fmt.Errorf("compose command failed: %w: %v: %s", ctxErr, err, output)
					}
					return fmt.Errorf("compose command failed: %w: %v", ctxErr, err)
				}
				if output != "" {
					return fmt.Errorf("compose command failed: %w: %s", err, output)
				}
				return fmt.Errorf("compose command failed: %w", err)
			}
			return nil
		}
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(buf.String())
		if ctxErr := ctx.Err(); ctxErr != nil {
			if output != "" {
				return fmt.Errorf("compose command failed: %w: %v: %s", ctxErr, err, output)
			}
			return fmt.Errorf("compose command failed: %w: %v", ctxErr, err)
		}
		if output != "" {
			return fmt.Errorf("compose command failed: %w: %s", err, output)
		}
		return fmt.Errorf("compose command failed: %w", err)
	}

	return nil
}

// Up runs `up -d --remove-orphans` on the specified compose file, appending any extraArgs.
func (r *CommandRunner) Up(ctx context.Context, composePath string, out io.Writer, extraArgs ...string) error {
	args := append([]string{"up", "-d", "--remove-orphans"}, extraArgs...)
	return r.run(ctx, composePath, out, args...)
}

// Down runs `down` on the specified compose file.
func (r *CommandRunner) Down(ctx context.Context, composePath string, out io.Writer) error {
	return r.run(ctx, composePath, out, "down")
}

// Restart runs `restart <service>` if service is provided, or `restart` if service is empty.
func (r *CommandRunner) Restart(ctx context.Context, composePath string, service string, out io.Writer) error {
	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	}
	return r.run(ctx, composePath, out, args...)
}

// Pull runs `pull` on the specified compose file.
func (r *CommandRunner) Pull(ctx context.Context, composePath string, out io.Writer) error {
	return r.run(ctx, composePath, out, "pull")
}

// MultiVersionRunner dynamically resolves the compose binary per operation.
type MultiVersionRunner struct {
	mgr        *Manager
	dockerHost string
	locks      map[string]*sync.Mutex
	locksMu    sync.Mutex
}

// Ensure MultiVersionRunner implements Runner at compile time.
var _ Runner = (*MultiVersionRunner)(nil)

// NewMultiVersionRunner creates a new MultiVersionRunner instance.
func NewMultiVersionRunner(mgr *Manager, dockerHost string) *MultiVersionRunner {
	return &MultiVersionRunner{
		mgr:        mgr,
		dockerHost: dockerHost,
		locks:      make(map[string]*sync.Mutex),
	}
}

func (r *MultiVersionRunner) getLock(composePath string) *sync.Mutex {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	if r.locks == nil {
		r.locks = make(map[string]*sync.Mutex)
	}

	key := filepath.Clean(composePath)
	mu, ok := r.locks[key]
	if !ok {
		mu = &sync.Mutex{}
		r.locks[key] = mu
	}
	return mu
}

func (r *MultiVersionRunner) run(ctx context.Context, composePath string, out io.Writer, extraArgs ...string) error {
	mu := r.getLock(composePath)
	mu.Lock()
	defer mu.Unlock()

	bin := r.mgr.ResolveForStack(ctx, composePath, out)
	cmd := buildExecCommand(ctx, bin, r.dockerHost, composePath, extraArgs...)
	return executeCommand(ctx, cmd, out)
}

// Up runs `up -d --remove-orphans` dynamically resolving the compose binary.
func (r *MultiVersionRunner) Up(ctx context.Context, composePath string, out io.Writer, extraArgs ...string) error {
	args := append([]string{"up", "-d", "--remove-orphans"}, extraArgs...)
	return r.run(ctx, composePath, out, args...)
}

// Down runs `down` dynamically resolving the compose binary.
func (r *MultiVersionRunner) Down(ctx context.Context, composePath string, out io.Writer) error {
	return r.run(ctx, composePath, out, "down")
}

// Restart runs `restart` dynamically resolving the compose binary.
func (r *MultiVersionRunner) Restart(ctx context.Context, composePath string, service string, out io.Writer) error {
	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	}
	return r.run(ctx, composePath, out, args...)
}

// Pull runs `pull` dynamically resolving the compose binary.
func (r *MultiVersionRunner) Pull(ctx context.Context, composePath string, out io.Writer) error {
	return r.run(ctx, composePath, out, "pull")
}
