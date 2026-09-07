package cluster

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	nodeIDSubdir   = ".dokidoki"
	nodeIDFilename = "node_id"
)

// ResolveNodeID resolves the unique ID for this Dokidoki node.
// Precedence:
// 1. Configured ID (if non-empty).
// 2. Persisted ID from $stacksDir/.dokidoki/node_id (if valid UUID).
// 3. Newly generated UUIDv4, persisted to $stacksDir/.dokidoki/node_id.
// If writing the node ID file fails, a warning is logged and the in-memory UUID is returned.
func ResolveNodeID(configuredID, stacksDir string) (string, error) {
	if trimmed := strings.TrimSpace(configuredID); trimmed != "" {
		return trimmed, nil
	}

	targetDir := filepath.Join(stacksDir, nodeIDSubdir)
	targetFile := filepath.Join(targetDir, nodeIDFilename)

	if data, err := os.ReadFile(targetFile); err == nil {
		persistedID := strings.TrimSpace(string(data))
		if _, parseErr := uuid.Parse(persistedID); parseErr == nil && persistedID != "" {
			return persistedID, nil
		}
	}

	generatedID := uuid.NewString()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		slog.Warn("Failed to create directory for node ID", "dir", targetDir, "error", err)
		return generatedID, nil
	}

	if err := os.WriteFile(targetFile, []byte(generatedID+"\n"), 0644); err != nil {
		slog.Warn("Failed to write node ID", "file", targetFile, "error", err)
		return generatedID, nil
	}

	return generatedID, nil
}
