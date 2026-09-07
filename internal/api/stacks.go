package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chickenzord/dokidoki/internal/stack"
	"github.com/go-chi/chi/v5"
)

// handleListStacks handles GET /api/v1/stacks.
// Supports query parameter ?source=managed|external|all (default "all").
func (s *Server) handleListStacks(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeJSON(w, http.StatusOK, []stack.Summary{})
		return
	}

	sourceParam := r.URL.Query().Get("source")
	result, err := s.stackSvc.ListStacks(r.Context(), sourceParam)
	if err != nil {
		if errors.Is(err, stack.ErrInvalidSource) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list stacks: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGetStack handles GET /api/v1/stacks/{name}.
func (s *Server) handleGetStack(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	name := chi.URLParam(r, "name")
	detail, err := s.stackSvc.GetStack(r.Context(), name)
	if err != nil {
		if errors.Is(err, stack.ErrNotFound) {
			writeError(w, http.StatusNotFound, "stack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get stack: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

// handleGetStackCompose handles GET /api/v1/stacks/{name}/compose.
func (s *Server) handleGetStackCompose(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "compose file not found")
		return
	}

	name := chi.URLParam(r, "name")
	resp, err := s.stackSvc.GetStackCompose(r.Context(), name)
	if err != nil {
		if errors.Is(err, stack.ErrNotFound) || errors.Is(err, stack.ErrComposeNotFound) {
			writeError(w, http.StatusNotFound, "compose file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get compose: "+err.Error())
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

// handleGetStackContainers handles GET /api/v1/stacks/{name}/containers.
func (s *Server) handleGetStackContainers(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	name := chi.URLParam(r, "name")
	containers, err := s.stackSvc.GetStackContainers(r.Context(), name)
	if err != nil {
		if errors.Is(err, stack.ErrNotFound) {
			writeError(w, http.StatusNotFound, "stack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get stack containers: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

// handleCreateStack handles POST /api/v1/stacks.
func (s *Server) handleCreateStack(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusInternalServerError, "stack service unavailable")
		return
	}

	var req stack.CreateStackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "stack name is required")
		return
	}

	summary, err := s.stackSvc.CreateOrImportStack(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create stack: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, summary)
}

// handleUpdateStack handles PUT /api/v1/stacks/{name}.
func (s *Server) handleUpdateStack(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusInternalServerError, "stack service unavailable")
		return
	}

	name := chi.URLParam(r, "name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "stack name is required")
		return
	}

	var req stack.CreateStackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Name = name

	summary, err := s.stackSvc.CreateOrImportStack(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update stack: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// handleGetStackFiles handles GET /api/v1/stacks/{name}/files.
func (s *Server) handleGetStackFiles(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	name := chi.URLParam(r, "name")
	resp, err := s.stackSvc.GetStackFiles(r.Context(), name)
	if err != nil {
		if errors.Is(err, stack.ErrNotFound) {
			writeError(w, http.StatusNotFound, "stack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get stack files: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetStackFile handles GET /api/v1/stacks/{name}/files/{filename}.
func (s *Server) handleGetStackFile(w http.ResponseWriter, r *http.Request) {
	if s.stackSvc == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	name := chi.URLParam(r, "name")
	filename := chi.URLParam(r, "filename")

	file, err := s.stackSvc.GetStackFile(r.Context(), name, filename)
	if err != nil {
		if errors.Is(err, stack.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		if strings.Contains(err.Error(), "path traversal") || strings.Contains(err.Error(), "invalid") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get stack file: "+err.Error())
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/plain") || (strings.Contains(accept, "text/yaml") && file.IsCompose) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(file.Content))
		return
	}

	writeJSON(w, http.StatusOK, file)
}

