package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/chickenzord/dokidoki/internal/compose"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/stack"
	"github.com/chickenzord/dokidoki/web"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server holds dependencies for the REST API.
type Server struct {
	cfg             *config.Config
	dockerCli       docker.Client
	stackSvc        StackService
	clusterSvc      ClusterService
	stacksDir       string
	selfContainerID string
}

// NewServer creates a new API Server instance.
func NewServer(cfg *config.Config, dockerCli docker.Client, stackSvc StackService, clusterSvc ClusterService, selfContainerID string) *Server {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if stackSvc == nil {
		stackSvc = stack.NewService(stack.NewScanner(cfg.StacksDir), dockerCli, selfContainerID)
	}
	return &Server{
		cfg:             cfg,
		dockerCli:       dockerCli,
		stackSvc:        stackSvc,
		clusterSvc:      clusterSvc,
		stacksDir:       cfg.StacksDir,
		selfContainerID: selfContainerID,
	}
}

// NewRouter creates and configures the chi Router for Dokidoki.
func NewRouter(cfg *config.Config, dockerCli docker.Client, stackSvc StackService, clusterSvc ClusterService, selfContainerID string) http.Handler {
	s := NewServer(cfg, dockerCli, stackSvc, clusterSvc, selfContainerID)
	return s.Routes()
}

// Routes returns the configured http.Handler with all middleware and routes.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	// Setup middleware
	r.Use(HTTPLogger())
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Stacks routes
		r.Get("/stacks", s.handleListStacks)
		r.Post("/stacks", s.handleCreateStack)
		r.Get("/stacks/{name}", s.handleGetStack)
		r.Put("/stacks/{name}", s.handleUpdateStack)
		r.Get("/stacks/{name}/compose", s.handleGetStackCompose)
		r.Get("/stacks/{name}/containers", s.handleGetStackContainers)
		r.Get("/stacks/{name}/files", s.handleGetStackFiles)
		r.Get("/stacks/{name}/files/{filename}", s.handleGetStackFile)
		// Long-running streaming operations (compose up/down/restart/pull, container pull)
		r.Group(func(r chi.Router) {
			r.Use(SlidingDeadlineMiddleware(DefaultStreamingIdleTimeout))

			r.Post("/stacks/{name}/up", s.handleComposeUp)
			r.Post("/stacks/{name}/down", s.handleComposeDown)
			r.Post("/stacks/{name}/restart", s.handleComposeRestart)
			r.Post("/stacks/{name}/pull", s.handleComposePull)
			r.Post("/containers/{id}/pull", s.handlePullContainer)
		})

		// Containers routes
		r.Get("/containers", s.handleListContainers)
		r.Get("/containers/{id}", s.handleInspectContainer)
		r.Get("/containers/{id}/compose", s.handleGetContainerCompose)
		r.Post("/containers/{id}/restart", s.handleRestartContainer)
		r.Post("/containers/{id}/start", s.handleStartContainer)
		r.Post("/containers/{id}/stop", s.handleStopContainer)

		// Host routes
		r.Get("/host", s.handleHostInfo)
		r.Get("/host/ping", s.handlePing)

		// Cluster routes
		r.Get("/nodes", s.handleListNodes)
		r.Get("/nodes/{id}", s.handleGetNode)
		r.Delete("/nodes/{id}", s.handleDeleteNode)
		r.Post("/cluster/handshake", s.handleHandshake)
		r.Post("/cluster/heartbeat", s.handleHeartbeat)
		r.Post("/cluster/leave", s.handleLeave)
	})

	// Mount embedded Web UI handler for SPA routing with client-side fallback
	r.Handle("/*", web.Handler())

	return r
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func isStreamRequested(r *http.Request) bool {
	if r.URL.Query().Get("stream") == "true" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/plain") || strings.Contains(accept, "text/event-stream")
}

type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}

func parseTerminalSize(r *http.Request) (compose.TerminalSize, bool) {
	q := r.URL.Query()
	colsStr := q.Get("cols")
	rowsStr := q.Get("rows")
	if colsStr == "" {
		colsStr = r.Header.Get("X-Terminal-Cols")
	}
	if rowsStr == "" {
		rowsStr = r.Header.Get("X-Terminal-Rows")
	}

	cols, err1 := strconv.Atoi(colsStr)
	rows, err2 := strconv.Atoi(rowsStr)
	if err1 != nil || err2 != nil || cols <= 0 || rows <= 0 {
		return compose.TerminalSize{}, false
	}

	if cols < 20 {
		cols = 20
	} else if cols > 300 {
		cols = 300
	}
	if rows < 5 {
		rows = 5
	} else if rows > 150 {
		rows = 150
	}

	return compose.TerminalSize{Cols: uint16(cols), Rows: uint16(rows)}, true
}

func streamContext(r *http.Request) context.Context {
	ctx := r.Context()
	size, ok := parseTerminalSize(r)
	if !ok {
		return ctx
	}
	return compose.ContextWithTerminalSize(ctx, size)
}
