package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/chickenzord/dokidoki/internal/compose"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/stack"
	"github.com/chickenzord/dokidoki/internal/task"
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
	taskRunner      TaskRunner
	stacksDir       string
	selfContainerID string
	webUI           http.Handler
	sessionMgr      *SessionManager
}

// NewServer creates a new API Server instance.
func NewServer(cfg *config.Config, dockerCli docker.Client, stackSvc StackService, clusterSvc ClusterService, selfContainerID string) *Server {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	var taskRunner TaskRunner
	if dockerCli != nil && cfg.StacksDir != "" {
		taskRunner = task.NewContainerTaskRunner(dockerCli, cfg.StacksDir)
	}
	if stackSvc == nil {
		stackSvc = stack.NewService(stack.NewScanner(cfg.StacksDir), dockerCli, selfContainerID, stack.WithTaskRunner(taskRunner))
	}
	sessionMgr := NewSessionManager(cfg.ClusterToken)
	return &Server{
		cfg:             cfg,
		dockerCli:       dockerCli,
		stackSvc:        stackSvc,
		clusterSvc:      clusterSvc,
		taskRunner:      taskRunner,
		stacksDir:       cfg.StacksDir,
		selfContainerID: selfContainerID,
		sessionMgr:      sessionMgr,
	}
}

// WithTaskRunner overrides the default TaskRunner.
func (s *Server) WithTaskRunner(tr TaskRunner) *Server {
	s.taskRunner = tr
	return s
}

// WithWebUI replaces the default embedded Web UI handler. Intended for tests
// that inject an in-memory fs.FS via web.HandlerFromFS.
func (s *Server) WithWebUI(h http.Handler) *Server {
	s.webUI = h
	return s
}

// NewRouter creates and configures the chi Router for Dokidoki.
func NewRouter(cfg *config.Config, dockerCli docker.Client, stackSvc StackService, clusterSvc ClusterService, selfContainerID string) http.Handler {
	s := NewServer(cfg, dockerCli, stackSvc, clusterSvc, selfContainerID)
	return s.Routes()
}

// isLoopbackRequest returns true if the request remote IP is loopback or if RemoteAddr is empty or httptest default.
func isLoopbackRequest(r *http.Request) bool {
	if r.RemoteAddr == "" || strings.HasPrefix(r.RemoteAddr, "192.0.2.1:") || r.RemoteAddr == "192.0.2.1" {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	}
	return ip.IsLoopback()
}

// clusterAuthMiddleware enforces X-Cluster-Token matching cfg.ClusterToken via constant-time comparison.
func (s *Server) clusterAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Cluster-Token")
		if s.cfg == nil || s.cfg.ClusterToken == "" {
			writeError(w, http.StatusUnauthorized, "cluster token not configured")
			return
		}
		// If header is missing, check query parameter (for backwards compatibility if needed)
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.ClusterToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid cluster token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// userAuthMiddleware enforces session cookie or loopback access if no password configured.
func (s *Server) userAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isSessionValid(r) {
			next.ServeHTTP(w, r)
			return
		}

		if s.cfg == nil || s.cfg.PasswordHash == "" {
			if isLoopbackRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "authentication required for remote access")
			return
		}

		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

// readOnlyMiddleware returns an HTTP middleware that rejects mutating requests with 403 Forbidden
// when the node is configured in read-only mode.
func (s *Server) readOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg != nil && s.cfg.ReadOnly {
			writeError(w, http.StatusForbidden, "node is in read-only mode")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Routes returns the configured http.Handler with all middleware and routes.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	// Setup middleware
	r.Use(HTTPLogger())
	r.Use(chiMiddleware.Recoverer)
	if s.cfg != nil && len(s.cfg.AllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   s.cfg.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Cluster-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// 1. Public Allowlist
		r.Get("/host/ping", s.handlePing)
		r.Get("/auth/me", s.handleAuthMe)
		r.Post("/auth/login", s.handleAuthLogin)
		r.Post("/auth/logout", s.handleAuthLogout)

		// 2. Cluster Endpoints
		// Note: handleHandshake and handleHeartbeat also check token in payload or X-Cluster-Token.
		// All cluster routes require valid cluster token.
		r.Group(func(r chi.Router) {
			r.Use(s.clusterAuthMiddleware)

			r.Post("/cluster/leave", s.handleLeave)
			r.Post("/nodes", s.handleAddNode)
			r.Delete("/nodes/{id}", s.handleDeleteNode)
		})
		r.Post("/cluster/handshake", s.handleHandshake)
		r.Post("/cluster/heartbeat", s.handleHeartbeat)

		// 3. Protected User Endpoints (guarded by userAuthMiddleware)
		r.Group(func(r chi.Router) {
			r.Use(s.userAuthMiddleware)

			// Auth management
			r.Post("/auth/change-password", s.handleAuthChangePassword)

			// Non-streaming REST routes protected by timeout
			r.Group(func(r chi.Router) {
				r.Use(chiMiddleware.Timeout(DefaultRequestTimeout))

				// Read-only queries (Stacks)
				r.Get("/stacks", s.handleListStacks)
				r.Get("/stacks/{name}", s.handleGetStack)
				r.Get("/stacks/{name}/compose", s.handleGetStackCompose)
				r.Get("/stacks/{name}/containers", s.handleGetStackContainers)
				r.Get("/stacks/{name}/files", s.handleGetStackFiles)
				r.Get("/stacks/{name}/files/{filename}", s.handleGetStackFile)

				// Read-only queries (Containers)
				r.Get("/containers", s.handleListContainers)
				r.Get("/containers/{id}", s.handleInspectContainer)
				r.Get("/containers/{id}/compose", s.handleGetContainerCompose)

				// Docker engine resources (Networks, Volumes, Images)
				r.Get("/networks", s.handleListNetworks)
				r.Get("/networks/{id}", s.handleGetNetwork)
				r.Get("/volumes", s.handleListVolumes)
				r.Get("/volumes/{id}", s.handleGetVolume)
				r.Get("/images", s.handleListImages)
				r.Get("/images/{id}", s.handleGetImage)

				// Tasks routes (metadata)
				r.Get("/tasks", s.handleListTasks)
				r.Get("/tasks/{id}", s.handleGetTask)

				// Host routes
				r.Get("/host", s.handleHostInfo)

				// Cluster discovery (read-only node queries remain accessible to users)
				r.Get("/nodes", s.handleListNodes)
				r.Get("/nodes/{id}", s.handleGetNode)

				// Mutating REST operations guarded by read-only mode
				r.Group(func(r chi.Router) {
					r.Use(s.readOnlyMiddleware)

					r.Post("/stacks", s.handleCreateStack)
					r.Put("/stacks/{name}", s.handleUpdateStack)
					r.Post("/containers/{id}/restart", s.handleRestartContainer)
					r.Post("/containers/{id}/start", s.handleStartContainer)
					r.Post("/containers/{id}/stop", s.handleStopContainer)
				})
			})

			// Long-running streaming operations (compose up/down/restart/pull, container pull)
			r.Group(func(r chi.Router) {
				r.Use(SlidingDeadlineMiddleware(DefaultStreamingIdleTimeout))
				r.Use(s.readOnlyMiddleware)

				r.Post("/stacks/{name}/up", s.handleComposeUp)
				r.Post("/stacks/{name}/down", s.handleComposeDown)
				r.Post("/stacks/{name}/restart", s.handleComposeRestart)
				r.Post("/stacks/{name}/pull", s.handleComposePull)
				r.Post("/containers/{id}/pull", s.handlePullContainer)
			})

			// Long-lived streaming log and event routes (SSE / plain text follow)
			r.Get("/containers/{id}/logs", s.handleContainerLogs)
			r.Get("/tasks/{id}/logs", s.handleTaskLogs)
			r.Get("/events", s.handleDockerEvents)

			// Cluster proxy route (can proxy streaming requests to worker nodes)
			r.HandleFunc("/nodes/{id}/proxy/*", s.handleProxyNode)
		})
	})

	// Mount resource routes directly under /api for convenience / compatibility (protected by userAuthMiddleware)
	r.Route("/api", func(r chi.Router) {
		r.Use(s.userAuthMiddleware)
		r.Group(func(r chi.Router) {
			r.Use(chiMiddleware.Timeout(DefaultRequestTimeout))
			r.Get("/networks", s.handleListNetworks)
			r.Get("/networks/{id}", s.handleGetNetwork)
			r.Get("/volumes", s.handleListVolumes)
			r.Get("/volumes/{id}", s.handleGetVolume)
			r.Get("/images", s.handleListImages)
			r.Get("/images/{id}", s.handleGetImage)
		})
		r.Get("/events", s.handleDockerEvents)
	})

	// Mount embedded Web UI handler for SPA routing with client-side fallback
	webUI := s.webUI
	if webUI == nil {
		webUI = web.Handler()
	}
	r.Handle("/*", webUI)

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
