package stack

import (
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/docker/docker/api/types"
)

// Source represents where a stack originated ("managed" or "external").
type Source string

const (
	SourceManaged  Source = "managed"
	SourceExternal Source = "external"
)

// Backwards-compatible string constants.
const (
	StackSourceManaged  = "managed"
	StackSourceExternal = "external"
)

// Discovered represents a stack found on the filesystem.
type Discovered struct {
	Name           string `json:"name"`
	ComposePath    string `json:"composePath"`
	ComposePresent bool   `json:"composePresent"`
}

// DiscoveredStack is an alias for Discovered to avoid stuttering.
type DiscoveredStack = Discovered

// Rollup aggregates container counts by raw state.
type Rollup struct {
	Running    int `json:"running"`
	Exited     int `json:"exited"`
	Restarting int `json:"restarting"`
	Paused     int `json:"paused"`
	Dead       int `json:"dead"`
	Total      int `json:"total"`
}

// Summary represents high-level stack metadata and container rollups.
type Summary struct {
	Name           string   `json:"name"`
	Source         string   `json:"source"` // "managed" | "external"
	ComposePresent bool     `json:"composePresent"`
	ComposePath    string   `json:"composePath,omitempty"`
	Rollup         Rollup   `json:"rollup"`
	Services       []string `json:"services"`
}

// Detail provides the Summary plus full container details.
type Detail struct {
	Summary
	Containers []docker.Container `json:"containers"`
}

// Grouped contains containers organized by stack alongside standalone containers.
type Grouped struct {
	Stacks     map[string][]docker.Container `json:"stacks"`
	Standalone []docker.Container            `json:"standalone"`
}

// ComposeResponse represents the content and path of a stack's compose file.
type ComposeResponse struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// EnrichedContainerInspect wraps container inspection data with Dokidoki metadata.
type EnrichedContainerInspect struct {
	types.ContainerJSON
	StackName   string `json:"stack_name"`
	ServiceName string `json:"service_name"`
	Source      string `json:"source"`
	IsSelf      bool   `json:"is_self"`
}

// StackFile represents a file within a stack directory.
type StackFile struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Content   string `json:"content,omitempty"`
	IsCompose bool   `json:"isCompose"`
	IsEnv     bool   `json:"isEnv"`
}

// StackFilesResponse lists all files in a stack directory.
type StackFilesResponse struct {
	Stack string      `json:"stack"`
	Dir   string      `json:"dir"`
	Files []StackFile `json:"files"`
}

// CreateStackRequest specifies the parameters to create or import a stack.
type CreateStackRequest struct {
	Name        string `json:"name"`
	Content     string `json:"content,omitempty"`
	ComposePath string `json:"composePath,omitempty"`
}

// ContainerComposeResponse represents the generated Docker Compose content for a container.
type ContainerComposeResponse struct {
	StackName string `json:"stackName"`
	Content   string `json:"content"`
}

