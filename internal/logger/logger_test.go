package logger

import (
	"bytes"
	"log/slog"
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

func TestConsoleHandlerLevels(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, slog.LevelDebug, true)
	l := slog.New(h)

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

func TestConsoleHandlerFiltering(t *testing.T) {
	var buf bytes.Buffer
	lvlVar := &slog.LevelVar{}
	lvlVar.Set(slog.LevelInfo)
	h := NewConsoleHandler(&buf, lvlVar, false)
	l := slog.New(h)

	// LevelInfo: Debug should be suppressed
	l.Debug("debug hidden")
	l.Info("info visible")
	if strings.Contains(buf.String(), "debug hidden") {
		t.Errorf("expected debug to be suppressed at LevelInfo, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "info visible") {
		t.Errorf("expected info to be visible at LevelInfo, got: %q", buf.String())
	}

	// Set to LevelWarn: Info should be suppressed
	buf.Reset()
	lvlVar.Set(slog.LevelWarn)
	l.Info("info hidden")
	l.Warn("warn visible")
	l.Error("error visible")
	if strings.Contains(buf.String(), "info hidden") {
		t.Errorf("expected info to be suppressed at LevelWarn, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "warn visible") || !strings.Contains(buf.String(), "error visible") {
		t.Errorf("expected warn and error to be visible at LevelWarn, got: %q", buf.String())
	}

	// Set to LevelDebug: Debug should be visible
	buf.Reset()
	lvlVar.Set(slog.LevelDebug)
	l.Debug("debug visible")
	if !strings.Contains(buf.String(), "debug visible") {
		t.Errorf("expected debug to be visible at LevelDebug, got: %q", buf.String())
	}
}

func TestConsoleHandlerAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, slog.LevelDebug, false)
	h = h.WithGroup("cluster").WithAttrs([]slog.Attr{slog.String("region", "us-east")}).(*ConsoleHandler)
	l := slog.New(h)

	l.Info("node connected", "node_id", "node-1")
	out := buf.String()

	if !strings.Contains(out, "cluster.region=us-east") {
		t.Errorf("expected cluster.region=us-east in %q", out)
	}
	if !strings.Contains(out, "cluster.node_id=node-1") {
		t.Errorf("expected cluster.node_id=node-1 in %q", out)
	}
}

func TestSetup(t *testing.T) {
	var buf bytes.Buffer

	// Text/Console setup
	l := Setup(&buf, slog.LevelInfo, "text", false)
	if l == nil {
		t.Fatal("expected non-nil logger from Setup")
	}
	slog.Info("test global slog info")
	if !strings.Contains(buf.String(), "test global slog info") {
		t.Errorf("expected log output from global slog: %q", buf.String())
	}

	// JSON setup
	buf.Reset()
	Setup(&buf, slog.LevelInfo, "json", false)
	slog.Info("test json slog", "key", "val")
	if !strings.Contains(buf.String(), `"msg":"test json slog"`) || !strings.Contains(buf.String(), `"key":"val"`) {
		t.Errorf("expected JSON output from slog: %q", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input       string
		expected    slog.Level
		expectError bool
	}{
		{"debug", slog.LevelDebug, false},
		{"DEBUG", slog.LevelDebug, false},
		{"verbose", slog.LevelDebug, false},
		{"VERBOSE", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"INFO", slog.LevelInfo, false},
		{"", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"invalid", slog.LevelInfo, true},
	}

	for _, tc := range tests {
		lvl, err := ParseLevel(tc.input)
		if (err != nil) != tc.expectError {
			t.Errorf("ParseLevel(%q) unexpected error status: %v", tc.input, err)
		}
		if !tc.expectError && lvl != tc.expected {
			t.Errorf("ParseLevel(%q) = %v, expected %v", tc.input, lvl, tc.expected)
		}
	}
}

func TestPrintConfigMasking(t *testing.T) {
	cfg := sampleConfig{
		Bind:         "0.0.0.0",
		Port:         8080,
		ClusterToken: "my-secret-token",
		EmptyToken:   "",
		Password:     "supersecret",
	}

	items := StructToConfigItems(cfg)
	out := FormatConfig("Config", items, false)

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
	items := []ConfigItem{
		{Key: "A", Value: "val1"},
		{Key: "LongKeyName", Value: "val2"},
		{Key: "ClusterToken", Value: "abc", Sensitive: true},
		{Key: "EmptyKey", Value: "", Sensitive: true},
	}

	out := FormatConfig("My Settings", items, false)
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
