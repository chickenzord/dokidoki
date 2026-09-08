package api

import (
	"errors"
	"fmt"
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

// handleRestartContainer handles POST /api/v1/containers/{id}/restart.
// Supports query parameter ?force=true to override self-container protection.
func (s *Server) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	id := chi.URLParam(r, "id")
	force := r.URL.Query().Get("force") == "true"

	res, err := s.stackSvc.RestartContainer(r.Context(), id, force)
	if err != nil {
		if errors.Is(err, stack.ErrSelfContainer) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, stack.ErrContainerNotFound) || errors.Is(err, stack.ErrNotFound) || client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to restart container: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// handleStartContainer handles POST /api/v1/containers/{id}/start.
func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	id := chi.URLParam(r, "id")
	res, err := s.stackSvc.StartContainer(r.Context(), id)
	if err != nil {
		if errors.Is(err, stack.ErrContainerNotFound) || errors.Is(err, stack.ErrNotFound) || client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to start container: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// handleStopContainer handles POST /api/v1/containers/{id}/stop.
// Supports query parameter ?force=true to override self-container protection.
func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	id := chi.URLParam(r, "id")
	force := r.URL.Query().Get("force") == "true"

	res, err := s.stackSvc.StopContainer(r.Context(), id, force)
	if err != nil {
		if errors.Is(err, stack.ErrSelfContainer) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, stack.ErrContainerNotFound) || errors.Is(err, stack.ErrNotFound) || client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to stop container: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// handlePullContainer handles POST /api/v1/containers/{id}/pull.
// Supports ?stream=true or Accept: text/plain to stream progress live.
func (s *Server) handlePullContainer(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	id := chi.URLParam(r, "id")

	if isStreamRequested(r) {
		flusher, ok := w.(http.Flusher)
		if ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			fw := &flushWriter{w: w, f: flusher}
			_, err := s.stackSvc.PullContainerImage(r.Context(), id, fw)
			if err != nil {
				_, _ = fmt.Fprintf(fw, "\nError: %v\n", err)
			}
			return
		}
	}

	res, err := s.stackSvc.PullContainerImage(r.Context(), id, nil)
	if err != nil {
		if errors.Is(err, stack.ErrContainerNotFound) || errors.Is(err, stack.ErrNotFound) || client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to pull container image: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

