package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
)

// handleListNetworks handles GET /api/networks and GET /api/v1/networks.
func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	if s.dockerCli == nil {
		writeJSON(w, http.StatusOK, []network.Inspect{})
		return
	}

	nets, err := s.dockerCli.NetworkList(r.Context(), network.ListOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list networks: "+err.Error())
		return
	}

	if nets == nil {
		nets = []network.Inspect{}
	}
	writeJSON(w, http.StatusOK, nets)
}

// handleGetNetwork handles GET /api/networks/{id} and GET /api/v1/networks/{id}.
func (s *Server) handleGetNetwork(w http.ResponseWriter, r *http.Request) {
	if s.dockerCli == nil {
		writeError(w, http.StatusNotFound, "network not found")
		return
	}

	id := chi.URLParam(r, "id")
	verbose := r.URL.Query().Get("verbose") == "true"
	scope := r.URL.Query().Get("scope")

	netInspect, err := s.dockerCli.NetworkInspect(r.Context(), id, network.InspectOptions{
		Verbose: verbose,
		Scope:   scope,
	})
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "no such network") {
			writeError(w, http.StatusNotFound, "network not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect network: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, netInspect)
}

// handleListVolumes handles GET /api/volumes and GET /api/v1/volumes.
func (s *Server) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	if s.dockerCli == nil {
		writeJSON(w, http.StatusOK, []volume.Volume{})
		return
	}

	res, err := s.dockerCli.VolumeList(r.Context(), volume.ListOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list volumes: "+err.Error())
		return
	}

	vols := res.Volumes
	if vols == nil {
		vols = []*volume.Volume{}
	}
	writeJSON(w, http.StatusOK, vols)
}

// handleGetVolume handles GET /api/volumes/{id} and GET /api/v1/volumes/{id}.
func (s *Server) handleGetVolume(w http.ResponseWriter, r *http.Request) {
	if s.dockerCli == nil {
		writeError(w, http.StatusNotFound, "volume not found")
		return
	}

	id := chi.URLParam(r, "id")
	vol, err := s.dockerCli.VolumeInspect(r.Context(), id)
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "no such volume") {
			writeError(w, http.StatusNotFound, "volume not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect volume: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, vol)
}

// handleListImages handles GET /api/images and GET /api/v1/images.
func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	if s.dockerCli == nil {
		writeJSON(w, http.StatusOK, []image.Summary{})
		return
	}

	all := r.URL.Query().Get("all") == "true"
	imgs, err := s.dockerCli.ImageList(r.Context(), image.ListOptions{All: all})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list images: "+err.Error())
		return
	}

	if imgs == nil {
		imgs = []image.Summary{}
	}
	writeJSON(w, http.StatusOK, imgs)
}

// handleGetImage handles GET /api/images/{id} and GET /api/v1/images/{id}.
func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	if s.dockerCli == nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}

	id := chi.URLParam(r, "id")
	inspect, err := s.dockerCli.ImageInspect(r.Context(), id)
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "no such image") {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect image: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, inspect)
}

// handleDockerEvents handles GET /api/events and GET /api/v1/events with SSE streaming.
func (s *Server) handleDockerEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	if s.dockerCli == nil {
		writeError(w, http.StatusServiceUnavailable, "docker client unavailable")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	since := r.URL.Query().Get("since")
	until := r.URL.Query().Get("until")

	opts := events.ListOptions{
		Since: since,
		Until: until,
	}

	ctx := r.Context()
	msgCh, errCh := s.dockerCli.Events(ctx, opts)

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if !ok || err == nil {
				return
			}
			data, _ := json.Marshal(map[string]string{"error": err.Error()})
			_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
			flusher.Flush()
			return
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
