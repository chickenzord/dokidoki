package logger

import (
	"bytes"
	"strings"
	"testing"
)

type sampleConfig struct {
	Bind         string
	Port         int
	ClusterToken string
	EmptyToken   string `sensitive:"true"`
	Password     string
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.SetColor(true)

	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")
	l.Debug("debug message")

	out := buf.String()

	if !strings.Contains(out, ColorCyan+"[INFO]"+ColorReset) {
		t.Errorf("expected [INFO] in cyan, got: %q", out)
	}
	if !strings.Contains(out, ColorYellow+"[WARN]"+ColorReset) {
		t.Errorf("expected [WARN] in yellow, got: %q", out)
	}
	if !strings.Contains(out, ColorRed+"[ERROR]"+ColorReset) {
		t.Errorf("expected [ERROR] in red, got: %q", out)
	}
	if !strings.Contains(out, ColorGray+"[DEBUG]"+ColorReset) {
		t.Errorf("expected [DEBUG] in gray, got: %q", out)
	}

	// Verify timestamp is wrapped in gray
	if !strings.Contains(out, ColorGray+"20") {
		t.Errorf("expected timestamp in gray, got: %q", out)
	}
}

func TestPrintConfigMasking(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.SetColor(false) // test plain text alignment and masking

	cfg := sampleConfig{
		Bind:         "0.0.0.0",
		Port:         8080,
		ClusterToken: "my-secret-token",
		EmptyToken:   "",
		Password:     "supersecret",
	}

	l.PrintConfig(cfg)
	out := buf.String()

	if !strings.Contains(out, "Bind") || !strings.Contains(out, "0.0.0.0") {
		t.Errorf("expected Bind to be printed, got: %q", out)
	}
	if !strings.Contains(out, "Port") || !strings.Contains(out, "8080") {
		t.Errorf("expected Port to be printed, got: %q", out)
	}
	if !strings.Contains(out, "[configured: 15 chars]") {
		t.Errorf("expected ClusterToken to be masked with 15 chars, got: %q", out)
	}
	if !strings.Contains(out, "[disabled / open]") {
		t.Errorf("expected EmptyToken to be printed as [disabled / open], got: %q", out)
	}
	if strings.Contains(out, "my-secret-token") {
		t.Errorf("token leaked in output: %q", out)
	}
	if strings.Contains(out, "supersecret") {
		t.Errorf("password leaked in output: %q", out)
	}
}

func TestConfigItemsAlignment(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.SetColor(false)

	items := []ConfigItem{
		{Key: "A", Value: "val1"},
		{Key: "LongKeyName", Value: "val2"},
		{Key: "ClusterToken", Value: "abc", Sensitive: true},
		{Key: "EmptyKey", Value: "", Sensitive: true},
	}

	out := l.FormatConfig("My Settings", items)
	lines := strings.Split(strings.TrimSpace(out), "\n")

	if lines[0] != "My Settings" {
		t.Errorf("unexpected title line: %q", lines[0])
	}
	// Check alignment: all colons should align
	colonIdx := -1
	for _, line := range lines[1:] {
		idx := strings.Index(line, ":")
		if colonIdx == -1 {
			colonIdx = idx
		} else if idx != colonIdx {
			t.Errorf("colon not aligned at %d: %q", colonIdx, line)
		}
	}
}
