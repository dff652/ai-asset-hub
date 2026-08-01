package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestDecodesOptionalProvenanceAndFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	body := []byte(`schemaVersion: 1
name: provenance-test
version: "1"
assets:
  - id: skill.review
    type: skill
    path: assets/skills/review
    targets: [claude, codex]
    scope: global
    portability: portable
    sensitivity: public
    source:
      url: https://example.com/review.git
      revision: abc123
      importedAt: 2026-08-01T00:00:00Z
    files:
      - path: SKILL.md
        sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
profiles:
  personal:
    include: [skill.review]
`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	document, normalized, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if err := ValidateManifestSchema(normalized); err != nil {
		t.Fatalf("validate manifest: %v", err)
	}
	if len(document.Assets) != 1 {
		t.Fatalf("assets = %#v", document.Assets)
	}
	asset := document.Assets[0]
	if asset.Source == nil || asset.Source.URL != "https://example.com/review.git" ||
		asset.Source.Revision != "abc123" || asset.Source.ImportedAt != "2026-08-01T00:00:00Z" {
		t.Fatalf("source = %#v", asset.Source)
	}
	if len(asset.Files) != 1 || asset.Files[0].Path != "SKILL.md" ||
		asset.Files[0].SHA256 != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("files = %#v", asset.Files)
	}
}
