package stack

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ComposeFileCandidates lists the compose filenames in order of preference.
var ComposeFileCandidates = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

// Scanner scans a directory for Docker Compose stacks.
type Scanner struct {
	stacksDir string
}

// NewScanner returns a new Scanner for the given stacks directory.
func NewScanner(stacksDir string) *Scanner {
	return &Scanner{stacksDir: stacksDir}
}

// StacksDir returns the configured stacks directory path.
func (s *Scanner) StacksDir() string {
	if s == nil {
		return ""
	}
	return s.stacksDir
}

// Scan searches the configured stacks directory for stacks.
func (s *Scanner) Scan() ([]Discovered, error) {
	return ScanDir(s.stacksDir)
}

// ScanDir searches the given directory for subdirectories containing compose files.
func ScanDir(stacksDir string) ([]Discovered, error) {
	entries, err := os.ReadDir(stacksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Discovered{}, nil
		}
		return nil, err
	}

	discovered := make([]Discovered, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		stackDir := filepath.Join(stacksDir, name)
		composePath, found := FindComposeFile(stackDir)
		if found {
			discovered = append(discovered, Discovered{
				Name:           name,
				ComposePath:    composePath,
				ComposePresent: true,
			})
		}
	}

	// Sort alphabetically for deterministic ordering
	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].Name < discovered[j].Name
	})

	return discovered, nil
}

// FindComposeFile checks candidate filenames in order of preference within a directory.
func FindComposeFile(dir string) (string, bool) {
	for _, candidate := range ComposeFileCandidates {
		p := filepath.Join(dir, candidate)
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return p, true
		}
	}
	return "", false
}

// ReadComposeFile reads the content of the compose file at path.
func ReadComposeFile(path string) (string, error) {
	if path == "" {
		return "", os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetComposeFileResponse reads and constructs a ComposeResponse.
func GetComposeFileResponse(name, path string) (*ComposeResponse, error) {
	content, err := ReadComposeFile(path)
	if err != nil {
		return nil, err
	}
	return &ComposeResponse{
		Name:    name,
		Path:    path,
		Content: content,
	}, nil
}
