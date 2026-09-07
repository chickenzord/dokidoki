package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/stack"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
)

// EnrichedContainerInspect represents a Docker ContainerJSON enriched with Dokidoki stack metadata.
type EnrichedContainerInspect = stack.EnrichedContainerInspect

// handleListContainers handles GET /api/v1/containers.
// Supports query parameters:
// - ?grouped=true: returns stack.Grouped
// - ?stack={name}: returns []docker.Container filtered by stack
// - otherwise: returns flat []docker.Container
func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeJSON(w, http.StatusOK, []docker.Container{})
		return
	}

	grouped := r.URL.Query().Get("grouped") == "true"
	stackFilter := r.URL.Query().Get("stack")

	result, err := s.stackSvc.ListContainers(r.Context(), stackFilter, grouped)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list containers: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleInspectContainer handles GET /api/v1/containers/{id}.
func (s *Server) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	id := chi.URLParam(r, "id")
	enriched, err := s.stackSvc.InspectContainer(r.Context(), id)
	if err != nil {
		if errors.Is(err, stack.ErrContainerNotFound) || errors.Is(err, stack.ErrNotFound) || client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect container: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, enriched)
}

// handleGetContainerCompose handles GET /api/v1/containers/{id}/compose.
// Generates a Docker Compose representation of the specified container.
// Respects Accept: text/yaml to return raw YAML.
func (s *Server) handleGetContainerCompose(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	id := chi.URLParam(r, "id")
	resp, err := s.stackSvc.GetContainerCompose(r.Context(), id)
	if err != nil {
		if errors.Is(err, stack.ErrContainerNotFound) || errors.Is(err, stack.ErrNotFound) || client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to generate compose: "+err.Error())
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/yaml") {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp.Content))
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

