package cluster

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	nodesFilename = "nodes.json"
)

// NodesFilePath returns the full path to the persisted discovered nodes file.
func NodesFilePath(stacksDir string) string {
	return filepath.Join(stacksDir, nodeIDSubdir, nodesFilename)
}

// LoadPersistedNodes reads discovered nodes from $stacksDir/.dokidoki/nodes.json.
// Returns an empty slice without error if the file does not exist.
func LoadPersistedNodes(stacksDir string) ([]Node, error) {
	if strings.TrimSpace(stacksDir) == "" {
		return nil, nil
	}

	path := NodesFilePath(stacksDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	var nodes []Node
	if err := json.Unmarshal([]byte(trimmed), &nodes); err != nil {
		return nil, err
	}

	return nodes, nil
}

// SavePersistedNodes saves discovered peers to $stacksDir/.dokidoki/nodes.json atomically.
func SavePersistedNodes(stacksDir string, nodes []Node) error {
	if strings.TrimSpace(stacksDir) == "" {
		return nil
	}

	targetDir := filepath.Join(stacksDir, nodeIDSubdir)
	targetFile := filepath.Join(targetDir, nodesFilename)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Sort nodes deterministically before saving
	sorted := make([]Node, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Name == sorted[j].Name {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Name < sorted[j].Name
	})

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := filepath.Join(targetDir, fmt.Sprintf(".%s.tmp.%d", nodesFilename, time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, append(data, '\n'), 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, targetFile)
}
