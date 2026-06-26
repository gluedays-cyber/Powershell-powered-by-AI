package fs

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileState struct {
	Path    string
	Size    int64
	ModTime time.Time
}

func GetDirState(dir string, excludePrefix string) (map[string]FileState, error) {
	state := make(map[string]FileState)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "__pycache__" || name == ".idea" || name == ".vscode" {
				return filepath.SkipDir
			}
			return nil
		}
		if excludePrefix != "" && strings.HasPrefix(filepath.Base(path), excludePrefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			rel = path
		}
		state[rel] = FileState{
			Path:    rel,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		return nil
	})
	return state, err
}

type FsDiff struct {
	Created  []string
	Modified []string
	Deleted  []string
}

func CompareDirStates(before, after map[string]FileState) FsDiff {
	var diff FsDiff
	for path, stateAfter := range after {
		stateBefore, exists := before[path]
		if !exists {
			diff.Created = append(diff.Created, path)
		} else if stateAfter.Size != stateBefore.Size || !stateAfter.ModTime.Equal(stateBefore.ModTime) {
			diff.Modified = append(diff.Modified, path)
		}
	}
	for path := range before {
		if _, exists := after[path]; !exists {
			diff.Deleted = append(diff.Deleted, path)
		}
	}
	return diff
}
