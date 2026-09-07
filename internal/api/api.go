package api

import (
	"encoding/json"
	"net/http"

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
		r.Get("/stacks/{name}", s.handleGetStack)
		r.Get("/stacks/{name}/compose", s.handleGetStackCompose)
		r.Get("/stacks/{name}/containers", s.handleGetStackContainers)

		// Containers routes
		r.Get("/containers", s.handleListContainers)
		r.Get("/containers/{id}", s.handleInspectContainer)

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
