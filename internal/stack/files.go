package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxFileSize = 256 * 1024 // 256KB

// ReadDirectoryFiles reads candidate config and env files in the specified directory.
// Ignores subdirectories and dotfiles except .env*.
// Reads text/config files up to 256KB and checks for valid UTF-8.
func ReadDirectoryFiles(dir string) ([]StackFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	files := make([]StackFile, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryName := entry.Name()
		// Exclude dotfiles except .env*
		if strings.HasPrefix(entryName, ".") && !strings.HasPrefix(entryName, ".env") {
			continue
		}

		fullPath := filepath.Join(dir, entryName)
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		isCompose := isComposeFilename(entryName)
		isEnv := isEnvFilename(entryName)

		var content string
		if fi.Size() <= maxFileSize {
			data, err := os.ReadFile(fullPath)
			if err == nil && utf8.Valid(data) {
				content = string(data)
			}
		}

		files = append(files, StackFile{
			Name:      entryName,
			Path:      fullPath,
			Size:      fi.Size(),
			Content:   content,
			IsCompose: isCompose,
			IsEnv:     isEnv,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsCompose != files[j].IsCompose {
			return files[i].IsCompose
		}
		if files[i].IsEnv != files[j].IsEnv {
			return files[i].IsEnv
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}
