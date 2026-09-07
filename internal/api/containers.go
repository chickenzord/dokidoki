package api

import (
	"net/http"
	"strings"

	"github.com/chickenzord/dokidoki/internal/model"
	"github.com/chickenzord/dokidoki/internal/stacks"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
)

// EnrichedContainerInspect represents a Docker ContainerJSON enriched with Dokidoki stack metadata.
type EnrichedContainerInspect struct {
	types.ContainerJSON
	StackName   string `json:"stack_name"`
	ServiceName string `json:"service_name"`
	Source      string `json:"source"`
	IsSelf      bool   `json:"is_self"`
}

// handleListContainers handles GET /api/v1/containers.
// Supports query parameters:
// - ?grouped=true: returns model.GroupedContainers
// - ?stack={name}: returns []model.ContainerSummary filtered by stack
// - otherwise: returns flat []model.ContainerSummary
func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	discovered, err := s.scanner.Scan()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan stacks: "+err.Error())
		return
	}

	rawContainers, err := s.dockerCli.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list containers: "+err.Error())
		return
	}

	categorized := stacks.CategorizeRaw(discovered, rawContainers, s.selfContainerID)

	// Check grouped query param
	if r.URL.Query().Get("grouped") == "true" {
		grouped := categorized.Grouped
		if grouped.Stacks == nil {
			grouped.Stacks = map[string][]model.ContainerSummary{}
		}
		if grouped.Standalone == nil {
			grouped.Standalone = []model.ContainerSummary{}
		}
		writeJSON(w, http.StatusOK, grouped)
		return
	}

	// Check stack filter query param
	if stackFilter := strings.TrimSpace(r.URL.Query().Get("stack")); stackFilter != "" {
		var filtered []model.ContainerSummary
		if list, ok := categorized.Grouped.Stacks[stackFilter]; ok && list != nil {
			filtered = list
		} else {
			filtered = []model.ContainerSummary{}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	// Otherwise, return all containers flat
	all := stacks.ConvertContainers(rawContainers, s.selfContainerID)
	if all == nil {
		all = []model.ContainerSummary{}
	}

	writeJSON(w, http.StatusOK, all)
}

// handleInspectContainer handles GET /api/v1/containers/{id}.
func (s *Server) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	inspect, err := s.dockerCli.InspectContainer(r.Context(), id)
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect container: "+err.Error())
		return
	}

	var stackName, serviceName string
	if inspect.Config != nil && inspect.Config.Labels != nil {
		stackName = inspect.Config.Labels[model.ComposeProjectLabel]
		serviceName = inspect.Config.Labels[model.ComposeServiceLabel]
	}

	var source string
	if stackName == "" {
		source = "standalone"
	} else {
		isManaged := false
		if discovered, err := s.scanner.Scan(); err == nil {
			for _, d := range discovered {
				if d.Name == stackName {
					isManaged = true
					break
				}
			}
		}

		if isManaged {
			source = model.StackSourceManaged
		} else {
			source = model.StackSourceExternal
		}
	}

	isSelf := false
	if s.selfContainerID != "" {
		if inspect.ID == s.selfContainerID ||
			id == s.selfContainerID ||
			(len(s.selfContainerID) >= 12 && strings.HasPrefix(inspect.ID, s.selfContainerID)) ||
			(len(inspect.ID) >= 12 && strings.HasPrefix(s.selfContainerID, inspect.ID)) {
			isSelf = true
		}
	}

	enriched := EnrichedContainerInspect{
		ContainerJSON: inspect,
		StackName:     stackName,
		ServiceName:   serviceName,
		Source:        source,
		IsSelf:        isSelf,
	}

	writeJSON(w, http.StatusOK, enriched)
}
