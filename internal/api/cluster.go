package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chickenzord/dokidoki/internal/cluster"

	"github.com/go-chi/chi/v5"
)

// handleListNodes handles GET /api/v1/nodes.
// Optional query parameter ?status=alive|suspect|offline|active (where "active" filters out offline nodes).
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if s.clusterSvc == nil {
		writeJSON(w, http.StatusOK, []cluster.Node{})
		return
	}
	nodes := s.clusterSvc.ListNodes()
	if nodes == nil {
		nodes = []cluster.Node{}
	}

	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		filtered := make([]cluster.Node, 0, len(nodes))
		for _, n := range nodes {
			if statusFilter == "active" {
				if n.Status != cluster.StatusOffline {
					filtered = append(filtered, n)
				}
			} else if string(n.Status) == statusFilter {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	writeJSON(w, http.StatusOK, nodes)
}

// handleGetNode handles GET /api/v1/nodes/{id}.
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s.clusterSvc == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	node, ok := s.clusterSvc.GetNode(id)
	if !ok {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	writeJSON(w, http.StatusOK, node)
}

// handleHandshake handles POST /api/v1/cluster/handshake.
func (s *Server) handleHandshake(w http.ResponseWriter, r *http.Request) {
	var req cluster.HandshakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ClusterToken == "" {
		req.ClusterToken = r.Header.Get("X-Cluster-Token")
	}

	if s.clusterSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster manager not initialized")
		return
	}

	resp, err := s.clusterSvc.HandleHandshake(req)
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
	var msg cluster.HeartbeatMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg.ClusterToken == "" {
		msg.ClusterToken = r.Header.Get("X-Cluster-Token")
	}

	if s.clusterSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster manager not initialized")
		return
	}

	if err := s.clusterSvc.HandleHeartbeat(msg); err != nil {
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

	if s.clusterSvc != nil {
		s.clusterSvc.HandleLeave(nodeID)
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

	if s.clusterSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster manager not initialized")
		return
	}

	err := s.clusterSvc.RemoveNode(id)
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
