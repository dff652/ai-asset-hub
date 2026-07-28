package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareRootRequiresAnExplicitDirectory(t *testing.T) {
	home := t.TempDir()
	root, created, err := PrepareRoot("~/ai-assets", home, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "ai-assets")
	if root != want || !created {
		t.Fatalf("PrepareRoot = (%q, %v), want (%q, true)", root, created, want)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("workspace directory was not created: info=%v err=%v", info, err)
	}

	again, created, err := PrepareRoot(root, home, "")
	if err != nil || again != root || created {
		t.Fatalf("existing root = (%q, %v, %v), want (%q, false, nil)", again, created, err, root)
	}
	homeRoot, created, err := PrepareRoot("~", home, "")
	if err != nil || homeRoot != home || created {
		t.Fatalf("home root = (%q, %v, %v), want (%q, false, nil)", homeRoot, created, err, home)
	}
}

func TestPrepareRootRejectsEmptyAndFiles(t *testing.T) {
	home := t.TempDir()
	if _, _, err := PrepareRoot("", home, ""); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("empty path error = %v, want ErrInvalidOptions", err)
	}
	file := filepath.Join(home, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareRoot(file, home, ""); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("file path error = %v, want ErrInvalidOptions", err)
	}
}

func TestPrepareRootRejectsManagedToolDirectories(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	for _, boundary := range []string{home, project} {
		for _, name := range managedToolDirectories {
			candidate := filepath.Join(boundary, name, "workspace")
			if _, _, err := PrepareRoot(candidate, home, project); !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("%s under %s error = %v, want ErrInvalidOptions", name, boundary, err)
			}
			if _, err := os.Stat(filepath.Join(boundary, name)); !os.IsNotExist(err) {
				t.Fatalf("rejected managed directory %s was created: %v", filepath.Join(boundary, name), err)
			}
		}
	}

	sibling := filepath.Join(home, ".claude-workspace")
	root, created, err := PrepareRoot(sibling, home, project)
	if err != nil || root != sibling || !created {
		t.Fatalf("safe sibling = (%q, %v, %v), want (%q, true, nil)", root, created, err, sibling)
	}
}

func TestPrepareRootRejectsSymlinkIntoManagedToolDirectory(t *testing.T) {
	home := t.TempDir()
	managed := filepath.Join(home, ".claude")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(managed, link); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(link, "nested")
	if _, _, err := PrepareRoot(candidate, home, ""); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("symlinked managed path error = %v, want ErrInvalidOptions", err)
	}
	if _, err := os.Stat(filepath.Join(managed, "nested")); !os.IsNotExist(err) {
		t.Fatalf("rejected symlink path created a directory: %v", err)
	}
}
