package config

import (
	"reflect"
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
	if cfg.NodeID != "" {
		t.Errorf("expected empty NodeID, got %s", cfg.NodeID)
	}
	if cfg.NodeName == "" {
		t.Errorf("expected non-empty NodeName, got empty")
	}
	if len(cfg.AdvertiseAddrs) != 0 {
		t.Errorf("expected empty AdvertiseAddrs, got %v", cfg.AdvertiseAddrs)
	}
	if len(cfg.Peers) != 0 {
		t.Errorf("expected empty Peers, got %v", cfg.Peers)
	}
	if cfg.ClusterToken != "" {
		t.Errorf("expected empty ClusterToken, got %s", cfg.ClusterToken)
	}
	if !cfg.EnableMDNS {
		t.Errorf("expected EnableMDNS true, got false")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel info, got %s", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("expected LogFormat text, got %s", cfg.LogFormat)
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
		"DOKIDOKI_NODE_ID=custom-node-id",
		"DOKIDOKI_NODE_NAME=my-custom-node",
		"DOKIDOKI_ADVERTISE_ADDR=http://10.0.0.1:8080, http://10.0.0.2:8080",
		"DOKIDOKI_PEERS=http://192.168.1.50:8080,http://192.168.1.51:8080",
		"DOKIDOKI_CLUSTER_TOKEN=secret-cluster-token",
		"DOKIDOKI_ENABLE_MDNS=false",
		"DOKIDOKI_LOG_LEVEL=debug",
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
	if cfg.NodeID != "custom-node-id" {
		t.Errorf("expected NodeID custom-node-id, got %s", cfg.NodeID)
	}
	if cfg.NodeName != "my-custom-node" {
		t.Errorf("expected NodeName my-custom-node, got %s", cfg.NodeName)
	}
	expectedAddrs := []string{"http://10.0.0.1:8080", "http://10.0.0.2:8080"}
	if !reflect.DeepEqual(cfg.AdvertiseAddrs, expectedAddrs) {
		t.Errorf("expected AdvertiseAddrs %v, got %v", expectedAddrs, cfg.AdvertiseAddrs)
	}
	expectedPeers := []string{"http://192.168.1.50:8080", "http://192.168.1.51:8080"}
	if !reflect.DeepEqual(cfg.Peers, expectedPeers) {
		t.Errorf("expected Peers %v, got %v", expectedPeers, cfg.Peers)
	}
	if cfg.ClusterToken != "secret-cluster-token" {
		t.Errorf("expected ClusterToken secret-cluster-token, got %s", cfg.ClusterToken)
	}
	if cfg.EnableMDNS != false {
		t.Errorf("expected EnableMDNS false, got true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug, got %s", cfg.LogLevel)
	}
}

func TestFlagsOverrideEnv(t *testing.T) {
	env := []string{
		"DOKIDOKI_BIND=127.0.0.1",
		"DOKIDOKI_PORT=9090",
		"DOKIDOKI_STACKS_DIR=/custom/stacks",
		"DOCKER_HOST=tcp://1.2.3.4:2375",
		"DOKIDOKI_NODE_ID=env-node-id",
		"DOKIDOKI_NODE_NAME=env-node-name",
		"DOKIDOKI_ADVERTISE_ADDR=http://env-addr:8080",
		"DOKIDOKI_PEERS=http://env-peer:8080",
		"DOKIDOKI_CLUSTER_TOKEN=env-token",
		"DOKIDOKI_ENABLE_MDNS=false",
		"DOKIDOKI_LOG_LEVEL=info",
	}

	args := []string{
		"-bind=192.168.1.100",
		"-port=9999",
		"-stacks-dir=/flag/stacks",
		"-docker-host=unix:///tmp/docker.sock",
		"-node-id=flag-node-id",
		"-node-name=flag-node-name",
		"-advertise-addr=http://flag-addr:8080,http://flag-addr-2:8080",
		"-peers=http://flag-peer-1:8080, http://flag-peer-2:8080",
		"-cluster-token=flag-token",
		"-enable-mdns=true",
		"-log-level=warn",
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
	if cfg.NodeID != "flag-node-id" {
		t.Errorf("expected NodeID flag-node-id, got %s", cfg.NodeID)
	}
	if cfg.NodeName != "flag-node-name" {
		t.Errorf("expected NodeName flag-node-name, got %s", cfg.NodeName)
	}
	expectedAddrs := []string{"http://flag-addr:8080", "http://flag-addr-2:8080"}
	if !reflect.DeepEqual(cfg.AdvertiseAddrs, expectedAddrs) {
		t.Errorf("expected AdvertiseAddrs %v, got %v", expectedAddrs, cfg.AdvertiseAddrs)
	}
	expectedPeers := []string{"http://flag-peer-1:8080", "http://flag-peer-2:8080"}
	if !reflect.DeepEqual(cfg.Peers, expectedPeers) {
		t.Errorf("expected Peers %v, got %v", expectedPeers, cfg.Peers)
	}
	if cfg.ClusterToken != "flag-token" {
		t.Errorf("expected ClusterToken flag-token, got %s", cfg.ClusterToken)
	}
	if cfg.EnableMDNS != true {
		t.Errorf("expected EnableMDNS true, got false")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected LogLevel warn, got %s", cfg.LogLevel)
	}
}

func TestLogLevelFlagsAndEnv(t *testing.T) {
	// 1. -verbose flag
	cfg, err := LoadFrom([]string{"-verbose"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug with -verbose, got %s", cfg.LogLevel)
	}

	// 2. -v shorthand flag
	cfg, err = LoadFrom([]string{"-v"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug with -v, got %s", cfg.LogLevel)
	}

	// 3. -debug flag
	cfg, err = LoadFrom([]string{"-debug"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug with -debug, got %s", cfg.LogLevel)
	}

	// 4. DOKIDOKI_VERBOSE env
	cfg, err = LoadFrom(nil, []string{"DOKIDOKI_VERBOSE=true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug with DOKIDOKI_VERBOSE=true, got %s", cfg.LogLevel)
	}

	// 5. DOKIDOKI_DEBUG env
	cfg, err = LoadFrom(nil, []string{"DOKIDOKI_DEBUG=1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug with DOKIDOKI_DEBUG=1, got %s", cfg.LogLevel)
	}

	// 6. -log-format flag
	cfg, err = LoadFrom([]string{"-log-format=json"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("expected LogFormat json with -log-format=json, got %s", cfg.LogFormat)
	}

	// 7. DOKIDOKI_LOG_FORMAT env
	cfg, err = LoadFrom(nil, []string{"DOKIDOKI_LOG_FORMAT=json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("expected LogFormat json with DOKIDOKI_LOG_FORMAT=json, got %s", cfg.LogFormat)
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

func TestInvalidEnableMDNS(t *testing.T) {
	env := []string{
		"DOKIDOKI_ENABLE_MDNS=not-a-bool",
	}

	_, err := LoadFrom([]string{}, env)
	if err == nil {
		t.Fatal("expected error for invalid DOKIDOKI_ENABLE_MDNS in env, got nil")
	}
}

