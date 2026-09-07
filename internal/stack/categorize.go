package stack

import (
	"sort"
	"strings"

	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/docker/docker/api/types"
)

// CategorizedResult holds the result of categorizing containers and stacks.
type CategorizedResult struct {
	Stacks     []Summary          `json:"stacks"`
	Details    map[string]Detail  `json:"details"`
	Grouped    Grouped            `json:"grouped"`
	Standalone []docker.Container `json:"standalone"`
}

// GetStack returns the Detail for the requested stack name, if found.
func (r *CategorizedResult) GetStack(name string) (Detail, bool) {
	detail, ok := r.Details[name]
	return detail, ok
}

// Managed returns only the managed stack summaries.
func (r *CategorizedResult) Managed() []Summary {
	result := make([]Summary, 0)
	for _, s := range r.Stacks {
		if s.Source == string(SourceManaged) {
			result = append(result, s)
		}
	}
	return result
}

// ManagedStacks is an alias for Managed for backwards compatibility.
func (r *CategorizedResult) ManagedStacks() []Summary {
	return r.Managed()
}

// External returns only the external stack summaries.
func (r *CategorizedResult) External() []Summary {
	result := make([]Summary, 0)
	for _, s := range r.Stacks {
		if s.Source == string(SourceExternal) {
			result = append(result, s)
		}
	}
	return result
}

// ExternalStacks is an alias for External for backwards compatibility.
func (r *CategorizedResult) ExternalStacks() []Summary {
	return r.External()
}

// CalculateRollup aggregates container counts by their raw state.
func CalculateRollup(containers []docker.Container) Rollup {
	var rollup Rollup
	for _, c := range containers {
		rollup.Total++
		switch strings.ToLower(strings.TrimSpace(c.State)) {
		case "running":
			rollup.Running++
		case "exited":
			rollup.Exited++
		case "restarting":
			rollup.Restarting++
		case "paused":
			rollup.Paused++
		case "dead":
			rollup.Dead++
		}
	}
	return rollup
}

// extractServices returns a sorted slice of unique service names from containers.
func extractServices(containers []docker.Container) []string {
	serviceSet := make(map[string]struct{})
	for _, c := range containers {
		if s := strings.TrimSpace(c.Service); s != "" {
			serviceSet[s] = struct{}{}
		}
	}
	services := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		services = append(services, s)
	}
	sort.Strings(services)
	return services
}

// Categorize groups discovered filesystem stacks and container summaries into managed,
// external, and standalone categories with aggregated rollups.
func Categorize(discovered []Discovered, containers []docker.Container) *CategorizedResult {
	managedMap := make(map[string]Discovered, len(discovered))
	for _, d := range discovered {
		managedMap[d.Name] = d
	}

	stackContainers := make(map[string][]docker.Container)
	standalone := make([]docker.Container, 0)

	for _, c := range containers {
		stackName := strings.TrimSpace(c.Stack)
		if stackName == "" {
			standalone = append(standalone, c)
		} else {
			stackContainers[stackName] = append(stackContainers[stackName], c)
		}
	}

	details := make(map[string]Detail)
	summaries := make([]Summary, 0)

	// 1. Process Managed Stacks (defined in stacksDir)
	for _, d := range discovered {
		cList := stackContainers[d.Name]
		if cList == nil {
			cList = []docker.Container{}
		}

		rollup := CalculateRollup(cList)
		services := extractServices(cList)

		summary := Summary{
			Name:           d.Name,
			Source:         string(SourceManaged),
			ComposePresent: d.ComposePresent,
			ComposePath:    d.ComposePath,
			Rollup:         rollup,
			Services:       services,
		}

		detail := Detail{
			Summary:    summary,
			Containers: cList,
		}

		summaries = append(summaries, summary)
		details[d.Name] = detail
	}

	// 2. Process External Stacks (containers with compose label not in managed stacks)
	for stackName, cList := range stackContainers {
		if _, isManaged := managedMap[stackName]; isManaged {
			continue
		}

		rollup := CalculateRollup(cList)
		services := extractServices(cList)

		summary := Summary{
			Name:           stackName,
			Source:         string(SourceExternal),
			ComposePresent: false,
			ComposePath:    "",
			Rollup:         rollup,
			Services:       services,
		}

		detail := Detail{
			Summary:    summary,
			Containers: cList,
		}

		summaries = append(summaries, summary)
		details[stackName] = detail
	}

	// Sort summaries alphabetically by stack Name for deterministic ordering
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	// Ensure all stacks in Grouped have non-nil slices
	groupedStacks := make(map[string][]docker.Container, len(details))
	for name, detail := range details {
		if detail.Containers == nil {
			groupedStacks[name] = []docker.Container{}
		} else {
			groupedStacks[name] = detail.Containers
		}
	}

	return &CategorizedResult{
		Stacks:  summaries,
		Details: details,
		Grouped: Grouped{
			Stacks:     groupedStacks,
			Standalone: standalone,
		},
		Standalone: standalone,
	}
}

// CategorizeRaw converts raw Docker containers and categorizes them against discovered stacks.
func CategorizeRaw(discovered []Discovered, raw []types.Container, selfContainerID string) *CategorizedResult {
	return Categorize(discovered, docker.ConvertContainers(raw, selfContainerID))
}

// CategorizeContainers converts containers with selfContainerID and categorizes them against managed stacks map.
func CategorizeContainers(containers []types.Container, managed map[string]string, selfContainerID string) *CategorizedResult {
	var discovered []Discovered
	for name, composePath := range managed {
		discovered = append(discovered, Discovered{
			Name:           name,
			ComposePath:    composePath,
			ComposePresent: composePath != "",
		})
	}
	return CategorizeRaw(discovered, containers, selfContainerID)
}

// ConvertContainer converts a Docker types.Container to a normalized docker.Container.
var ConvertContainer = docker.ConvertContainer

// ConvertContainers converts a slice of Docker types.Container to []docker.Container.
var ConvertContainers = docker.ConvertContainers

