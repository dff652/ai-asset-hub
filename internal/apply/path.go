package apply

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

var (
	errUnsafePath = errors.New("unsafe install path")
	// errSymlinkInPath separates "your environment has a symlink where this
	// file goes" from "this path escapes the install root". Both refuse to
	// write, but only the first is something the user can fix.
	errSymlinkInPath = errors.New("existing symlink in install path")
)

// securePath returns a canonical lexical destination and rejects any existing
// symlink in the path. It is called during planning and again immediately
// before writes to reduce the gap between validation and mutation.
func securePath(root, relative string) (string, error) {
	if !workspace.SafeRelativePath(relative) {
		return "", errUnsafePath
	}
	native := filepath.FromSlash(relative)
	clean := filepath.Clean(native)
	if clean == "." || filepath.ToSlash(clean) != filepath.ToSlash(native) {
		return "", errUnsafePath
	}
	absolute := filepath.Join(root, clean)
	if !workspace.Within(root, absolute) {
		return "", errUnsafePath
	}

	current := root
	parts := splitPath(clean)
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return absolute, nil
		}
		if err != nil {
			return "", errUnsafePath
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errSymlinkInPath
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", errUnsafePath
		}
	}
	return absolute, nil
}

func secureRegularFile(root, relative string) (string, error) {
	absolute, err := securePath(root, relative)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return "", errUnsafePath
	}
	return absolute, nil
}

func splitPath(value string) []string {
	volume := filepath.VolumeName(value)
	value = value[len(volume):]
	parts := make([]string, 0)
	for value != "" && value != "." {
		dir, base := filepath.Split(value)
		if base != "" {
			parts = append([]string{base}, parts...)
		}
		value = filepath.Clean(dir)
		if value == string(filepath.Separator) {
			break
		}
	}
	return parts
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 128 || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
