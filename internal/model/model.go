package model

import "time"

// Stack source constants.
const (
	StackSourceManaged  = "managed"
	StackSourceExternal = "external"
)

// Compose labels used by Docker Compose.
const (
	ComposeProjectLabel = "com.docker.compose.project"
	ComposeServiceLabel = "com.docker.compose.service"
)

// ContainerRollup aggregates container counts by raw state.
type ContainerRollup struct {
	Running    int `json:"running"`
	Exited     int `json:"exited"`
	Restarting int `json:"restarting"`
	Paused     int `json:"paused"`
	Dead       int `json:"dead"`
	Total      int `json:"total"`
}

// StackSummary represents high-level stack metadata and container rollups.
type StackSummary struct {
	Name           string          `json:"name"`
	Source         string          `json:"source"` // "managed" | "external"
	ComposePresent bool            `json:"composePresent"`
	ComposePath    string          `json:"composePath,omitempty"`
	Rollup         ContainerRollup `json:"rollup"`
	Services       []string        `json:"services"`
}

// StackDetail provides the StackSummary plus full container details.
type StackDetail struct {
	StackSummary
	Containers []ContainerSummary `json:"containers"`
}

// PortMapping represents an exposed or published container port.
type PortMapping struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	Type        string `json:"type"`
}

// ContainerSummary represents container metadata normalized for Dokidoki.
type ContainerSummary struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Created int64             `json:"created"`
	Ports   []PortMapping     `json:"ports"`
	Labels  map[string]string `json:"labels"`
	Stack   string            `json:"stack,omitempty"`
	Service string            `json:"service,omitempty"`
}

// GroupedContainers contains containers organized by stack alongside standalone containers.
type GroupedContainers struct {
	Stacks     map[string][]ContainerSummary `json:"stacks"`
	Standalone []ContainerSummary            `json:"standalone"`
}

// ComposeFileResponse represents the content and path of a stack's compose file.
type ComposeFileResponse struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// DockerHostInfo provides Docker engine and daemon runtime details.
type DockerHostInfo struct {
	EngineVersion     string `json:"engineVersion"`
	APIVersion        string `json:"apiVersion"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	Containers        int    `json:"containers"`
	ContainersRunning int    `json:"containersRunning"`
	ContainersPaused  int    `json:"containersPaused"`
	ContainersStopped int    `json:"containersStopped"`
}

// HostInfo represents host system and Docker daemon information.
type HostInfo struct {
	Hostname        string         `json:"hostname"`
	OS              string         `json:"os"`
	Arch            string         `json:"arch"`
	DokidokiVersion string         `json:"dokidokiVersion"`
	StacksDir       string         `json:"stacksDir"`
	Docker          DockerHostInfo `json:"docker"`
}

// NodeStatus represents the state of a node in the cluster.
type NodeStatus string

const (
	NodeStatusAlive   NodeStatus = "alive"
	NodeStatusSuspect NodeStatus = "suspect"
	NodeStatusOffline NodeStatus = "offline"
)

// Node represents a Dokidoki cluster member.
type Node struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Addresses []string   `json:"addresses"`
	Status    NodeStatus `json:"status"`
	Version   string     `json:"version"`
	IsSelf    bool       `json:"is_self"`
	LastSeen  time.Time  `json:"last_seen,omitempty"`
}

// HandshakeRequest is sent by a node to introduce itself to a peer.
type HandshakeRequest struct {
	NodeID       string   `json:"node_id"`
	Name         string   `json:"name"`
	Addresses    []string `json:"addresses"`
	Version      string   `json:"version"`
	ClusterToken string   `json:"cluster_token,omitempty"`
}

// HandshakeResponse is returned to an incoming handshake request.
type HandshakeResponse struct {
	NodeID    string   `json:"node_id"`
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Version   string   `json:"version"`
	Peers     []Node   `json:"peers"`
}

// HeartbeatMessage is exchanged periodically between peers.
type HeartbeatMessage struct {
	NodeID       string   `json:"node_id"`
	Addresses    []string `json:"addresses"`
	ClusterToken string   `json:"cluster_token,omitempty"`
	KnownPeers   []Node   `json:"known_peers"`
}
