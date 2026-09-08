package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRunner(t *testing.T) {
	t.Run("default binary", func(t *testing.T) {
		r := NewRunner("", "")
		if r.bin != "docker compose" {
			t.Errorf("expected bin 'docker compose', got %q", r.bin)
		}
		if r.dockerHost != "" {
			t.Errorf("expected empty dockerHost, got %q", r.dockerHost)
		}
		if r.locks == nil {
			t.Error("expected locks map to be initialized")
		}
	})

	t.Run("custom binary and docker host", func(t *testing.T) {
		r := NewRunner("docker-compose", "tcp://192.168.1.100:2375")
		if r.bin != "docker-compose" {
			t.Errorf("expected bin 'docker-compose', got %q", r.bin)
		}
		if r.dockerHost != "tcp://192.168.1.100:2375" {
			t.Errorf("expected dockerHost 'tcp://192.168.1.100:2375', got %q", r.dockerHost)
		}
	})
}

func TestCommandArgumentParsing(t *testing.T) {
	ctx := context.Background()
	composePath := "/path/to/my-stack/docker-compose.yml"
	expectedDir := "/path/to/my-stack"

	tests := []struct {
		name         string
		bin          string
		dockerHost   string
		extraArgs    []string
		expectedCmd  string
		expectedArgs []string
	}{
		{
			name:         "docker compose up",
			bin:          "docker compose",
			extraArgs:    []string{"up", "-d", "--remove-orphans"},
			expectedCmd:  "docker",
			expectedArgs: []string{"docker", "compose", "-f", composePath, "up", "-d", "--remove-orphans"},
		},
		{
			name:         "docker-compose up",
			bin:          "docker-compose",
			extraArgs:    []string{"up", "-d", "--remove-orphans"},
			expectedCmd:  "docker-compose",
			expectedArgs: []string{"docker-compose", "-f", composePath, "up", "-d", "--remove-orphans"},
		},
		{
			name:         "docker compose down",
			bin:          "docker compose",
			extraArgs:    []string{"down"},
			expectedCmd:  "docker",
			expectedArgs: []string{"docker", "compose", "-f", composePath, "down"},
		},
		{
			name:         "docker-compose down",
			bin:          "docker-compose",
			extraArgs:    []string{"down"},
			expectedCmd:  "docker-compose",
			expectedArgs: []string{"docker-compose", "-f", composePath, "down"},
		},
		{
			name:         "docker compose restart with service",
			bin:          "docker compose",
			extraArgs:    []string{"restart", "app"},
			expectedCmd:  "docker",
			expectedArgs: []string{"docker", "compose", "-f", composePath, "restart", "app"},
		},
		{
			name:         "docker-compose restart without service",
			bin:          "docker-compose",
			extraArgs:    []string{"restart"},
			expectedCmd:  "docker-compose",
			expectedArgs: []string{"docker-compose", "-f", composePath, "restart"},
		},
		{
			name:         "docker compose pull",
			bin:          "docker compose",
			extraArgs:    []string{"pull"},
			expectedCmd:  "docker",
			expectedArgs: []string{"docker", "compose", "-f", composePath, "pull"},
		},
		{
			name:         "empty bin defaults to docker compose",
			bin:          "",
			extraArgs:    []string{"up", "-d", "--remove-orphans"},
			expectedCmd:  "docker",
			expectedArgs: []string{"docker", "compose", "-f", composePath, "up", "-d", "--remove-orphans"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRunner(tc.bin, tc.dockerHost)
			cmd := r.buildCommand(ctx, composePath, tc.extraArgs...)

			if cmd.Dir != expectedDir {
				t.Errorf("expected cmd.Dir %q, got %q", expectedDir, cmd.Dir)
			}

			if !reflect.DeepEqual(cmd.Args, tc.expectedArgs) {
				t.Errorf("args mismatch:\nexpected: %v\ngot:      %v", tc.expectedArgs, cmd.Args)
			}
		})
	}
}

func TestDockerHostEnvironment(t *testing.T) {
	ctx := context.Background()
	composePath := "/app/compose.yaml"

	t.Run("sets DOCKER_HOST when specified", func(t *testing.T) {
		host := "tcp://custom-host:2375"
		r := NewRunner("docker compose", host)
		cmd := r.buildCommand(ctx, composePath, "up")

		found := false
		for _, e := range cmd.Env {
			if e == "DOCKER_HOST="+host {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected cmd.Env to contain DOCKER_HOST=%s", host)
		}
	})

	t.Run("overrides existing DOCKER_HOST in environment", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		overrideHost := "tcp://remote-host:2376"
		r := NewRunner("docker compose", overrideHost)
		cmd := r.buildCommand(ctx, composePath, "up")

		var matched []string
		for _, e := range cmd.Env {
			if strings.HasPrefix(e, "DOCKER_HOST=") {
				matched = append(matched, e)
			}
		}

		if len(matched) != 1 {
			t.Fatalf("expected exactly 1 DOCKER_HOST entry in cmd.Env, got %d: %v", len(matched), matched)
		}
		if matched[0] != "DOCKER_HOST="+overrideHost {
			t.Errorf("expected %q, got %q", "DOCKER_HOST="+overrideHost, matched[0])
		}
	})

	t.Run("empty dockerHost preserves environment", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///original.sock")
		r := NewRunner("docker compose", "")
		cmd := r.buildCommand(ctx, composePath, "up")

		var found string
		for _, e := range cmd.Env {
			if strings.HasPrefix(e, "DOCKER_HOST=") {
				found = e
				break
			}
		}
		if found != "DOCKER_HOST=unix:///original.sock" {
			t.Errorf("expected original DOCKER_HOST preserved, got %q", found)
		}
	})
}

// createMockExecutable creates a script in a temporary directory that can be executed as a command runner.
func createMockExecutable(t *testing.T, scriptContent string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mock-bin")
	content := "#!/bin/sh\n" + scriptContent
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("failed to create mock executable: %v", err)
	}
	return path
}

func TestRealTimeStreaming(t *testing.T) {
	// Script that emits two distinct lines with a brief pause between them.
	script := `
echo "streaming line 1"
sleep 0.05
echo "streaming line 2" >&2
`
	binPath := createMockExecutable(t, script)
	r := NewRunner(binPath, "")

	composeDir := t.TempDir()
	composePath := filepath.Join(composeDir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to write compose.yaml: %v", err)
	}

	// Use a synchronized writer to verify chunks arrive in real-time.
	sw := &syncBuffer{}
	firstLineReceived := make(chan struct{})
	var once sync.Once

	sw.onWrite = func(p []byte) {
		if strings.Contains(string(p), "streaming line 1") {
			once.Do(func() {
				close(firstLineReceived)
			})
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- r.run(context.Background(), composePath, sw, "up")
	}()

	select {
	case <-firstLineReceived:
		// Line 1 was received while the command was still executing (or before done finished).
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first streamed chunk")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for command completion")
	}

	allOutput := sw.String()
	if !strings.Contains(allOutput, "streaming line 1") {
		t.Errorf("missing line 1 in output: %q", allOutput)
	}
	if !strings.Contains(allOutput, "streaming line 2") {
		t.Errorf("missing line 2 in output: %q", allOutput)
	}
}

func TestNilWriterExecution(t *testing.T) {
	script := `echo "success with nil writer"`
	binPath := createMockExecutable(t, script)
	r := NewRunner(binPath, "")

	composeDir := t.TempDir()
	composePath := filepath.Join(composeDir, "compose.yaml")

	err := r.run(context.Background(), composePath, nil, "up")
	if err != nil {
		t.Fatalf("expected no error with nil writer, got: %v", err)
	}
}

func TestErrorHandling(t *testing.T) {
	script := `
echo "standard error log: port 8080 already allocated" >&2
echo "standard out log: container web failed"
exit 1
`
	binPath := createMockExecutable(t, script)
	r := NewRunner(binPath, "")

	composeDir := t.TempDir()
	composePath := filepath.Join(composeDir, "compose.yaml")

	var out bytes.Buffer
	err := r.run(context.Background(), composePath, &out, "up")
	if err == nil {
		t.Fatal("expected error on non-zero exit code, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "compose command failed") {
		t.Errorf("expected error message to mention 'compose command failed', got: %q", errMsg)
	}
	if !strings.Contains(errMsg, "port 8080 already allocated") {
		t.Errorf("expected error message to contain stderr output, got: %q", errMsg)
	}
	if !strings.Contains(errMsg, "container web failed") {
		t.Errorf("expected error message to contain stdout output, got: %q", errMsg)
	}

	// Also verify that 'out' received the output despite failure.
	if !strings.Contains(out.String(), "port 8080 already allocated") {
		t.Errorf("expected output buffer to capture stderr, got: %q", out.String())
	}
}

func TestCancellation(t *testing.T) {
	t.Run("cancellation during execution", func(t *testing.T) {
		script := `
echo "started"
sleep 5
echo "finished"
`
		binPath := createMockExecutable(t, script)
		r := NewRunner(binPath, "")

		composeDir := t.TempDir()
		composePath := filepath.Join(composeDir, "compose.yaml")

		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		var once sync.Once
		sw := &syncBuffer{
			onWrite: func(p []byte) {
				if strings.Contains(string(p), "started") {
					once.Do(func() {
						close(started)
					})
				}
			},
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- r.run(ctx, composePath, sw, "up")
		}()

		select {
		case <-started:
			// Cancel while command is in the middle of sleeping.
			cancel()
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for command to start")
		}

		select {
		case err := <-errCh:
			if err == nil {
				t.Fatal("expected error on canceled context, got nil")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected errors.Is(err, context.Canceled), got: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for command to terminate after cancel")
		}
	})

	t.Run("pre-canceled context", func(t *testing.T) {
		script := `echo "should not run"`
		binPath := createMockExecutable(t, script)
		r := NewRunner(binPath, "")

		composeDir := t.TempDir()
		composePath := filepath.Join(composeDir, "compose.yaml")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := r.run(ctx, composePath, nil, "up")
		if err == nil {
			t.Fatal("expected error with pre-canceled context, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected errors.Is(err, context.Canceled), got: %v", err)
		}
	})
}

func TestMutexSerialization(t *testing.T) {
	script := `
echo "running $1"
sleep 0.1
echo "done $1"
`
	binPath := createMockExecutable(t, script)
	r := NewRunner(binPath, "")

	composeDir := t.TempDir()
	composePath1 := filepath.Join(composeDir, "stack1", "compose.yaml")
	composePath2 := filepath.Join(composeDir, "stack2", "compose.yaml")

	t.Run("concurrent runs for same composePath are serialized", func(t *testing.T) {
		var activeCount int32
		var maxConcurrent int32
		var wg sync.WaitGroup

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				sw := &syncBuffer{
					onWrite: func(p []byte) {
						if strings.Contains(string(p), "running") {
							current := atomic.AddInt32(&activeCount, 1)
							for {
								max := atomic.LoadInt32(&maxConcurrent)
								if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
									break
								}
							}
						}
						if strings.Contains(string(p), "done") {
							atomic.AddInt32(&activeCount, -1)
						}
					},
				}
				_ = r.run(context.Background(), composePath1, sw, fmt.Sprintf("%d", id))
			}(i)
		}

		wg.Wait()
		if maxConcurrent > 1 {
			t.Errorf("expected max concurrent execution for same composePath to be 1, got %d", maxConcurrent)
		}
	})

	t.Run("different composePaths have independent locks", func(t *testing.T) {
		lock1 := r.getLock(composePath1)
		lock2 := r.getLock(composePath2)
		if lock1 == lock2 {
			t.Error("expected different locks for different compose paths")
		}
	})
}

func TestRunnerInterfaceMethods(t *testing.T) {
	script := `
echo "args: $@"
`
	binPath := createMockExecutable(t, script)
	r := NewRunner(binPath, "")
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "compose.yaml")

	t.Run("Up", func(t *testing.T) {
		var out bytes.Buffer
		if err := r.Up(context.Background(), composePath, &out); err != nil {
			t.Fatalf("Up failed: %v", err)
		}
		expected := fmt.Sprintf("-f %s up -d --remove-orphans", composePath)
		if !strings.Contains(out.String(), expected) {
			t.Errorf("expected output to contain %q, got %q", expected, out.String())
		}
	})

	t.Run("Down", func(t *testing.T) {
		var out bytes.Buffer
		if err := r.Down(context.Background(), composePath, &out); err != nil {
			t.Fatalf("Down failed: %v", err)
		}
		expected := fmt.Sprintf("-f %s down", composePath)
		if !strings.Contains(out.String(), expected) {
			t.Errorf("expected output to contain %q, got %q", expected, out.String())
		}
	})

	t.Run("Restart with service", func(t *testing.T) {
		var out bytes.Buffer
		if err := r.Restart(context.Background(), composePath, "web", &out); err != nil {
			t.Fatalf("Restart failed: %v", err)
		}
		expected := fmt.Sprintf("-f %s restart web", composePath)
		if !strings.Contains(out.String(), expected) {
			t.Errorf("expected output to contain %q, got %q", expected, out.String())
		}
	})

	t.Run("Restart without service", func(t *testing.T) {
		var out bytes.Buffer
		if err := r.Restart(context.Background(), composePath, "", &out); err != nil {
			t.Fatalf("Restart failed: %v", err)
		}
		expected := fmt.Sprintf("-f %s restart", composePath)
		if !strings.Contains(out.String(), expected) {
			t.Errorf("expected output to contain %q, got %q", expected, out.String())
		}
	})

	t.Run("Pull", func(t *testing.T) {
		var out bytes.Buffer
		if err := r.Pull(context.Background(), composePath, &out); err != nil {
			t.Fatalf("Pull failed: %v", err)
		}
		expected := fmt.Sprintf("-f %s pull", composePath)
		if !strings.Contains(out.String(), expected) {
			t.Errorf("expected output to contain %q, got %q", expected, out.String())
		}
	})
}

func TestPTYExecutionAndTerminalSize(t *testing.T) {
	script := `
if [ -t 1 ]; then
    echo "stdout is a tty"
else
    echo "stdout is NOT a tty"
fi
`
	binPath := createMockExecutable(t, script)
	r := NewRunner(binPath, "")

	composeDir := t.TempDir()
	composePath := filepath.Join(composeDir, "compose.yaml")

	t.Run("with terminal size uses PTY", func(t *testing.T) {
		ctx := ContextWithTerminalSize(context.Background(), TerminalSize{Cols: 110, Rows: 30})
		size, ok := TerminalSizeFromContext(ctx)
		if !ok || size.Cols != 110 || size.Rows != 30 {
			t.Fatalf("expected TerminalSize {Cols: 110, Rows: 30}, got %+v", size)
		}

		var out bytes.Buffer
		err := r.run(ctx, composePath, &out, "up")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}

		output := out.String()
		if !strings.Contains(output, "stdout is a tty") {
			t.Errorf("expected stdout to be a tty, got output: %q", output)
		}
	})

	t.Run("without terminal size falls back to standard pipe", func(t *testing.T) {
		ctx := context.Background()

		var out bytes.Buffer
		err := r.run(ctx, composePath, &out, "up")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}

		output := out.String()
		if !strings.Contains(output, "stdout is NOT a tty") {
			t.Errorf("expected stdout to NOT be a tty (pipe fallback), got output: %q", output)
		}
	})
}

// syncBuffer is a thread-safe bytes.Buffer with an optional onWrite callback.
type syncBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	onWrite func([]byte)
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err = s.buf.Write(p)
	if s.onWrite != nil {
		s.onWrite(p)
	}
	return n, err
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
