package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	// ErrNotFound is returned when a requested stack is not found.
	ErrNotFound = errors.New("stack not found")

	// ErrComposeNotFound is returned when a compose file is not found for a stack.
	ErrComposeNotFound = errors.New("compose file not found")

	// ErrContainerNotFound is returned when a container is not found.
	ErrContainerNotFound = errors.New("container not found")

	// ErrInvalidSource is returned when an invalid source filter is requested.
	ErrInvalidSource = errors.New("invalid source parameter, must be managed, external, or all")
)

// Service provides domain logic for managing stacks and containers.
type Service struct {
	scanner         *Scanner
	dockerCli       docker.Client
	selfContainerID string
}

// NewService creates a new stack domain Service.
func NewService(scanner *Scanner, dockerCli docker.Client, selfContainerID string) *Service {
	return &Service{
		scanner:         scanner,
		dockerCli:       dockerCli,
		selfContainerID: selfContainerID,
	}
}

func (s *Service) getCategorized(ctx context.Context) (*CategorizedResult, []types.Container, error) {
	var discovered []Discovered
	if s.scanner != nil {
		var err error
		discovered, err = s.scanner.Scan()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan stacks: %w", err)
		}
	}

	var rawContainers []types.Container
	if s.dockerCli != nil {
		var err error
		rawContainers, err = s.dockerCli.ListContainers(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list containers: %w", err)
		}
	}

	categorized := CategorizeRaw(discovered, rawContainers, s.selfContainerID)
	return categorized, rawContainers, nil
}

// ListStacks returns stack summaries filtered by source ("managed", "external", or "all").
func (s *Service) ListStacks(ctx context.Context, source string) ([]Summary, error) {
	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	sourceParam := strings.ToLower(strings.TrimSpace(source))
	if sourceParam == "" {
		sourceParam = "all"
	}

	var result []Summary
	switch sourceParam {
	case "all":
		result = categorized.Stacks
	case "managed":
		result = categorized.Managed()
	case "external":
		result = categorized.External()
	default:
		return nil, ErrInvalidSource
	}

	if result == nil {
		result = []Summary{}
	}
	return result, nil
}

// GetStack returns detailed stack information including its containers.
func (s *Service) GetStack(ctx context.Context, name string) (*Detail, error) {
	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	detail, found := categorized.GetStack(name)
	if !found {
		return nil, ErrNotFound
	}

	if detail.Containers == nil {
		detail.Containers = []docker.Container{}
	}
	if detail.Services == nil {
		detail.Services = []string{}
	}
	return &detail, nil
}

// GetStackCompose returns the compose file path and content for a given stack.
func (s *Service) GetStackCompose(ctx context.Context, name string) (*ComposeResponse, error) {
	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	detail, found := categorized.GetStack(name)
	if !found || !detail.ComposePresent || detail.ComposePath == "" {
		return nil, ErrComposeNotFound
	}

	content, err := ReadComposeFile(detail.ComposePath)
	if err != nil {
		return nil, ErrComposeNotFound
	}

	return &ComposeResponse{
		Name:    detail.Name,
		Path:    detail.ComposePath,
		Content: content,
	}, nil
}

// GetStackContainers returns a list of containers belonging to the given stack.
func (s *Service) GetStackContainers(ctx context.Context, name string) ([]docker.Container, error) {
	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	detail, found := categorized.GetStack(name)
	if !found {
		return nil, ErrNotFound
	}

	containers := detail.Containers
	if containers == nil {
		containers = []docker.Container{}
	}
	return containers, nil
}

// ListContainers returns containers in either flat list, grouped structure, or filtered by stack.
func (s *Service) ListContainers(ctx context.Context, stackFilter string, grouped bool) (any, error) {
	categorized, rawContainers, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	if grouped {
		grp := categorized.Grouped
		if grp.Stacks == nil {
			grp.Stacks = map[string][]docker.Container{}
		}
		if grp.Standalone == nil {
			grp.Standalone = []docker.Container{}
		}
		for k, v := range grp.Stacks {
			if v == nil {
				grp.Stacks[k] = []docker.Container{}
			}
		}
		return grp, nil
	}

	if stackFilter = strings.TrimSpace(stackFilter); stackFilter != "" {
		var filtered []docker.Container
		if list, ok := categorized.Grouped.Stacks[stackFilter]; ok && list != nil {
			filtered = list
		} else {
			filtered = []docker.Container{}
		}
		return filtered, nil
	}

	all := docker.ConvertContainers(rawContainers, s.selfContainerID)
	if all == nil {
		all = []docker.Container{}
	}
	return all, nil
}

// InspectContainer returns container inspection enriched with stack and self metadata.
func (s *Service) InspectContainer(ctx context.Context, id string) (*EnrichedContainerInspect, error) {
	if s.dockerCli == nil {
		return nil, ErrContainerNotFound
	}

	inspect, err := s.dockerCli.InspectContainer(ctx, id)
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}

	var stackName, serviceName string
	if inspect.Config != nil && inspect.Config.Labels != nil {
		stackName = inspect.Config.Labels[docker.ComposeProjectLabel]
		serviceName = inspect.Config.Labels[docker.ComposeServiceLabel]
	}

	var source string
	if stackName == "" {
		source = "standalone"
	} else {
		isManaged := false
		if s.scanner != nil {
			if discovered, err := s.scanner.Scan(); err == nil {
				for _, d := range discovered {
					if d.Name == stackName {
						isManaged = true
						break
					}
				}
			}
		}

		if isManaged {
			source = string(SourceManaged)
		} else {
			source = string(SourceExternal)
		}
	}

	isSelf := docker.IsSelfContainer(inspect.ID, s.selfContainerID) || docker.IsSelfContainer(id, s.selfContainerID)

	enriched := &EnrichedContainerInspect{
		ContainerJSON: inspect,
		StackName:     stackName,
		ServiceName:   serviceName,
		Source:        source,
		IsSelf:        isSelf,
	}
	return enriched, nil
}
