package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	AuthFileName    = "auth.json"
	AuthFileVersion = 1
	AuthFilePerm    = 0600
)

var (
	ErrInsecurePermissions = errors.New("auth file has insecure permissions")
)

// AuthConfig represents the schema of $STACKS_DIR/.dokidoki/auth.json.
type AuthConfig struct {
	Version           int    `json:"version"`
	ClusterToken      string `json:"cluster_token,omitempty"`
	AdminPasswordHash string `json:"admin_password_hash,omitempty"`
}

// AuthFilePath returns the canonical path to auth.json given stacksDir.
func AuthFilePath(stacksDir string) string {
	return filepath.Join(stacksDir, ".dokidoki", AuthFileName)
}

// LoadAuthConfig reads and validates auth.json from the given path.
// It checks file permissions: if wider than 0600, it attempts to chmod to 0600.
// If chmod fails or permissions remain wider than 0600, it refuses to load and returns an error.
func LoadAuthConfig(path string) (*AuthConfig, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	perm := fi.Mode().Perm()
	if perm&0077 != 0 {
		// Permissions are wider than 0600 (group or other has any r/w/x bits)
		if err := os.Chmod(path, AuthFilePerm); err != nil {
			return nil, fmt.Errorf("%w: failed to chmod %s to 0600: %v", ErrInsecurePermissions, path, err)
		}
		// Re-stat to verify
		fi, err = os.Stat(path)
		if err != nil {
			return nil, err
		}
		if fi.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("%w: %s has permissions %#o", ErrInsecurePermissions, path, fi.Mode().Perm())
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var auth AuthConfig
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("failed to parse auth file: %w", err)
	}

	if auth.Version == 0 {
		auth.Version = AuthFileVersion
	}

	return &auth, nil
}

// SaveAuthConfig atomically writes the AuthConfig to path with 0600 permissions.
func SaveAuthConfig(path string, auth *AuthConfig) error {
	if auth == nil {
		return errors.New("auth config cannot be nil")
	}
	if auth.Version == 0 {
		auth.Version = AuthFileVersion
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth config: %w", err)
	}
	data = append(data, '\n')

	// Write atomically using a temporary file in the same directory
	tmpFile, err := os.CreateTemp(dir, "auth.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	// Ensure 0600 permissions
	if err := tmpFile.Chmod(AuthFilePerm); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write auth config to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Rename temp file to destination atomically
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace auth file: %w", err)
	}

	return nil
}

// IsNotExist returns true if the error indicates that a file does not exist.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
