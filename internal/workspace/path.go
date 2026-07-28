package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveRoot returns the absolute workspace root for a manifest path.
// Empty root means the directory containing the manifest.
func ResolveRoot(manifestPath, root string) (string, error) {
	manifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return "", ErrInvalidOptions
	}
	if root == "" {
		root = filepath.Dir(manifestPath)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", ErrInvalidOptions
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", ErrInvalidOptions
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(root), nil
}

// SafeRelativePath reports whether value is a relative path that cannot escape
// a workspace root via absolute form or ".." segments.
func SafeRelativePath(value string) bool {
	if value == "" || filepath.IsAbs(value) {
		return false
	}
	slash := filepath.ToSlash(value)
	if strings.HasPrefix(slash, "/") || strings.HasPrefix(slash, "../") || slash == ".." ||
		strings.Contains(slash, "/../") || strings.HasSuffix(slash, "/..") {
		return false
	}
	if len(slash) > 1 && slash[1] == ':' {
		return false
	}
	return true
}

// Within reports whether candidate is the boundary directory or a descendant.
func Within(boundary, candidate string) bool {
	relative, err := filepath.Rel(boundary, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// LogicalManifestPath returns a workspace-relative path for reports.
func LogicalManifestPath(root, manifestPath string) string {
	rel, err := filepath.Rel(root, manifestPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "manifest"
	}
	return filepath.ToSlash(rel)
}
