package api

import (
	"net/http"
	"strings"

	"github.com/chickenzord/dokidoki/internal/model"
	"github.com/chickenzord/dokidoki/internal/stacks"
	"github.com/go-chi/chi/v5"
)

// handleListStacks handles GET /api/v1/stacks.
// Supports query parameter ?source=managed|external|all (default "all").
func (s *Server) handleListStacks(w http.ResponseWriter, r *http.Request) {
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

	sourceParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if sourceParam == "" {
		sourceParam = "all"
	}

	var result []model.StackSummary
	switch sourceParam {
	case "all":
		result = categorized.Stacks
	case model.StackSourceManaged:
		result = categorized.ManagedStacks()
	case model.StackSourceExternal:
		result = categorized.ExternalStacks()
	default:
		writeError(w, http.StatusBadRequest, "invalid source parameter, must be managed, external, or all")
		return
	}

	if result == nil {
		result = []model.StackSummary{}
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGetStack handles GET /api/v1/stacks/{name}.
func (s *Server) handleGetStack(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

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
	detail, found := categorized.GetStack(name)
	if !found {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	if detail.Containers == nil {
		detail.Containers = []model.ContainerSummary{}
	}
	if detail.Services == nil {
		detail.Services = []string{}
	}

	writeJSON(w, http.StatusOK, detail)
}

// handleGetStackCompose handles GET /api/v1/stacks/{name}/compose.
func (s *Server) handleGetStackCompose(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

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
	detail, found := categorized.GetStack(name)
	if !found || !detail.ComposePresent || detail.ComposePath == "" {
		writeError(w, http.StatusNotFound, "compose file not found")
		return
	}

	content, err := stacks.ReadComposeFile(detail.ComposePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "compose file not found")
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/yaml") {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
		return
	}

	resp := model.ComposeFileResponse{
		Name:    detail.Name,
		Path:    detail.ComposePath,
		Content: content,
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetStackContainers handles GET /api/v1/stacks/{name}/containers.
func (s *Server) handleGetStackContainers(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

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
	detail, found := categorized.GetStack(name)
	if !found {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	containers := detail.Containers
	if containers == nil {
		containers = []model.ContainerSummary{}
	}

	writeJSON(w, http.StatusOK, containers)
}
