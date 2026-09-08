package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chickenzord/dokidoki/internal/stack"
)

func TestReadFilesSubcommand(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")
	_ = os.MkdirAll(targetDir, 0755)

	// 1. Setup test files
	_ = os.WriteFile(filepath.Join(targetDir, "compose.yaml"), []byte("services:\n  app:\n    image: redis\n"), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, ".env"), []byte("FOO=BAR\n"), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, ".env.production"), []byte("ENV=prod\n"), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, "app.conf"), []byte("key = value\n"), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, ".secret"), []byte("secret"), 0644)

	// Subdirectory that should be ignored
	subDir := filepath.Join(targetDir, "subdir")
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("inner"), 0644)

	// Build dokidoki binary for testing
	binPath := filepath.Join(tempDir, "dokidoki-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v, output: %s", err, string(out))
	}

	t.Run("Success", func(t *testing.T) {
		cmd := exec.Command(binPath, "read-files", targetDir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}

		var files []stack.StackFile
		if err := json.Unmarshal(out, &files); err != nil {
			t.Fatalf("failed to unmarshal output: %v, raw: %s", err, string(out))
		}

		if len(files) != 4 {
			t.Fatalf("expected 4 files, got %d: %+v", len(files), files)
		}

		// compose.yaml first
		if files[0].Name != "compose.yaml" || !files[0].IsCompose || files[0].IsEnv {
			t.Errorf("expected compose.yaml as first file, got %+v", files[0])
		}

		// .env files next
		if !files[1].IsEnv || files[1].IsCompose {
			t.Errorf("expected env file, got %+v", files[1])
		}
		if !files[2].IsEnv || files[2].IsCompose {
			t.Errorf("expected env file, got %+v", files[2])
		}

		// app.conf last
		if files[3].Name != "app.conf" || files[3].IsCompose || files[3].IsEnv {
			t.Errorf("expected app.conf, got %+v", files[3])
		}
	})

	t.Run("MissingDirectoryArgument", func(t *testing.T) {
		cmd := exec.Command(binPath, "read-files")
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit code when dir argument is missing")
		}
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		cmd := exec.Command(binPath, "read-files", filepath.Join(tempDir, "nonexistent"))
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit code when directory does not exist")
		}
	})
}
