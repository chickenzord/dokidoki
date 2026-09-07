package cluster

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chickenzord/dokidoki/internal/model"
)

func TestPersistDiscoveredNodes(t *testing.T) {
	tempDir := t.TempDir()

	// Initially should return nil, nil when file does not exist
	initialNodes, err := LoadPersistedNodes(tempDir)
	if err != nil {
		t.Fatalf("expected no error loading non-existent nodes, got: %v", err)
	}
	if len(initialNodes) != 0 {
		t.Fatalf("expected 0 nodes, got: %d", len(initialNodes))
	}

	testNodes := []model.Node{
		{
			ID:        "node-2",
			Name:      "alpha",
			Addresses: []string{"http://192.168.1.20:8080"},
			Status:    model.NodeStatusAlive,
			Version:   "0.1.0",
			LastSeen:  time.Now().UTC().Truncate(time.Second),
		},
		{
			ID:        "node-1",
			Name:      "beta",
			Addresses: []string{"http://192.168.1.10:8080"},
			Status:    model.NodeStatusOffline,
			Version:   "0.1.0",
			LastSeen:  time.Now().UTC().Truncate(time.Second),
		},
	}

	// Save nodes
	if err := SavePersistedNodes(tempDir, testNodes); err != nil {
		t.Fatalf("failed to save nodes: %v", err)
	}

	// Verify file path
	expectedFile := filepath.Join(tempDir, ".dokidoki", "nodes.json")
	if path := NodesFilePath(tempDir); path != expectedFile {
		t.Errorf("expected path %s, got %s", expectedFile, path)
	}

	// Load nodes back
	loaded, err := LoadPersistedNodes(tempDir)
	if err != nil {
		t.Fatalf("failed to load nodes: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(loaded))
	}

	// Should be sorted deterministically by Name ("alpha" first, then "beta")
	if loaded[0].Name != "alpha" || loaded[0].ID != "node-2" {
		t.Errorf("expected first node to be alpha (node-2), got %+v", loaded[0])
	}
	if loaded[1].Name != "beta" || loaded[1].ID != "node-1" {
		t.Errorf("expected second node to be beta (node-1), got %+v", loaded[1])
	}
}

func TestPersistEmptyStacksDir(t *testing.T) {
	nodes, err := LoadPersistedNodes("")
	if err != nil || len(nodes) != 0 {
		t.Errorf("expected nil, nil for empty stacksDir, got %v, %v", nodes, err)
	}

	if err := SavePersistedNodes("", []model.Node{{ID: "n1"}}); err != nil {
		t.Errorf("expected nil error for empty stacksDir, got %v", err)
	}
}
