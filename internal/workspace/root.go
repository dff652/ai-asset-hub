package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var managedToolDirectories = []string{".agents", ".claude", ".codex", ".grok"}

// ValidateExistingRoot validates one already-existing workspace root without
// creating or modifying it. Callers that only need to prefill or inspect a
// workspace must use this instead of PrepareRoot so validation cannot become
// implicit write authorization.
func ValidateExistingRoot(candidate, home, project string) (string, error) {
	absolute, err := validateRootCandidate(candidate, home, project)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: workspace root is not an accessible directory", ErrInvalidOptions)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("%w: resolve workspace root", ErrInvalidOptions)
	}
	return filepath.Clean(resolved), nil
}

// PrepareRoot opens or creates one workspace root that the user explicitly
// named. It does not choose a default path or allow a managed tool directory
// to become the workspace.
func PrepareRoot(candidate, home, project string) (string, bool, error) {
	absolute, err := validateRootCandidate(candidate, home, project)
	if err != nil {
		return "", false, err
	}
	created := false
	info, err := os.Stat(absolute)
	switch {
	case err == nil && !info.IsDir():
		return "", false, fmt.Errorf("%w: workspace root is not a directory", ErrInvalidOptions)
	case err == nil:
	case os.IsNotExist(err):
		if err := os.MkdirAll(absolute, 0o755); err != nil {
			return "", false, fmt.Errorf("%w: create workspace root: %v", ErrInvalidOptions, err)
		}
		created = true
	default:
		return "", false, fmt.Errorf("%w: inspect workspace root: %v", ErrInvalidOptions, err)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute), created, nil
}

func validateRootCandidate(candidate, home, project string) (string, error) {
	value := strings.TrimSpace(candidate)
	if value == "" {
		return "", ErrInvalidOptions
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		if strings.TrimSpace(home) == "" {
			return "", ErrInvalidOptions
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}

	absolute, err := resolveBoundaryPath(value)
	if err != nil {
		return "", fmt.Errorf("%w: resolve workspace root", ErrInvalidOptions)
	}
	managed, err := overlapsManagedToolDirectory(absolute, home, project)
	if err != nil {
		return "", fmt.Errorf("%w: resolve managed tool directories", ErrInvalidOptions)
	}
	if managed {
		return "", fmt.Errorf("%w: workspace root overlaps a managed tool directory", ErrInvalidOptions)
	}
	return filepath.Clean(absolute), nil
}

func overlapsManagedToolDirectory(candidate, home, project string) (bool, error) {
	resolvedCandidate, err := resolveBoundaryPath(candidate)
	if err != nil {
		return false, err
	}
	for _, boundary := range []string{home, project} {
		if strings.TrimSpace(boundary) == "" {
			continue
		}
		for _, name := range managedToolDirectories {
			root, err := resolveBoundaryPath(filepath.Join(boundary, name))
			if err != nil {
				return false, err
			}
			if Within(root, resolvedCandidate) {
				return true, nil
			}
		}
	}
	return false, nil
}

// resolveBoundaryPath follows every existing symlink prefix while preserving
// non-existent trailing components. The pre-create boundary check therefore
// also catches paths such as <symlink-to-.claude>/new-workspace.
func resolveBoundaryPath(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absolute)
	trailing := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(trailing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, trailing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(absolute), nil
		}
		trailing = append(trailing, filepath.Base(current))
		current = parent
	}
}
