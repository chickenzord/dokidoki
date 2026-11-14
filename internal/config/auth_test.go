package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthFile_PermissionsAndLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	authPath := AuthFilePath(tempDir)

	authCfg := &AuthConfig{
		Version:           1,
		ClusterToken:      "tok-12345",
		AdminPasswordHash: "$2a$10$abcdef",
	}

	// 1. SaveAuthConfig should create with 0600
	err := SaveAuthConfig(authPath, authCfg)
	require.NoError(t, err)

	fi, err := os.Stat(authPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm())

	// 2. LoadAuthConfig should load correctly
	loaded, err := LoadAuthConfig(authPath)
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Version)
	assert.Equal(t, "tok-12345", loaded.ClusterToken)
	assert.Equal(t, "$2a$10$abcdef", loaded.AdminPasswordHash)

	// 3. Test loose permissions: e.g. 0644 -> should auto-chmod to 0600
	err = os.Chmod(authPath, 0644)
	require.NoError(t, err)

	loaded2, err := LoadAuthConfig(authPath)
	require.NoError(t, err)
	assert.Equal(t, "tok-12345", loaded2.ClusterToken)

	fi2, err := os.Stat(authPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fi2.Mode().Perm())
}

func TestConfigLoad_AuthIntegration(t *testing.T) {
	tempDir := t.TempDir()
	authPath := AuthFilePath(tempDir)

	// Write auth.json
	password := "my-secret-pw"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	authCfg := &AuthConfig{
		ClusterToken:      "auth-file-token",
		AdminPasswordHash: string(hash),
	}
	err = SaveAuthConfig(authPath, authCfg)
	require.NoError(t, err)

	// 1. Load config from stacks dir with auth.json
	cfg, err := LoadFrom([]string{}, []string{"DOKIDOKI_STACKS_DIR=" + tempDir})
	require.NoError(t, err)
	assert.Equal(t, "auth-file-token", cfg.ClusterToken)
	assert.Equal(t, string(hash), cfg.PasswordHash)

	// 2. Explicit env variables override auth.json
	envOverride := []string{
		"DOKIDOKI_STACKS_DIR=" + tempDir,
		"DOKIDOKI_CLUSTER_TOKEN=env-token",
		"DOKIDOKI_PASSWORD=env-new-password",
	}
	cfgEnv, err := LoadFrom([]string{}, envOverride)
	require.NoError(t, err)
	assert.Equal(t, "env-token", cfgEnv.ClusterToken)
	assert.NotEqual(t, string(hash), cfgEnv.PasswordHash)
	// Verify that PasswordHash matches bcrypt of env-new-password
	err = bcrypt.CompareHashAndPassword([]byte(cfgEnv.PasswordHash), []byte("env-new-password"))
	assert.NoError(t, err)
}
