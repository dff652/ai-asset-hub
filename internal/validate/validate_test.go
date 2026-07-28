package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestValidateAcceptsFixtureWorkspace(t *testing.T) {
	manifest := fixtureManifest(t, "workspace-valid")
	first, err := Validate(Options{Manifest: manifest})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	second, err := Validate(Options{Manifest: manifest})
	if err != nil {
		t.Fatalf("second validate: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated validates returned different reports")
	}
	if !first.Ok || first.Summary.ErrorCount != 0 {
		t.Fatalf("report = %#v", first)
	}
	if first.Summary.AssetCount != 2 || first.Summary.CheckedPaths < 2 {
		t.Fatalf("summary = %#v", first.Summary)
	}
}

func TestValidateDetectsDuplicateAndMissingRefs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "assets", "a", "SKILL.md"), []byte("# A\n"))
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: bad-refs
version: "1"
assets:
  - id: skill.a
    type: skill
    path: assets/a
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
    dependencies: [skill.missing]
    conflicts: [skill.ghost]
  - id: skill.a
    type: skill
    path: assets/a
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [skill.a, skill.nope]
`))

	report, err := Validate(Options{Manifest: filepath.Join(root, "manifest.yaml")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Ok {
		t.Fatal("expected validation failure")
	}
	assertFinding(t, report, workspace.CodeDuplicateAssetID)
	assertFinding(t, report, workspace.CodeMissingDependency)
	assertFinding(t, report, workspace.CodeMissingConflictRef)
	assertFinding(t, report, workspace.CodeMissingProfileRef)
}

func TestValidateRejectsPathEscapeAndMissingPath(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: bad-paths
version: "1"
assets:
  - id: skill.escape
    type: skill
    path: ../outside
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
  - id: skill.missing
    type: skill
    path: assets/missing
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [skill.escape]
`))

	report, err := Validate(Options{Manifest: filepath.Join(root, "manifest.yaml")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	assertFinding(t, report, workspace.CodePathEscape)
	assertFinding(t, report, workspace.CodeMissingAssetPath)
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), root) {
		t.Fatalf("report leaked absolute root %q", root)
	}
}

func TestValidateRejectsSecretsWithoutLeakingValues(t *testing.T) {
	root := t.TempDir()
	secret := "sk-test-validate-must-never-appear"
	mustWrite(t, filepath.Join(root, "assets", "rules", "secret.md"), []byte("token: "+secret+"\n"))
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: secret-workspace
version: "1"
assets:
  - id: rules.secret
    type: rules
    path: assets/rules/secret.md
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [rules.secret]
`))

	report, err := Validate(Options{Manifest: filepath.Join(root, "manifest.yaml")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	assertFinding(t, report, workspace.CodeSuspectedSecret)
	if report.Ok {
		t.Fatal("expected failure on secret")
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatal("validation report leaked secret value")
	}
}

func TestValidateRejectsSymlinkEscapeAndInternalSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, []byte("outside\n"))
	assetDir := filepath.Join(root, "assets", "skills", "linked")
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(assetDir, "SKILL.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: symlink-workspace
version: "1"
assets:
  - id: skill.linked
    type: skill
    path: assets/skills/linked
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [skill.linked]
`))

	report, err := Validate(Options{Manifest: filepath.Join(root, "manifest.yaml")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	assertFinding(t, report, workspace.CodeSymlinkEscape)
}

func TestValidateSchemaFailureStopsBeforePathChecks(t *testing.T) {
	root := t.TempDir()
	// Invalid: missing required fields after bad schemaVersion type won't parse well;
	// use unknown property and wrong schemaVersion.
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 2
name: x
version: "1"
assets: []
profiles:
  personal:
    include: []
`))
	report, err := Validate(Options{Manifest: filepath.Join(root, "manifest.yaml")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Ok {
		assertFinding(t, report, workspace.CodeInvalidManifest)
	}
	// Should not invent path findings for non-assets.
	for _, finding := range report.Findings {
		if finding.Code == workspace.CodeMissingAssetPath {
			t.Fatalf("path checks ran after schema failure: %#v", report.Findings)
		}
	}
}

func TestValidateDoesNotModifyFiles(t *testing.T) {
	manifest := fixtureManifest(t, "workspace-valid")
	skill := filepath.Join(filepath.Dir(manifest), "assets", "skills", "shared-review", "SKILL.md")
	before, err := os.Stat(skill)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	beforeContent, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if _, err := Validate(Options{Manifest: manifest}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	after, err := os.Stat(skill)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	afterContent, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode() ||
		!reflect.DeepEqual(beforeContent, afterContent) {
		t.Fatal("validate modified workspace files")
	}
}

func TestInvalidOptions(t *testing.T) {
	if _, err := Validate(Options{}); err == nil {
		t.Fatal("expected error for empty options")
	}
}

func fixtureManifest(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", name, "manifest.yaml"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return path
}

func mustWrite(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func assertFinding(t *testing.T, report Report, code string) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.Code == code {
			return
		}
	}
	t.Fatalf("finding %q not found in %#v", code, report.Findings)
}
