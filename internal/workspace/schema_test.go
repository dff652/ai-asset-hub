package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedManifestSchemaMatchesSpec(t *testing.T) {
	specPath := filepath.Join("..", "..", "spec", "manifest.schema.json")
	want, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec schema: %v", err)
	}
	got := ManifestSchemaBytes()
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		t.Fatal("embedded manifest schema diverges from spec/manifest.schema.json")
	}
}
