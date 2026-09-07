package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
)

func TestIsHexID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"12 char lowercase hex", "47040a455a73", true},
		{"12 char uppercase hex", "47040A455A73", true},
		{"12 char mixed hex", "47040a455A73", true},
		{"64 char lowercase hex", "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1", true},
		{"64 char uppercase hex", "47040A455A73E4492976839AA640EBFBA41C30573EEF3EEBAE08447D21C16CF1", true},
		{"empty string", "", false},
		{"11 chars (too short)", "47040a455a7", false},
		{"13 chars (invalid length)", "47040a455a73a", false},
		{"63 chars (too short)", "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf", false},
		{"65 chars (too long)", "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf12", false},
		{"12 chars non-hex letter", "47040a455a7g", false},
		{"12 chars special char", "47040a45-a73", false},
		{"64 chars with non-hex", "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cfz", false},
		{"hostname string", "localhost", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHexID(tt.input); got != tt.expected {
				t.Errorf("isHexID(%q) = %v, expected %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractContainerIDFromCgroup(t *testing.T) {
	validID := "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1"

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "cgroup v1 standard docker path",
			content: `12:memory:/docker/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1
11:devices:/docker/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1`,
			expected: validID,
		},
		{
			name: "cgroup v1 with containers path",
			content: `1:name=systemd:/docker/containers/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1
2:cpu:/docker/containers/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1`,
			expected: validID,
		},
		{
			name:     "cgroup v2 systemd slice",
			content:  `0::/system.slice/docker-47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1.scope`,
			expected: validID,
		},
		{
			name:     "cgroup v2 user slice",
			content:  `0::/user.slice/user-1000.slice/user@1000.service/docker-47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1.scope`,
			expected: validID,
		},
		{
			name:     "cgroup v2 cgroupfs without scope",
			content:  `0::/docker/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1`,
			expected: validID,
		},
		{
			name:     "cgroup v2 root docker scope",
			content:  `0::/docker-47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1.scope`,
			expected: validID,
		},
		{
			name: "cgroup non-container root",
			content: `12:memory:/
0::/`,
			expected: "",
		},
		{
			name: "cgroup non-container system service",
			content: `0::/system.slice/systemd-journald.service
1:name=systemd:/init.scope`,
			expected: "",
		},
		{
			name:     "cgroup short id in docker scope",
			content:  `0::/system.slice/docker-12345.scope`,
			expected: "",
		},
		{
			name:     "cgroup non-hex id in docker scope",
			content:  `0::/system.slice/docker-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz.scope`,
			expected: "",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractContainerIDFromCgroup(tt.content); got != tt.expected {
				t.Errorf("extractContainerIDFromCgroup() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestExtractContainerIDFromMountinfo(t *testing.T) {
	validID := "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1"

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "mountinfo /var/lib/docker/containers/<id>/resolv.conf",
			content:  `587 568 259:1 /var/lib/docker/containers/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/root rw`,
			expected: validID,
		},
		{
			name:     "mountinfo /docker/containers/<id>/hostname",
			content:  `588 568 259:1 /docker/containers/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1/hostname /etc/hostname rw,relatime - ext4 /dev/root rw`,
			expected: validID,
		},
		{
			name:     "mountinfo /mnt/data/docker/containers/<id>/hosts",
			content:  `589 568 259:1 /mnt/data/docker/containers/47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1/hosts /etc/hosts rw,relatime - ext4 /dev/root rw`,
			expected: validID,
		},
		{
			name: "mountinfo non-container",
			content: `12788 12787 0:17 / /dev rw,nosuid,relatime master:2 - tmpfs tmpfs rw,seclabel,mode=755
12789 12788 0:18 / /dev/pts rw,relatime master:3 - devpts devpts rw,seclabel,mode=600,ptmxmode=000`,
			expected: "",
		},
		{
			name:     "mountinfo invalid short id",
			content:  `587 568 259:1 /var/lib/docker/containers/short-id/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/root rw`,
			expected: "",
		},
		{
			name:     "mountinfo empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractContainerIDFromMountinfo(tt.content); got != tt.expected {
				t.Errorf("extractContainerIDFromMountinfo() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestIdentifySelf(t *testing.T) {
	validID := "47040a455a73e4492976839aa640ebfba41c30573eef3eebae08447d21c16cf1"
	shortID := "47040a455a73"

	origMountinfo := procMountinfoPath
	origCgroup := procCgroupPath
	defer func() {
		procMountinfoPath = origMountinfo
		procCgroupPath = origCgroup
	}()

	t.Run("DetectedFromMountinfo", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountinfoFile := filepath.Join(tmpDir, "mountinfo")
		cgroupFile := filepath.Join(tmpDir, "cgroup")

		content := "587 568 259:1 /var/lib/docker/containers/" + validID + "/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/root rw\n"
		if err := os.WriteFile(mountinfoFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cgroupFile, []byte("0::/\n"), 0644); err != nil {
			t.Fatal(err)
		}

		procMountinfoPath = mountinfoFile
		procCgroupPath = cgroupFile

		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				if id == validID {
					return types.ContainerJSON{
						ContainerJSONBase: &types.ContainerJSONBase{
							ID: validID,
						},
					}, nil
				}
				return types.ContainerJSON{}, errors.New("not found")
			},
		}

		id := IdentifySelf(context.Background(), mockCli)
		if id != validID {
			t.Fatalf("expected container ID %q, got %q", validID, id)
		}
	})

	t.Run("DetectedFromCgroup", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountinfoFile := filepath.Join(tmpDir, "mountinfo")
		cgroupFile := filepath.Join(tmpDir, "cgroup")

		if err := os.WriteFile(mountinfoFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		cgroupContent := "0::/system.slice/docker-" + validID + ".scope\n"
		if err := os.WriteFile(cgroupFile, []byte(cgroupContent), 0644); err != nil {
			t.Fatal(err)
		}

		procMountinfoPath = mountinfoFile
		procCgroupPath = cgroupFile

		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				if id == validID {
					return types.ContainerJSON{
						ContainerJSONBase: &types.ContainerJSONBase{
							ID: validID,
						},
					}, nil
				}
				return types.ContainerJSON{}, errors.New("not found")
			},
		}

		id := IdentifySelf(context.Background(), mockCli)
		if id != validID {
			t.Fatalf("expected container ID %q, got %q", validID, id)
		}
	})

	t.Run("FallbackToHostname", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountinfoFile := filepath.Join(tmpDir, "mountinfo")
		cgroupFile := filepath.Join(tmpDir, "cgroup")

		if err := os.WriteFile(mountinfoFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cgroupFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		procMountinfoPath = mountinfoFile
		procCgroupPath = cgroupFile

		// If current system hostname is hex, test inspection. Otherwise verify behavior.
		currentHostname, _ := os.Hostname()
		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				if id == currentHostname && isHexID(currentHostname) {
					return types.ContainerJSON{
						ContainerJSONBase: &types.ContainerJSONBase{
							ID: validID,
						},
					}, nil
				}
				return types.ContainerJSON{}, errors.New("not found")
			},
		}

		got := IdentifySelf(context.Background(), mockCli)
		if isHexID(currentHostname) {
			if got != validID {
				t.Fatalf("expected fallback to hostname returning %q, got %q", validID, got)
			}
		} else {
			if got != "" {
				t.Fatalf("expected empty string for non-hex hostname, got %q", got)
			}
		}
	})

	t.Run("FallbackToEnvVar", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountinfoFile := filepath.Join(tmpDir, "mountinfo")
		cgroupFile := filepath.Join(tmpDir, "cgroup")

		if err := os.WriteFile(mountinfoFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cgroupFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		procMountinfoPath = mountinfoFile
		procCgroupPath = cgroupFile

		os.Setenv("DOKIDOKI_CONTAINER_ID", shortID)
		defer os.Unsetenv("DOKIDOKI_CONTAINER_ID")

		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				if id == shortID {
					return types.ContainerJSON{
						ContainerJSONBase: &types.ContainerJSONBase{
							ID: validID,
						},
					}, nil
				}
				return types.ContainerJSON{}, errors.New("not found")
			},
		}

		id := IdentifySelf(context.Background(), mockCli)
		if id != validID {
			t.Fatalf("expected container ID from env var %q, got %q", validID, id)
		}
	})

	t.Run("BareMetalHostReturnsEmpty", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountinfoFile := filepath.Join(tmpDir, "mountinfo")
		cgroupFile := filepath.Join(tmpDir, "cgroup")

		if err := os.WriteFile(mountinfoFile, []byte("123 456 0:1 / / rw\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cgroupFile, []byte("0::/\n"), 0644); err != nil {
			t.Fatal(err)
		}

		procMountinfoPath = mountinfoFile
		procCgroupPath = cgroupFile

		os.Unsetenv("DOKIDOKI_CONTAINER_ID")

		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				return types.ContainerJSON{}, errors.New("no such container")
			},
		}

		id := IdentifySelf(context.Background(), mockCli)
		if id != "" {
			t.Fatalf("expected empty string on bare-metal host, got %q", id)
		}
	})

	t.Run("MissingProcFilesGracefulHandling", func(t *testing.T) {
		procMountinfoPath = "/non/existent/path/to/mountinfo"
		procCgroupPath = "/non/existent/path/to/cgroup"

		os.Unsetenv("DOKIDOKI_CONTAINER_ID")

		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				return types.ContainerJSON{}, errors.New("no such container")
			},
		}

		// Should not panic, should return ""
		id := IdentifySelf(context.Background(), mockCli)
		if id != "" {
			t.Fatalf("expected empty string when proc files missing, got %q", id)
		}
	})

	t.Run("NilClientGracefulHandling", func(t *testing.T) {
		procMountinfoPath = "/non/existent/path/to/mountinfo"
		procCgroupPath = "/non/existent/path/to/cgroup"

		os.Unsetenv("DOKIDOKI_CONTAINER_ID")

		// Nil client must not panic
		id := IdentifySelf(context.Background(), nil)
		if id != "" {
			t.Fatalf("expected empty string with nil client, got %q", id)
		}
	})

	t.Run("DockerInspectErrorFailsGracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountinfoFile := filepath.Join(tmpDir, "mountinfo")
		cgroupFile := filepath.Join(tmpDir, "cgroup")

		content := "587 568 259:1 /var/lib/docker/containers/" + validID + "/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/root rw\n"
		if err := os.WriteFile(mountinfoFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cgroupFile, []byte("0::/docker/"+validID+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		procMountinfoPath = mountinfoFile
		procCgroupPath = cgroupFile

		os.Unsetenv("DOKIDOKI_CONTAINER_ID")

		mockCli := &MockClient{
			InspectContainerFunc: func(ctx context.Context, id string) (types.ContainerJSON, error) {
				return types.ContainerJSON{}, errors.New("daemon connection refused")
			},
		}

		// When inspect fails, it should gracefully fall through and return ""
		id := IdentifySelf(context.Background(), mockCli)
		if id != "" {
			t.Fatalf("expected empty string when inspect fails, got %q", id)
		}
	})
}
