package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareRootRequiresAnExplicitDirectory(t *testing.T) {
	home := t.TempDir()
	root, created, err := PrepareRoot("~/ai-assets", home)
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

	again, created, err := PrepareRoot(root, home)
	if err != nil || again != root || created {
		t.Fatalf("existing root = (%q, %v, %v), want (%q, false, nil)", again, created, err, root)
	}
	homeRoot, created, err := PrepareRoot("~", home)
	if err != nil || homeRoot != home || created {
		t.Fatalf("home root = (%q, %v, %v), want (%q, false, nil)", homeRoot, created, err, home)
	}
}

func TestPrepareRootRejectsEmptyAndFiles(t *testing.T) {
	home := t.TempDir()
	if _, _, err := PrepareRoot("", home); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("empty path error = %v, want ErrInvalidOptions", err)
	}
	file := filepath.Join(home, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareRoot(file, home); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("file path error = %v, want ErrInvalidOptions", err)
	}
}
