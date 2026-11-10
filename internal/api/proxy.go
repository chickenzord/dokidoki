package api

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleProxyNode proxies HTTP requests targeting a cluster node to that node's endpoint.
// Route: /api/v1/nodes/{id}/proxy/*
func (s *Server) handleProxyNode(w http.ResponseWriter, r *http.Request) {
	if s.clusterSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster service unavailable")
		return
	}

	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}

	node, ok := s.clusterSvc.GetNode(nodeID)
	if !ok || node == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("node %q not found", nodeID))
		return
	}

	if node.IsSelf {
		writeError(w, http.StatusBadRequest, "cannot proxy to self node; use direct api paths")
		return
	}

	if len(node.Addresses) == 0 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("node %q has no reachable address", node.Name))
		return
	}

	targetRaw := node.Addresses[0]
	if !strings.HasPrefix(targetRaw, "http://") && !strings.HasPrefix(targetRaw, "https://") {
		targetRaw = "http://" + targetRaw
	}
	targetURL, err := url.Parse(targetRaw)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("invalid address for node %q: %v", node.Name, err))
		return
	}

	subPath := chi.URLParam(r, "*")
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}

	proxy := &httputil.ReverseProxy{
		Director: func(outReq *http.Request) {
			outReq.URL.Scheme = targetURL.Scheme
			outReq.URL.Host = targetURL.Host
			outReq.URL.Path = subPath
			outReq.URL.RawQuery = r.URL.RawQuery
			outReq.Host = targetURL.Host

			if s.cfg.ClusterToken != "" {
				outReq.Header.Set("X-Cluster-Token", s.cfg.ClusterToken)
			}
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
			writeError(rw, http.StatusBadGateway, fmt.Sprintf("proxy error to node %q: %v", node.Name, proxyErr))
		},
	}

	proxy.ServeHTTP(w, r)
}
