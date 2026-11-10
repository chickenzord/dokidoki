package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestHandleProxyNode(t *testing.T) {
	// Setup target peer mock HTTP server
	peerMux := http.NewServeMux()
	var receivedToken string
	peerMux.HandleFunc("/api/v1/stacks", func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Cluster-Token")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"name": "remote-stack"},
		})
	})
	peerServer := httptest.NewServer(peerMux)
	defer peerServer.Close()

	peerNode := cluster.Node{
		ID:        "peer-123",
		Name:      "remote-node",
		Addresses: []string{peerServer.URL},
		Status:    cluster.StatusAlive,
		IsSelf:    false,
	}

	selfNode := cluster.Node{
		ID:     "self-123",
		Name:   "self-node",
		Status: cluster.StatusAlive,
		IsSelf: true,
	}

	mockCluster := &mockClusterService{
		nodes: []cluster.Node{selfNode, peerNode},
	}

	cfg := &config.Config{
		ClusterToken: "cluster-secret",
	}

	server := NewServer(cfg, nil, nil, mockCluster, "")
	router := chi.NewRouter()
	router.HandleFunc("/api/v1/nodes/{id}/proxy/*", server.handleProxyNode)

	t.Run("Success proxy GET request with cluster token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/peer-123/proxy/api/v1/stacks", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		if receivedToken != "cluster-secret" {
			t.Fatalf("expected X-Cluster-Token 'cluster-secret', got %q", receivedToken)
		}
		if !stringsContains(rec.Body.String(), "remote-stack") {
			t.Fatalf("expected response body to contain 'remote-stack', got %s", rec.Body.String())
		}
	})

	t.Run("Error when proxying to self", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/self-123/proxy/api/v1/stacks", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Error when node not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/unknown-id/proxy/api/v1/stacks", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()))
}
