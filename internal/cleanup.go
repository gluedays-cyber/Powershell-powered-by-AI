package internal

import (
	"os"
	"path/filepath"
)

func Cleanup() {
	tempDir := os.TempDir()
	patterns := []string{
		filepath.Join(tempDir, "py-cli-*.py"),
		filepath.Join(tempDir, "go-cli-*.go"),
		filepath.Join(tempDir, "go-cli-*.exe"),
		filepath.Join(tempDir, "go-cli-*"),
		filepath.Join(tempDir, "gen-cli-*.*"),
		filepath.Join(tempDir, "gen-cli-*"),
		filepath.Join(".", "go-cli-*.go"),
		filepath.Join(".", "go-cli-*.exe"),
		filepath.Join(".", "go-cli-*"),
		filepath.Join(".", "gen-cli-*.*"),
		filepath.Join(".", "gen-cli-*"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			_ = os.Remove(match)
		}
	}
}
