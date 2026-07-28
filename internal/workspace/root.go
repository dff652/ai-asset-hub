package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareRoot opens or creates one workspace root that the user explicitly
// named. It does not choose a default path.
func PrepareRoot(candidate, home string) (string, bool, error) {
	value := strings.TrimSpace(candidate)
	if value == "" {
		return "", false, ErrInvalidOptions
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		if strings.TrimSpace(home) == "" {
			return "", false, ErrInvalidOptions
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}

	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", false, fmt.Errorf("%w: resolve workspace root", ErrInvalidOptions)
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
