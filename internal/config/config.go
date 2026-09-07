package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultBind      = "0.0.0.0"
	DefaultPort      = 8080
	DefaultStacksDir = "/opt/stacks"
	DefaultNodeName  = "dokidoki-node"
)

// Config holds runtime configuration settings.
type Config struct {
	Bind           string
	Port           int
	StacksDir      string
	DockerHost     string
	NodeID         string
	NodeName       string
	AdvertiseAddrs []string
	Peers          []string
	ClusterToken   string
	EnableMDNS     bool
}

// Addr returns the host:port string for binding the HTTP server.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Bind, c.Port)
}

// DefaultConfig returns a new Config with standard default values.
func DefaultConfig() *Config {
	nodeName, err := os.Hostname()
	if err != nil || strings.TrimSpace(nodeName) == "" {
		nodeName = DefaultNodeName
	}

	return &Config{
		Bind:       DefaultBind,
		Port:       DefaultPort,
		StacksDir:  DefaultStacksDir,
		NodeName:   nodeName,
		EnableMDNS: true,
	}
}

// Load loads configuration from environment variables and os.Args.
// Precedence: CLI Flags > Environment Variables > Defaults.
func Load() (*Config, error) {
	return LoadFrom(os.Args[1:], os.Environ())
}

// LoadFrom parses configuration from the provided args and environment variables.
func LoadFrom(args []string, environ []string) (*Config, error) {
	cfg := DefaultConfig()

	// Parse environment variables
	envMap := make(map[string]string, len(environ))
	for _, env := range environ {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				envMap[env[:i]] = env[i+1:]
				break
			}
		}
	}

	if val, ok := envMap["DOKIDOKI_BIND"]; ok && val != "" {
		cfg.Bind = val
	}
	if val, ok := envMap["DOKIDOKI_PORT"]; ok && val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid DOKIDOKI_PORT %q: %w", val, err)
		}
		cfg.Port = port
	}
	if val, ok := envMap["DOKIDOKI_STACKS_DIR"]; ok && val != "" {
		cfg.StacksDir = val
	}
	if val, ok := envMap["DOCKER_HOST"]; ok && val != "" {
		cfg.DockerHost = val
	}
	if val, ok := envMap["DOKIDOKI_NODE_ID"]; ok && val != "" {
		cfg.NodeID = strings.TrimSpace(val)
	}
	if val, ok := envMap["DOKIDOKI_NODE_NAME"]; ok && val != "" {
		cfg.NodeName = strings.TrimSpace(val)
	}
	if val, ok := envMap["DOKIDOKI_ADVERTISE_ADDR"]; ok {
		cfg.AdvertiseAddrs = parseCommaSeparated(val)
	}
	if val, ok := envMap["DOKIDOKI_PEERS"]; ok {
		cfg.Peers = parseCommaSeparated(val)
	}
	if val, ok := envMap["DOKIDOKI_CLUSTER_TOKEN"]; ok {
		cfg.ClusterToken = strings.TrimSpace(val)
	}
	if val, ok := envMap["DOKIDOKI_ENABLE_MDNS"]; ok && strings.TrimSpace(val) != "" {
		enableMDNS, err := strconv.ParseBool(strings.TrimSpace(val))
		if err != nil {
			return nil, fmt.Errorf("invalid DOKIDOKI_ENABLE_MDNS %q: %w", val, err)
		}
		cfg.EnableMDNS = enableMDNS
	}

	// CLI flags override environment variables and defaults
	fs := flag.NewFlagSet("dokidoki", flag.ContinueOnError)
	fs.StringVar(&cfg.Bind, "bind", cfg.Bind, "Address to bind HTTP server to")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	fs.StringVar(&cfg.StacksDir, "stacks-dir", cfg.StacksDir, "Directory containing stack definitions")
	fs.StringVar(&cfg.DockerHost, "docker-host", cfg.DockerHost, "Docker daemon host URL")
	fs.StringVar(&cfg.NodeID, "node-id", cfg.NodeID, "Node unique ID")
	fs.StringVar(&cfg.NodeName, "node-name", cfg.NodeName, "Node human-readable name")

	var advertiseAddrFlag string
	var peersFlag string
	fs.StringVar(&advertiseAddrFlag, "advertise-addr", strings.Join(cfg.AdvertiseAddrs, ","), "Comma-separated advertised addresses")
	fs.StringVar(&peersFlag, "peers", strings.Join(cfg.Peers, ","), "Comma-separated seed peers")
	fs.StringVar(&cfg.ClusterToken, "cluster-token", cfg.ClusterToken, "Cluster security authentication token")
	fs.BoolVar(&cfg.EnableMDNS, "enable-mdns", cfg.EnableMDNS, "Enable LAN mDNS discovery")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "advertise-addr":
			cfg.AdvertiseAddrs = parseCommaSeparated(advertiseAddrFlag)
		case "peers":
			cfg.Peers = parseCommaSeparated(peersFlag)
		}
	})

	return cfg, nil
}

func parseCommaSeparated(val string) []string {
	parts := strings.Split(val, ",")
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
