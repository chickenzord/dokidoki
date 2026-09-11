package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chickenzord/dokidoki/internal/compose"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"gopkg.in/yaml.v3"
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

	// ErrSelfContainer is returned when mutating dokidoki's own container without force.
	ErrSelfContainer = errors.New("cannot perform operation on Dokidoki's own container without force")
)

// ServiceOption configures a Service instance.
type ServiceOption func(*Service)

// WithComposeRunner sets the compose runner for the service.
func WithComposeRunner(runner compose.Runner) ServiceOption {
	return func(s *Service) {
		s.composeRunner = runner
	}
}

// Service provides domain logic for managing stacks and containers.
type Service struct {
	scanner         *Scanner
	dockerCli       docker.Client
	composeRunner   compose.Runner
	selfContainerID string
}

// NewService creates a new stack domain Service.
func NewService(scanner *Scanner, dockerCli docker.Client, selfContainerID string, opts ...ServiceOption) *Service {
	s := &Service{
		scanner:         scanner,
		dockerCli:       dockerCli,
		selfContainerID: selfContainerID,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) getComposeRunner() compose.Runner {
	if s.composeRunner != nil {
		return s.composeRunner
	}
	return compose.NewRunner("", "")
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

func (s *Service) stacksDir() string {
	if s.scanner != nil {
		return s.scanner.StacksDir()
	}
	return ""
}

func isComposeFilename(name string) bool {
	lower := strings.ToLower(name)
	for _, candidate := range ComposeFileCandidates {
		if lower == candidate {
			return true
		}
	}
	return false
}

func isEnvFilename(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, ".env") || strings.HasSuffix(lower, ".env")
}

func isValidStackName(name string) bool {
	if name == "" {
		return false
	}
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

// parseDockerLogsStdout extracts stdout bytes from Docker log stream, demultiplexing stdcopy headers if present.
func parseDockerLogsStdout(raw []byte) []byte {
	var stdoutBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdoutBuf, io.Discard, bytes.NewReader(raw)); err == nil && stdoutBuf.Len() > 0 {
		return stdoutBuf.Bytes()
	}
	return bytes.TrimSpace(raw)
}

func (s *Service) getContainerLogsString(ctx context.Context, id string) (string, error) {
	rc, err := s.dockerCli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	var outBuf, errBuf bytes.Buffer
	if _, copyErr := stdcopy.StdCopy(&outBuf, &errBuf, bytes.NewReader(raw)); copyErr == nil {
		combined := strings.TrimSpace(outBuf.String() + "\n" + errBuf.String())
		return combined, nil
	}
	return strings.TrimSpace(string(raw)), nil
}

// GetStackFiles returns all readable configuration and env files for a stack directory.
// For external stacks, resolves the host directory. If forceRead is false, returns empty files slice.
// If forceRead is true, reads directly on bare-metal or uses a one-off helper container when containerized.
func (s *Service) GetStackFiles(ctx context.Context, name string, forceRead bool) (*StackFilesResponse, error) {
	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	detail, found := categorized.GetStack(name)
	if !found {
		return nil, ErrNotFound
	}

	if detail.Source == string(SourceExternal) {
		hostDir := ""
		for _, c := range detail.Containers {
			if wd, ok := c.Labels["com.docker.compose.project.working_dir"]; ok && wd != "" {
				hostDir = wd
				break
			}
		}
		if hostDir == "" {
			for _, c := range detail.Containers {
				if cf, ok := c.Labels["com.docker.compose.project.config_files"]; ok && cf != "" {
					first := strings.Split(cf, ",")[0]
					hostDir = filepath.Dir(first)
					break
				}
			}
		}
		if hostDir == "" && detail.ComposePath != "" {
			hostDir = filepath.Dir(detail.ComposePath)
		}
		if hostDir != "" {
			hostDir = filepath.Clean(hostDir)
		}

		if !forceRead {
			return &StackFilesResponse{
				Stack: name,
				Dir:   hostDir,
				Files: []StackFile{},
			}, nil
		}

		if hostDir == "" {
			return nil, errors.New("unable to resolve external stack directory")
		}

		if s.selfContainerID == "" {
			files, err := ReadDirectoryFiles(hostDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read host directory: %w", err)
			}
			return &StackFilesResponse{
				Stack: name,
				Dir:   hostDir,
				Files: files,
			}, nil
		}

		// Dokidoki running in container: one-off container execution
		if s.dockerCli == nil {
			return nil, errors.New("docker client unavailable")
		}

		selfInspect, err := s.dockerCli.InspectContainer(ctx, s.selfContainerID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect self container: %w", err)
		}

		image := ""
		if selfInspect.Config != nil && selfInspect.Config.Image != "" {
			image = selfInspect.Config.Image
		} else if selfInspect.Image != "" {
			image = selfInspect.Image
		}
		if image == "" {
			return nil, errors.New("unable to determine Dokidoki container image")
		}

		entrypoint := []string{"dokidoki"}
		if selfInspect.Config != nil && len(selfInspect.Config.Entrypoint) > 0 {
			entrypoint = selfInspect.Config.Entrypoint
		}
		cmd := []string{"read-files", "/mnt/target"}

		createResp, err := s.dockerCli.CreateContainer(ctx, &container.Config{
			Image:      image,
			Entrypoint: entrypoint,
			Cmd:        cmd,
		}, &container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:%s:ro", hostDir, "/mnt/target")},
		}, nil, nil, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create helper container: %w", err)
		}

		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.dockerCli.RemoveContainer(cleanupCtx, createResp.ID, container.RemoveOptions{Force: true})
		}()

		if err := s.dockerCli.StartContainer(ctx, createResp.ID); err != nil {
			return nil, fmt.Errorf("failed to start helper container: %w", err)
		}

		statusCh, errCh := s.dockerCli.WaitContainer(ctx, createResp.ID, container.WaitConditionNotRunning)
		var statusCode int64
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case waitErr := <-errCh:
			if waitErr != nil {
				logs, _ := s.getContainerLogsString(context.Background(), createResp.ID)
				return nil, fmt.Errorf("error waiting for helper container: %w; logs: %s", waitErr, logs)
			}
		case status := <-statusCh:
			statusCode = status.StatusCode
			if status.Error != nil && status.Error.Message != "" {
				logs, _ := s.getContainerLogsString(context.Background(), createResp.ID)
				return nil, fmt.Errorf("helper container error (%d): %s; logs: %s", status.StatusCode, status.Error.Message, logs)
			}
		}

		if statusCode != 0 {
			logs, _ := s.getContainerLogsString(context.Background(), createResp.ID)
			return nil, fmt.Errorf("helper container exited with status %d: %s", statusCode, logs)
		}

		logsRc, err := s.dockerCli.ContainerLogs(ctx, createResp.ID, container.LogsOptions{ShowStdout: true})
		if err != nil {
			return nil, fmt.Errorf("failed to get helper container logs: %w", err)
		}
		defer logsRc.Close()

		rawLogs, err := io.ReadAll(logsRc)
		if err != nil {
			return nil, fmt.Errorf("failed to read helper container logs: %w", err)
		}

		stdoutBytes := parseDockerLogsStdout(rawLogs)
		var files []StackFile
		if err := json.Unmarshal(stdoutBytes, &files); err != nil {
			return nil, fmt.Errorf("failed to parse files JSON from helper container: %w; raw output: %s", err, string(stdoutBytes))
		}

		for i := range files {
			files[i].Path = filepath.Join(hostDir, files[i].Name)
		}

		return &StackFilesResponse{
			Stack: name,
			Dir:   hostDir,
			Files: files,
		}, nil
	}

	// Managed stack
	stacksDir := s.stacksDir()
	if stacksDir == "" {
		return nil, errors.New("stacks directory is not configured")
	}

	stackDir := filepath.Join(stacksDir, name)
	if _, err := os.Stat(stackDir); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to access stack directory: %w", err)
	}

	files, err := ReadDirectoryFiles(stackDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack directory: %w", err)
	}

	return &StackFilesResponse{
		Stack: name,
		Dir:   stackDir,
		Files: files,
	}, nil
}

// GetStackFile returns a single file from the stack directory with path traversal protection.
func (s *Service) GetStackFile(ctx context.Context, name, filename string) (*StackFile, error) {
	cleanName := filepath.Clean(strings.TrimSpace(name))
	if cleanName == "" || cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
		return nil, errors.New("invalid stack name: path traversal detected")
	}

	cleanFilename := filepath.Clean(strings.TrimSpace(filename))
	if cleanFilename == "" || cleanFilename == "." || filepath.IsAbs(cleanFilename) || strings.HasPrefix(cleanFilename, "..") {
		return nil, errors.New("invalid filename: path traversal detected")
	}

	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return nil, err
	}

	detail, found := categorized.GetStack(cleanName)
	if !found {
		return nil, ErrNotFound
	}

	if detail.Source == string(SourceExternal) {
		detectedPath := detail.ComposePath
		if detectedPath == "" {
			for _, c := range detail.Containers {
				if p, ok := c.Labels["com.docker.compose.project.config_files"]; ok && p != "" {
					detectedPath = strings.Split(p, ",")[0]
					break
				}
			}
		}

		isComposeCandidate := isComposeFilename(cleanFilename) || (detectedPath != "" && cleanFilename == filepath.Base(detectedPath))
		if !isComposeCandidate {
			return nil, ErrNotFound
		}

		if detectedPath != "" {
			fi, statErr := os.Stat(detectedPath)
			if statErr == nil && !fi.IsDir() {
				data, readErr := os.ReadFile(detectedPath)
				if readErr == nil {
					var content string
					if fi.Size() < 256*1024 && utf8.Valid(data) {
						content = string(data)
					}
					return &StackFile{
						Name:      filepath.Base(detectedPath),
						Path:      detectedPath,
						Size:      fi.Size(),
						Content:   content,
						IsCompose: true,
						IsEnv:     false,
					}, nil
				}
			}
		}

		// Stub external compose.yaml
		stubContent := fmt.Sprintf("# External stack detected: %s\n# Host compose path: %s\n# Note: Path is outside Dokidoki volume mount.\n# Provide compose content to import and manage this stack in Dokidoki.\n", cleanName, detectedPath)
		return &StackFile{
			Name:      "compose.yaml",
			Path:      detectedPath,
			Size:      int64(len(stubContent)),
			Content:   stubContent,
			IsCompose: true,
			IsEnv:     false,
		}, nil
	}

	// Managed stack
	stacksDir := s.stacksDir()
	if stacksDir == "" {
		return nil, errors.New("stacks directory is not configured")
	}

	stackDir := filepath.Join(stacksDir, cleanName)
	targetPath := filepath.Clean(filepath.Join(stackDir, cleanFilename))

	rel, err := filepath.Rel(stackDir, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return nil, errors.New("invalid file path: path traversal detected")
	}

	fi, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if fi.IsDir() {
		return nil, errors.New("requested path is a directory")
	}

	var content string
	if fi.Size() < 256*1024 {
		data, err := os.ReadFile(targetPath)
		if err == nil && utf8.Valid(data) {
			content = string(data)
		}
	}

	return &StackFile{
		Name:      fi.Name(),
		Path:      targetPath,
		Size:      fi.Size(),
		Content:   content,
		IsCompose: isComposeFilename(fi.Name()),
		IsEnv:     isEnvFilename(fi.Name()),
	}, nil
}

// CreateOrImportStack creates or imports a stack into Dokidoki's managed stacks directory.
func (s *Service) CreateOrImportStack(ctx context.Context, req CreateStackRequest) (*Summary, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("stack name is required")
	}
	if !isValidStackName(name) {
		return nil, errors.New("invalid stack name: must contain only alphanumeric characters, hyphens, and underscores")
	}

	if len(req.Files) == 0 {
		return nil, errors.New("at least one file is required")
	}

	stacksDir := s.stacksDir()
	if stacksDir == "" {
		return nil, errors.New("stacks directory is not configured")
	}

	for _, f := range req.Files {
		cleanName := strings.TrimSpace(f.Name)
		if cleanName == "" || cleanName == "." || strings.Contains(cleanName, "..") || strings.Contains(cleanName, "/") || strings.Contains(cleanName, "\\") {
			return nil, errors.New("invalid file name: path traversal detected")
		}
	}

	stackDir := filepath.Join(stacksDir, name)
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create stack directory: %w", err)
	}

	hasCompose := false
	for _, f := range req.Files {
		cleanName := strings.TrimSpace(f.Name)
		if isComposeFilename(cleanName) {
			hasCompose = true
		}
		filePath := filepath.Join(stackDir, cleanName)
		if err := os.WriteFile(filePath, []byte(f.Content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", cleanName, err)
		}
	}

	if !hasCompose {
		// Check if a compose file already exists in stackDir
		existingCompose := false
		for _, candidate := range ComposeFileCandidates {
			if _, err := os.Stat(filepath.Join(stackDir, candidate)); err == nil {
				existingCompose = true
				break
			}
		}
		if !existingCompose {
			starterContent := fmt.Sprintf("services:\n  %s:\n    image: nginx:alpine\n    restart: unless-stopped\n", name)
			if err := os.WriteFile(filepath.Join(stackDir, "compose.yaml"), []byte(starterContent), 0644); err != nil {
				return nil, fmt.Errorf("failed to write default compose file: %w", err)
			}
		}
	}

	detail, err := s.GetStack(ctx, name)
	if err == nil {
		return &detail.Summary, nil
	}

	return &Summary{
		Name:            name,
		Source:          string(SourceManaged),
		ComposePresent:  true,
		ComposePath:     filepath.Join(stackDir, "compose.yaml"),
		Rollup:          Rollup{},
		Services:        []string{},
		PendingImport:  false,
	}, nil
}

// ContainerLogs returns the logs of a container.
func (s *Service) ContainerLogs(ctx context.Context, id string, options container.LogsOptions) (io.ReadCloser, error) {
	if s.dockerCli == nil {
		return nil, ErrContainerNotFound
	}
	rc, err := s.dockerCli.ContainerLogs(ctx, id, options)
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, ErrContainerNotFound
		}
		return nil, err
	}
	return rc, nil
}

type composeServiceConfig struct {
	Image       string   `yaml:"image,omitempty"`
	Command     string   `yaml:"command,omitempty"`
	Restart     string   `yaml:"restart,omitempty"`
	Ports       []string `yaml:"ports,omitempty"`
	Environment []string `yaml:"environment,omitempty"`
	Volumes     []string `yaml:"volumes,omitempty"`
}

type composeConfigDoc struct {
	Services map[string]composeServiceConfig `yaml:"services"`
}

// GetContainerCompose inspects a container and generates standard Docker Compose YAML.
func (s *Service) GetContainerCompose(ctx context.Context, id string) (*ContainerComposeResponse, error) {
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

	if inspect.Config == nil {
		return nil, errors.New("container config is nil")
	}

	serviceName := ""
	stackName := ""
	if inspect.Config.Labels != nil {
		serviceName = inspect.Config.Labels[docker.ComposeServiceLabel]
		stackName = inspect.Config.Labels[docker.ComposeProjectLabel]
	}
	if serviceName == "" {
		serviceName = strings.TrimPrefix(inspect.Name, "/")
	}
	if serviceName == "" {
		serviceName = "app"
	}
	serviceName = strings.ReplaceAll(serviceName, " ", "_")
	if stackName == "" {
		stackName = serviceName
	}

	svc := composeServiceConfig{
		Image: inspect.Config.Image,
	}

	if len(inspect.Config.Cmd) > 0 {
		svc.Command = strings.Join(inspect.Config.Cmd, " ")
	}

	if inspect.HostConfig != nil {
		restart := inspect.HostConfig.RestartPolicy.Name
		if restart != "" && restart != "no" {
			svc.Restart = string(restart)
		}

		// Ports
		if len(inspect.HostConfig.PortBindings) > 0 {
			for containerPort, bindings := range inspect.HostConfig.PortBindings {
				portStr := string(containerPort)
				if strings.HasSuffix(portStr, "/tcp") {
					portStr = strings.TrimSuffix(portStr, "/tcp")
				}
				for _, b := range bindings {
					p := ""
					if b.HostIP != "" && b.HostIP != "0.0.0.0" && b.HostIP != "::" {
						p = fmt.Sprintf("%s:%s:%s", b.HostIP, b.HostPort, portStr)
					} else if b.HostPort != "" {
						p = fmt.Sprintf("%s:%s", b.HostPort, portStr)
					} else {
						p = portStr
					}
					svc.Ports = append(svc.Ports, p)
				}
			}
			sort.Strings(svc.Ports)
		}

		// Volumes / Binds
		if len(inspect.HostConfig.Binds) > 0 {
			svc.Volumes = append(svc.Volumes, inspect.HostConfig.Binds...)
		} else if len(inspect.Mounts) > 0 {
			for _, m := range inspect.Mounts {
				src := m.Source
				if m.Type == "volume" && m.Name != "" {
					src = m.Name
				}
				bindStr := fmt.Sprintf("%s:%s", src, m.Destination)
				if !m.RW {
					bindStr += ":ro"
				}
				svc.Volumes = append(svc.Volumes, bindStr)
			}
		}
		if len(svc.Volumes) > 0 {
			sort.Strings(svc.Volumes)
		}
	}

	// Environment
	if len(inspect.Config.Env) > 0 {
		envList := make([]string, len(inspect.Config.Env))
		copy(envList, inspect.Config.Env)
		sort.Strings(envList)
		svc.Environment = envList
	}

	doc := composeConfigDoc{
		Services: map[string]composeServiceConfig{
			serviceName: svc,
		},
	}

	yamlData, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compose yaml: %w", err)
	}

	return &ContainerComposeResponse{
		StackName: stackName,
		Content:   string(yamlData),
	}, nil
}

// RestartContainer restarts a container by ID, with self-protection safeguard unless force is true.
func (s *Service) RestartContainer(ctx context.Context, id string, force bool) (*OperationResult, error) {
	if s.dockerCli == nil {
		return nil, ErrContainerNotFound
	}
	if docker.IsSelfContainer(id, s.selfContainerID) && !force {
		return nil, ErrSelfContainer
	}

	if err := s.dockerCli.RestartContainer(ctx, id, nil); err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, ErrContainerNotFound
		}
		return nil, fmt.Errorf("failed to restart container: %w", err)
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("container %s restarted successfully", id),
	}, nil
}

// StartContainer starts a container by ID.
func (s *Service) StartContainer(ctx context.Context, id string) (*OperationResult, error) {
	if s.dockerCli == nil {
		return nil, ErrContainerNotFound
	}

	if err := s.dockerCli.StartContainer(ctx, id); err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, ErrContainerNotFound
		}
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("container %s started successfully", id),
	}, nil
}

// StopContainer stops a container by ID, with self-protection safeguard unless force is true.
func (s *Service) StopContainer(ctx context.Context, id string, force bool) (*OperationResult, error) {
	if s.dockerCli == nil {
		return nil, ErrContainerNotFound
	}
	if docker.IsSelfContainer(id, s.selfContainerID) && !force {
		return nil, ErrSelfContainer
	}

	if err := s.dockerCli.StopContainer(ctx, id, nil); err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, ErrContainerNotFound
		}
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("container %s stopped successfully", id),
	}, nil
}

// PullContainerImage pulls the latest image used by a container, streaming progress to out if non-nil.
func (s *Service) PullContainerImage(ctx context.Context, id string, out io.Writer) (*OperationResult, error) {
	if s.dockerCli == nil {
		return nil, ErrContainerNotFound
	}

	inspect, err := s.dockerCli.InspectContainer(ctx, id)
	if err != nil {
		if client.IsErrNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, ErrContainerNotFound
		}
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	if inspect.Config == nil || strings.TrimSpace(inspect.Config.Image) == "" {
		return nil, errors.New("container has no image defined")
	}

	imageRef := inspect.Config.Image
	rc, err := s.dockerCli.PullImage(ctx, imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}
	if rc == nil {
		return &OperationResult{
			Success: true,
			Message: fmt.Sprintf("pulled image %s", imageRef),
		}, nil
	}
	defer rc.Close()

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if out != nil {
		writer = io.MultiWriter(out, &buf)
	}

	if _, err := io.Copy(writer, rc); err != nil {
		return nil, fmt.Errorf("error reading pull response stream: %w", err)
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("successfully pulled image %s", imageRef),
		Output:  buf.String(),
	}, nil
}

// resolveComposeFile finds and validates the compose file for the given stack.
func (s *Service) resolveComposeFile(ctx context.Context, name string) (string, error) {
	cleanName := filepath.Clean(strings.TrimSpace(name))
	if cleanName == "" || cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
		return "", errors.New("invalid stack name: path traversal detected")
	}

	categorized, _, err := s.getCategorized(ctx)
	if err != nil {
		return "", err
	}

	detail, found := categorized.GetStack(cleanName)
	if !found {
		return "", ErrNotFound
	}

	if detail.Source == string(SourceExternal) {
		return "", errors.New("compose operations are not permitted on unmanaged stacks; import the stack into Dokidoki first")
	}

	if !detail.ComposePresent || detail.ComposePath == "" {
		return "", ErrComposeNotFound
	}

	fi, err := os.Stat(detail.ComposePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("compose file %q not found or not accessible on this host mount", detail.ComposePath)
		}
		return "", fmt.Errorf("failed to access compose file %q: %w", detail.ComposePath, err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("compose path %q is a directory, not a file", detail.ComposePath)
	}

	return detail.ComposePath, nil
}

// ComposeUp executes `docker compose up -d --remove-orphans` on the given stack.
// If the stack is in PendingImport, it appends `--force-recreate` to ensure containers are recreated
// and their labels/working_dir point to Dokidoki's stack directory.
func (s *Service) ComposeUp(ctx context.Context, stackName string, out io.Writer) (*OperationResult, error) {
	composePath, err := s.resolveComposeFile(ctx, stackName)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if out != nil {
		writer = io.MultiWriter(out, &buf)
	}

	var extraArgs []string
	if categorized, _, err := s.getCategorized(ctx); err == nil && categorized != nil {
		if detail, ok := categorized.GetStack(stackName); ok && detail.PendingImport {
			extraArgs = append(extraArgs, "--force-recreate")
		}
	}

	runner := s.getComposeRunner()
	if err := runner.Up(ctx, composePath, writer, extraArgs...); err != nil {
		return nil, err
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("stack %s brought up successfully", stackName),
		Output:  buf.String(),
	}, nil
}

// ComposeDown executes `docker compose down` on the given stack.
func (s *Service) ComposeDown(ctx context.Context, stackName string, out io.Writer) (*OperationResult, error) {
	composePath, err := s.resolveComposeFile(ctx, stackName)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if out != nil {
		writer = io.MultiWriter(out, &buf)
	}

	runner := s.getComposeRunner()
	if err := runner.Down(ctx, composePath, writer); err != nil {
		return nil, err
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("stack %s brought down successfully", stackName),
		Output:  buf.String(),
	}, nil
}

// ComposeRestart executes `docker compose restart [service]` on the given stack.
// If service is empty and the stack is in PendingImport, it delegates to ComposeUp (with --force-recreate)
// so that restarting an imported stack recreates containers under Dokidoki management instead of staying pending.
func (s *Service) ComposeRestart(ctx context.Context, stackName string, service string, out io.Writer) (*OperationResult, error) {
	composePath, err := s.resolveComposeFile(ctx, stackName)
	if err != nil {
		return nil, err
	}

	if service == "" {
		if categorized, _, err := s.getCategorized(ctx); err == nil && categorized != nil {
			if detail, ok := categorized.GetStack(stackName); ok && detail.PendingImport {
				return s.ComposeUp(ctx, stackName, out)
			}
		}
	}

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if out != nil {
		writer = io.MultiWriter(out, &buf)
	}

	runner := s.getComposeRunner()
	if err := runner.Restart(ctx, composePath, service, writer); err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("stack %s restarted successfully", stackName)
	if service != "" {
		msg = fmt.Sprintf("service %s in stack %s restarted successfully", service, stackName)
	}

	return &OperationResult{
		Success: true,
		Message: msg,
		Output:  buf.String(),
	}, nil
}

// ComposePull executes `docker compose pull` on the given stack.
func (s *Service) ComposePull(ctx context.Context, stackName string, out io.Writer) (*OperationResult, error) {
	composePath, err := s.resolveComposeFile(ctx, stackName)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if out != nil {
		writer = io.MultiWriter(out, &buf)
	}

	runner := s.getComposeRunner()
	if err := runner.Pull(ctx, composePath, writer); err != nil {
		return nil, err
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("images for stack %s pulled successfully", stackName),
		Output:  buf.String(),
	}, nil
}
