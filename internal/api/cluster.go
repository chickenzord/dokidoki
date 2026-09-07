package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/model"
	"github.com/go-chi/chi/v5"
)

// handleListNodes handles GET /api/v1/nodes.
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if s.clusterManager == nil {
		writeJSON(w, http.StatusOK, []model.Node{})
		return
	}
	nodes := s.clusterManager.ListNodes()
	if nodes == nil {
		nodes = []model.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// handleGetNode handles GET /api/v1/nodes/{id}.
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s.clusterManager == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	node, ok := s.clusterManager.GetNode(id)
	if !ok {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	writeJSON(w, http.StatusOK, node)
}

// handleHandshake handles POST /api/v1/cluster/handshake.
func (s *Server) handleHandshake(w http.ResponseWriter, r *http.Request) {
	var req model.HandshakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ClusterToken == "" {
		req.ClusterToken = r.Header.Get("X-Cluster-Token")
	}

	if s.clusterManager == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster manager not initialized")
		return
	}

	resp, err := s.clusterManager.HandleHandshake(req)
	if err != nil {
		if errors.Is(err, cluster.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleHeartbeat handles POST /api/v1/cluster/heartbeat.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var msg model.HeartbeatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg.ClusterToken == "" {
		msg.ClusterToken = r.Header.Get("X-Cluster-Token")
	}

	if s.clusterManager == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster manager not initialized")
		return
	}

	if err := s.clusterManager.HandleHeartbeat(msg); err != nil {
		if errors.Is(err, cluster.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleLeave handles POST /api/v1/cluster/leave.
func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID string `json:"node_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = r.URL.Query().Get("node_id")
	}
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	if s.clusterManager != nil {
		s.clusterManager.HandleLeave(nodeID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleDeleteNode handles DELETE /api/v1/nodes/{id}.
func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}

	if s.clusterManager == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster manager not initialized")
		return
	}

	err := s.clusterManager.RemoveNode(id)
	if err != nil {
		if errors.Is(err, cluster.ErrCannotRemoveSelf) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, cluster.ErrNodeNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "node removed"})
}
