package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := LoadFrom([]string{}, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Bind != "0.0.0.0" {
		t.Errorf("expected Bind 0.0.0.0, got %s", cfg.Bind)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected Port 8080, got %d", cfg.Port)
	}
	if cfg.StacksDir != "/opt/stacks" {
		t.Errorf("expected StacksDir /opt/stacks, got %s", cfg.StacksDir)
	}
	if cfg.DockerHost != "" {
		t.Errorf("expected empty DockerHost, got %s", cfg.DockerHost)
	}
	if cfg.Addr() != "0.0.0.0:8080" {
		t.Errorf("expected Addr 0.0.0.0:8080, got %s", cfg.Addr())
	}
}

func TestEnvOverride(t *testing.T) {
	env := []string{
		"DOKIDOKI_BIND=127.0.0.1",
		"DOKIDOKI_PORT=9090",
		"DOKIDOKI_STACKS_DIR=/custom/stacks",
		"DOCKER_HOST=tcp://1.2.3.4:2375",
	}

	cfg, err := LoadFrom([]string{}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Bind != "127.0.0.1" {
		t.Errorf("expected Bind 127.0.0.1, got %s", cfg.Bind)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected Port 9090, got %d", cfg.Port)
	}
	if cfg.StacksDir != "/custom/stacks" {
		t.Errorf("expected StacksDir /custom/stacks, got %s", cfg.StacksDir)
	}
	if cfg.DockerHost != "tcp://1.2.3.4:2375" {
		t.Errorf("expected DockerHost tcp://1.2.3.4:2375, got %s", cfg.DockerHost)
	}
}

func TestFlagsOverrideEnv(t *testing.T) {
	env := []string{
		"DOKIDOKI_BIND=127.0.0.1",
		"DOKIDOKI_PORT=9090",
		"DOKIDOKI_STACKS_DIR=/custom/stacks",
		"DOCKER_HOST=tcp://1.2.3.4:2375",
	}

	args := []string{
		"-bind=192.168.1.100",
		"-port=9999",
		"-stacks-dir=/flag/stacks",
		"-docker-host=unix:///tmp/docker.sock",
	}

	cfg, err := LoadFrom(args, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Bind != "192.168.1.100" {
		t.Errorf("expected Bind 192.168.1.100, got %s", cfg.Bind)
	}
	if cfg.Port != 9999 {
		t.Errorf("expected Port 9999, got %d", cfg.Port)
	}
	if cfg.StacksDir != "/flag/stacks" {
		t.Errorf("expected StacksDir /flag/stacks, got %s", cfg.StacksDir)
	}
	if cfg.DockerHost != "unix:///tmp/docker.sock" {
		t.Errorf("expected DockerHost unix:///tmp/docker.sock, got %s", cfg.DockerHost)
	}
}

func TestInvalidPort(t *testing.T) {
	env := []string{
		"DOKIDOKI_PORT=invalid",
	}

	_, err := LoadFrom([]string{}, env)
	if err == nil {
		t.Fatal("expected error for invalid port in env, got nil")
	}
}
