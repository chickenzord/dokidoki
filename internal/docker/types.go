package docker

// PortMapping represents an exposed or published container port.
type PortMapping struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	Type        string `json:"type"`
}

// Container represents container metadata normalized for Dokidoki.
type Container struct {
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
	IsSelf  bool              `json:"is_self"`
}

// HostInfo provides Docker engine and daemon runtime details.
type HostInfo struct {
	EngineVersion     string `json:"engineVersion"`
	APIVersion        string `json:"apiVersion"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	Containers        int    `json:"containers"`
	ContainersRunning int    `json:"containersRunning"`
	ContainersPaused  int    `json:"containersPaused"`
	ContainersStopped int    `json:"containersStopped"`
}
