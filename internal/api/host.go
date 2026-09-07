package api

import (
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/chickenzord/dokidoki/internal/model"
)

// handleHostInfo handles GET /api/v1/host.
// Returns host system and Docker engine information.
func (s *Server) handleHostInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hostname, _ := os.Hostname()
	hostInfo := model.HostInfo{
		Hostname:        hostname,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		DokidokiVersion: "0.1.0",
		StacksDir:       s.stacksDir,
		Docker:          model.DockerHostInfo{},
	}

	ver, verErr := s.dockerCli.ServerVersion(ctx)
	if verErr == nil {
		hostInfo.Docker.EngineVersion = ver.Version
		hostInfo.Docker.APIVersion = ver.APIVersion
		hostInfo.Docker.OS = ver.Os
		hostInfo.Docker.Arch = ver.Arch
	}

	info, infoErr := s.dockerCli.Info(ctx)
	if infoErr == nil {
		hostInfo.Docker.Containers = info.Containers
		hostInfo.Docker.ContainersRunning = info.ContainersRunning
		hostInfo.Docker.ContainersPaused = info.ContainersPaused
		hostInfo.Docker.ContainersStopped = info.ContainersStopped
		if hostInfo.Docker.OS == "" {
			hostInfo.Docker.OS = info.OperatingSystem
		}
		if hostInfo.Docker.Arch == "" {
			hostInfo.Docker.Arch = info.Architecture
		}
	} else {
		// Fallback: If Info() is blocked by a proxy, calculate container counts via ListContainers
		if rawContainers, err := s.dockerCli.ListContainers(ctx); err == nil {
			hostInfo.Docker.Containers = len(rawContainers)
			for _, c := range rawContainers {
				switch strings.ToLower(strings.TrimSpace(c.State)) {
				case "running":
					hostInfo.Docker.ContainersRunning++
				case "paused":
					hostInfo.Docker.ContainersPaused++
				case "exited", "dead":
					hostInfo.Docker.ContainersStopped++
				}
			}
		} else if verErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to get docker info: "+infoErr.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, hostInfo)
}

// handlePing handles GET /api/v1/host/ping.
// Fast liveness check that verifies connectivity to Docker daemon.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	_, err := s.dockerCli.Ping(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
