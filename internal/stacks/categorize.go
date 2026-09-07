package stacks

import (
	"sort"
	"strings"

	"github.com/chickenzord/dokidoki/internal/model"
	"github.com/docker/docker/api/types"
)

// CategorizedResult holds the result of categorizing containers and stacks.
type CategorizedResult struct {
	Stacks     []model.StackSummary         `json:"stacks"`
	Details    map[string]model.StackDetail `json:"details"`
	Grouped    model.GroupedContainers      `json:"grouped"`
	Standalone []model.ContainerSummary     `json:"standalone"`
}

// GetStack returns the StackDetail for the requested stack name, if found.
func (r *CategorizedResult) GetStack(name string) (model.StackDetail, bool) {
	detail, ok := r.Details[name]
	return detail, ok
}

// ManagedStacks returns only the managed stack summaries.
func (r *CategorizedResult) ManagedStacks() []model.StackSummary {
	var result []model.StackSummary
	for _, s := range r.Stacks {
		if s.Source == model.StackSourceManaged {
			result = append(result, s)
		}
	}
	return result
}

// ExternalStacks returns only the external stack summaries.
func (r *CategorizedResult) ExternalStacks() []model.StackSummary {
	var result []model.StackSummary
	for _, s := range r.Stacks {
		if s.Source == model.StackSourceExternal {
			result = append(result, s)
		}
	}
	return result
}

// ConvertContainer converts a Docker types.Container to a normalized model.ContainerSummary.
func ConvertContainer(c types.Container) model.ContainerSummary {
	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	if name == "" {
		name = c.ID
		if len(name) > 12 {
			name = name[:12]
		}
	}

	ports := make([]model.PortMapping, len(c.Ports))
	for i, p := range c.Ports {
		ports[i] = model.PortMapping{
			IP:          p.IP,
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
		}
	}

	var stack, service string
	if c.Labels != nil {
		stack = c.Labels[model.ComposeProjectLabel]
		service = c.Labels[model.ComposeServiceLabel]
	}

	labels := make(map[string]string, len(c.Labels))
	for k, v := range c.Labels {
		labels[k] = v
	}

	return model.ContainerSummary{
		ID:      c.ID,
		Name:    name,
		Image:   c.Image,
		State:   c.State,
		Status:  c.Status,
		Created: c.Created,
		Ports:   ports,
		Labels:  labels,
		Stack:   stack,
		Service: service,
	}
}

// ConvertContainers converts a slice of Docker types.Container to model.ContainerSummary.
func ConvertContainers(raw []types.Container) []model.ContainerSummary {
	res := make([]model.ContainerSummary, len(raw))
	for i, c := range raw {
		res[i] = ConvertContainer(c)
	}
	return res
}

// CalculateRollup aggregates container counts by their raw state.
func CalculateRollup(containers []model.ContainerSummary) model.ContainerRollup {
	var rollup model.ContainerRollup
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
func extractServices(containers []model.ContainerSummary) []string {
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
func Categorize(discovered []DiscoveredStack, containers []model.ContainerSummary) *CategorizedResult {
	managedMap := make(map[string]DiscoveredStack, len(discovered))
	for _, d := range discovered {
		managedMap[d.Name] = d
	}

	stackContainers := make(map[string][]model.ContainerSummary)
	standalone := make([]model.ContainerSummary, 0)

	for _, c := range containers {
		stackName := strings.TrimSpace(c.Stack)
		if stackName == "" {
			standalone = append(standalone, c)
		} else {
			stackContainers[stackName] = append(stackContainers[stackName], c)
		}
	}

	details := make(map[string]model.StackDetail)
	var summaries []model.StackSummary

	// 1. Process Managed Stacks (defined in stacksDir)
	for _, d := range discovered {
		cList := stackContainers[d.Name]
		if cList == nil {
			cList = []model.ContainerSummary{}
		}

		rollup := CalculateRollup(cList)
		services := extractServices(cList)

		summary := model.StackSummary{
			Name:           d.Name,
			Source:         model.StackSourceManaged,
			ComposePresent: d.ComposePresent,
			ComposePath:    d.ComposePath,
			Rollup:         rollup,
			Services:       services,
		}

		detail := model.StackDetail{
			StackSummary: summary,
			Containers:   cList,
		}

		summaries = append(summaries, summary)
		details[d.Name] = detail
	}

	// 2. Process External Stacks (containers with com.docker.compose.project not in managed stacks)
	for stackName, cList := range stackContainers {
		if _, isManaged := managedMap[stackName]; isManaged {
			continue
		}

		rollup := CalculateRollup(cList)
		services := extractServices(cList)

		summary := model.StackSummary{
			Name:           stackName,
			Source:         model.StackSourceExternal,
			ComposePresent: false,
			ComposePath:    "",
			Rollup:         rollup,
			Services:       services,
		}

		detail := model.StackDetail{
			StackSummary: summary,
			Containers:   cList,
		}

		summaries = append(summaries, summary)
		details[stackName] = detail
	}

	// Sort summaries alphabetically by stack Name for deterministic ordering
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	// Ensure all stacks in Grouped have non-nil slices
	groupedStacks := make(map[string][]model.ContainerSummary, len(details))
	for name, detail := range details {
		if detail.Containers == nil {
			groupedStacks[name] = []model.ContainerSummary{}
		} else {
			groupedStacks[name] = detail.Containers
		}
	}

	return &CategorizedResult{
		Stacks:     summaries,
		Details:    details,
		Grouped: model.GroupedContainers{
			Stacks:     groupedStacks,
			Standalone: standalone,
		},
		Standalone: standalone,
	}
}

// CategorizeRaw converts raw Docker containers and categorizes them against discovered stacks.
func CategorizeRaw(discovered []DiscoveredStack, raw []types.Container) *CategorizedResult {
	return Categorize(discovered, ConvertContainers(raw))
}
