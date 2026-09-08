package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestParseExitCode(t *testing.T) {
	tests := []struct {
		status   string
		expected int
		ok       bool
	}{
		{"Exited (0) 4 minutes ago", 0, true},
		{"Exited (1) 2 hours ago", 1, true},
		{"Exited (137) 10 seconds ago", 137, true},
		{"exited (0)", 0, true},
		{"Up 4 minutes", 0, false},
		{"Created", 0, false},
		{"", 0, false},
		{"Exited (abc)", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			code, ok := ParseExitCode(tt.status)
			if ok != tt.ok {
				t.Fatalf("ParseExitCode(%q) ok = %v, expected %v", tt.status, ok, tt.ok)
			}
			if ok && code != tt.expected {
				t.Fatalf("ParseExitCode(%q) code = %d, expected %d", tt.status, code, tt.expected)
			}
		})
	}
}

func TestConvertContainer_ExitCode(t *testing.T) {
	cExited0 := types.Container{
		ID:     "cid-0",
		Names:  []string{"/romm-db-init"},
		State:  "exited",
		Status: "Exited (0) 4 minutes ago",
	}
	res0 := ConvertContainer(cExited0, "")
	if res0.ExitCode == nil || *res0.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %v", res0.ExitCode)
	}

	cRunning := types.Container{
		ID:     "cid-run",
		Names:  []string{"/romm"},
		State:  "running",
		Status: "Up 4 minutes",
	}
	resRun := ConvertContainer(cRunning, "")
	if resRun.ExitCode != nil {
		t.Fatalf("expected nil ExitCode for running container, got %v", resRun.ExitCode)
	}
}
