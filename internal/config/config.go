package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

const (
	DefaultBind      = "0.0.0.0"
	DefaultPort      = 8080
	DefaultStacksDir = "/opt/stacks"
)

// Config holds runtime configuration settings.
type Config struct {
	Bind       string
	Port       int
	StacksDir  string
	DockerHost string
}

// Addr returns the host:port string for binding the HTTP server.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Bind, c.Port)
}

// DefaultConfig returns a new Config with standard default values.
func DefaultConfig() *Config {
	return &Config{
		Bind:      DefaultBind,
		Port:      DefaultPort,
		StacksDir: DefaultStacksDir,
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

	// CLI flags override environment variables and defaults
	fs := flag.NewFlagSet("dokidoki", flag.ContinueOnError)
	fs.StringVar(&cfg.Bind, "bind", cfg.Bind, "Address to bind HTTP server to")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	fs.StringVar(&cfg.StacksDir, "stacks-dir", cfg.StacksDir, "Directory containing stack definitions")
	fs.StringVar(&cfg.DockerHost, "docker-host", cfg.DockerHost, "Docker daemon host URL")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return cfg, nil
}
