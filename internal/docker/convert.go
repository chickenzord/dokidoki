package docker

import (
	"strings"

	"github.com/docker/docker/api/types"
)

// Compose labels used by Docker Compose.
const (
	ComposeProjectLabel = "com.docker.compose.project"
	ComposeServiceLabel = "com.docker.compose.service"
)

// IsSelfContainer returns true if containerID matches selfContainerID or matches prefix.
func IsSelfContainer(containerID, selfContainerID string) bool {
	if containerID == "" || selfContainerID == "" {
		return false
	}
	if containerID == selfContainerID {
		return true
	}
	if len(selfContainerID) >= 12 && strings.HasPrefix(containerID, selfContainerID) {
		return true
	}
	if len(containerID) >= 12 && strings.HasPrefix(selfContainerID, containerID) {
		return true
	}
	return false
}

// ConvertContainer converts a raw Docker types.Container to a normalized Container domain model.
func ConvertContainer(c types.Container, selfContainerID string) Container {
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

	ports := make([]PortMapping, len(c.Ports))
	for i, p := range c.Ports {
		ports[i] = PortMapping{
			IP:          p.IP,
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
		}
	}

	var stack, service string
	if c.Labels != nil {
		stack = c.Labels[ComposeProjectLabel]
		service = c.Labels[ComposeServiceLabel]
	}

	labels := make(map[string]string, len(c.Labels))
	for k, v := range c.Labels {
		labels[k] = v
	}

	return Container{
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
		IsSelf:  IsSelfContainer(c.ID, selfContainerID),
	}
}

// ConvertContainers converts a slice of raw Docker types.Container to []Container.
func ConvertContainers(raw []types.Container, selfContainerID string) []Container {
	res := make([]Container, len(raw))
	for i, c := range raw {
		res[i] = ConvertContainer(c, selfContainerID)
	}
	return res
}
