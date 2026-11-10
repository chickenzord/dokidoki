package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// handleListTasks handles GET /api/v1/tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if s.taskRunner == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	tasks, err := s.taskRunner.ListTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list tasks: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, tasks)
}

// handleGetTask handles GET /api/v1/tasks/{id}.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if s.taskRunner == nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	id := chi.URLParam(r, "id")
	task, err := s.taskRunner.GetTask(r.Context(), id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleTaskLogs handles GET /api/v1/tasks/{id}/logs.
// Supports ?follow=true or ?stream=true to follow/stream logs until task finishes.
func (s *Server) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	if s.taskRunner == nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	id := chi.URLParam(r, "id")
	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("stream") == "true"

	if follow {
		flusher, ok := w.(http.Flusher)
		if ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			fw := &flushWriter{w: w, f: flusher}
			_ = s.taskRunner.TailTaskLogs(r.Context(), id, fw, true)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err := s.taskRunner.TailTaskLogs(r.Context(), id, w, false)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// Log file might not exist yet
		return
	}
}
