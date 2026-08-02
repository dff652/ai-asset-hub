package mcp

import (
	"path/filepath"
	"testing"
)

// canonicalTempDir is t.TempDir() with symlinks resolved.
//
// On macOS /var is a symlink to /private/var, so t.TempDir() hands back a path
// containing a symlink while production code canonicalises what it is given --
// deliberately, because resolving is how escaping symlinks are caught. A test
// that builds its expectation from the raw temp path then compares against a
// resolved one fails on macOS and passes on Linux, where /tmp is a real
// directory. Canonicalising here keeps the assertion about behaviour instead of
// about which spelling of the path the platform happened to produce.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	return resolved
}
