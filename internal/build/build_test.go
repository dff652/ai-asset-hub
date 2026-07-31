package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestBuildIsDeterministicAndExtractable(t *testing.T) {
	manifest := fixtureManifest(t)
	out1 := t.TempDir()
	out2 := t.TempDir()

	first, err := Build(Options{Manifest: manifest, Profile: "personal", OutDir: out1})
	if err != nil {
		t.Fatalf("build1: %v", err)
	}
	second, err := Build(Options{Manifest: manifest, Profile: "personal", OutDir: out2})
	if err != nil {
		t.Fatalf("build2: %v", err)
	}
	if !first.Ok || !second.Ok {
		t.Fatalf("builds failed: %#v %#v", first, second)
	}
	if first.Package.Digest != second.Package.Digest {
		t.Fatalf("archive digest differs: %s vs %s", first.Package.Digest, second.Package.Digest)
	}

	a1, err := os.ReadFile(filepath.Join(out1, first.Package.Archive))
	if err != nil {
		t.Fatalf("read archive1: %v", err)
	}
	a2, err := os.ReadFile(filepath.Join(out2, second.Package.Archive))
	if err != nil {
		t.Fatalf("read archive2: %v", err)
	}
	if !reflect.DeepEqual(a1, a2) {
		t.Fatal("archives are not byte-identical")
	}
	sum := sha256.Sum256(a1)
	if hex.EncodeToString(sum[:]) != first.Package.Digest {
		t.Fatal("digest does not match archive bytes")
	}

	shaText, err := os.ReadFile(filepath.Join(out1, first.Package.SHA256))
	if err != nil {
		t.Fatalf("read sha: %v", err)
	}
	if !strings.Contains(string(shaText), first.Package.Digest) ||
		!strings.Contains(string(shaText), first.Package.Archive) {
		t.Fatalf("sha file content unexpected: %q", shaText)
	}

	body := readArchiveFile(t, filepath.Join(out1, first.Package.Archive), "assets/skills/shared-review/SKILL.md")
	if !strings.Contains(string(body), "Shared Review") {
		t.Fatalf("skill body = %q", body)
	}

	pkgManifest, err := os.ReadFile(filepath.Join(out1, first.Package.Manifest))
	if err != nil {
		t.Fatalf("read package manifest: %v", err)
	}
	if strings.Contains(string(pkgManifest), filepath.Dir(manifest)) {
		t.Fatal("package manifest leaked absolute workspace path")
	}
	if first.Summary.AssetCount != 2 || first.Summary.FileCount != 2 {
		t.Fatalf("summary = %#v", first.Summary)
	}
	if got := first.Capabilities["claude"]; !reflect.DeepEqual(got, []string{"rules", "skill"}) {
		t.Fatalf("capabilities claude = %#v", got)
	}
}

func TestBuildFailsOnValidationWithoutWriting(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: broken
version: "1"
assets: []
profiles:
  personal:
    include: [missing.id]
`))
	report, err := Build(Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if report.Ok {
		t.Fatal("expected failure")
	}
	assertFinding(t, report, codeValidationFailed)
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".aiah-build-") {
			t.Fatalf("output dir should not contain package artifacts, got %#v", entries)
		}
	}
}

func TestBuildFailsOnSecretWithoutLeaking(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	secret := "sk-build-secret-must-never-appear"
	mustWrite(t, filepath.Join(root, "assets", "rules", "x.md"), []byte("token: "+secret+"\n"))
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: secret-pkg
version: "1"
assets:
  - id: rules.x
    type: rules
    path: assets/rules/x.md
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [rules.x]
`))
	report, err := Build(Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if report.Ok {
		t.Fatal("expected secret failure")
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), secret) {
		t.Fatal("build report leaked secret")
	}
	entries, _ := os.ReadDir(out)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tar") {
			t.Fatalf("artifacts written despite secret: %#v", entries)
		}
	}
}

func TestBuildPullsDependenciesAndRespectsExclude(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	mustWrite(t, filepath.Join(root, "assets", "skills", "base", "SKILL.md"), []byte("# Base\n"))
	mustWrite(t, filepath.Join(root, "assets", "skills", "extra", "SKILL.md"), []byte("# Extra\n"))
	mustWrite(t, filepath.Join(root, "assets", "rules", "main.md"), []byte("# Main\n"))
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: dep-pkg
version: "1"
assets:
  - id: skill.base
    type: skill
    path: assets/skills/base
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
  - id: skill.extra
    type: skill
    path: assets/skills/extra
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
  - id: rules.main
    type: rules
    path: assets/rules/main.md
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
    dependencies: [skill.base]
profiles:
  personal:
    include: [rules.main, skill.extra]
    exclude: [skill.extra]
`))
	report, err := Build(Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !report.Ok {
		t.Fatalf("build failed: %#v", report)
	}
	if report.Summary.AssetCount != 2 {
		t.Fatalf("assetCount = %d, want 2", report.Summary.AssetCount)
	}
	pkgManifest, err := os.ReadFile(filepath.Join(out, report.Package.Manifest))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(pkgManifest)
	if !strings.Contains(text, "skill.base") || !strings.Contains(text, "rules.main") {
		t.Fatalf("package missing expected assets: %s", text)
	}
	if strings.Contains(text, "skill.extra") {
		t.Fatal("excluded asset was packed")
	}
}

func TestBuildRejectsEmptyTargetIntersection(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	mustWrite(t, filepath.Join(root, "assets", "rules", "only-claude.md"), []byte("# Claude only\n"))
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: target-filter
version: "1"
assets:
  - id: rules.claude
    type: rules
    path: assets/rules/only-claude.md
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  codex-only:
    include: [rules.claude]
    targets: [codex]
`))
	report, err := Build(Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Profile:  "codex-only",
		OutDir:   out,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if report.Ok {
		t.Fatal("expected empty targets failure")
	}
	assertFinding(t, report, codeEmptyTargets)
}

func TestBuildUnknownProfile(t *testing.T) {
	manifest := fixtureManifest(t)
	report, err := Build(Options{
		Manifest: manifest,
		Profile:  "missing-profile",
		OutDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if report.Ok {
		t.Fatal("expected unknown profile failure")
	}
	assertFinding(t, report, codeUnknownProfile)
}

func TestBuildDoesNotModifySource(t *testing.T) {
	manifest := fixtureManifest(t)
	skill := filepath.Join(filepath.Dir(manifest), "assets", "skills", "shared-review", "SKILL.md")
	before, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	statBefore, err := os.Stat(skill)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if _, err := Build(Options{
		Manifest: manifest,
		Profile:  "personal",
		OutDir:   t.TempDir(),
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	after, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	statAfter, err := os.Stat(skill)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !reflect.DeepEqual(before, after) || !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Fatal("build modified source workspace")
	}
}

func TestPrepareResolvesTheBuildInputWithoutWriting(t *testing.T) {
	manifest := fixtureManifest(t)
	root := filepath.Dir(manifest)
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	prepared, report, err := Prepare(PrepareOptions{
		Manifest: manifest,
		Root:     root,
		Profile:  "personal",
	})
	if err != nil || !report.Ok {
		t.Fatalf("prepare: err=%v report=%#v", err, report)
	}
	if prepared.Manifest.Name != "fixture-personal" ||
		prepared.Manifest.Profile != "personal" ||
		len(prepared.Manifest.Assets) != 2 ||
		len(prepared.Files) != 2 {
		t.Fatalf("prepared=%#v", prepared)
	}
	after, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("prepare changed workspace entries: before=%#v after=%#v", before, after)
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); !os.IsNotExist(err) {
		t.Fatalf("prepare created dist: %v", err)
	}
}

func TestBuildUsesSingleInspectPassForPackaging(t *testing.T) {
	// Smoke: workspace.Inspect with bodies is what build uses; ensure fixture packs.
	manifest := fixtureManifest(t)
	insp, err := workspace.Inspect(workspace.Options{
		Manifest:      manifest,
		CollectBodies: true,
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if workspace.HasError(insp.Findings) {
		t.Fatalf("inspect findings: %#v", insp.Findings)
	}
	if len(insp.Files) < 2 {
		t.Fatalf("expected packable files, got %d", len(insp.Files))
	}
}

func fixtureManifest(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return path
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
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
	t.Fatalf("finding %q not in %#v", code, report.Findings)
}

func TestBuildPublishIncludesCompletionMarker(t *testing.T) {
	// Verify that build completion is marked atomically by marker file.
	// Absence of marker signals incomplete/interrupted build.
	manifest := fixtureManifest(t)
	out := t.TempDir()

	report, err := Build(Options{Manifest: manifest, Profile: "personal", OutDir: out})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !report.Ok {
		t.Fatalf("build failed: %#v", report)
	}

	// Check marker file exists. Name must track publishArtifacts: the marker is
	// the archive name, dot-prefixed and .complete-suffixed, not a re-derived
	// extension guess.
	markerPath := filepath.Join(out, "."+report.Package.Archive+".complete")
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("marker file missing: %s (%v)", markerPath, err)
	}

	// Verify all artifacts exist.
	for _, name := range []string{report.Package.Archive, report.Package.Manifest, report.Package.Lock, report.Package.SHA256} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("artifact missing: %s (%v)", name, err)
		}
	}
}

// Two profiles of the same package are different packages. Before the profile
// was part of the artifact name, the second build overwrote the first and left
// a .sha256, manifest and lock beside it that still read as one set.
func TestBuildProfilesDoNotOverwriteEachOther(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	mustWrite(t, filepath.Join(root, "assets", "skills", "base", "SKILL.md"), []byte("# Base\n"))
	mustWrite(t, filepath.Join(root, "assets", "rules", "main.md"), []byte("# Main\n"))
	mustWrite(t, filepath.Join(root, "manifest.yaml"), []byte(`
schemaVersion: 1
name: two-profiles
version: "1"
assets:
  - id: skill.base
    type: skill
    path: assets/skills/base
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
  - id: rules.main
    type: rules
    path: assets/rules/main.md
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [skill.base, rules.main]
  minimal:
    include: [skill.base]
`))
	manifest := filepath.Join(root, "manifest.yaml")

	first, err := Build(Options{Manifest: manifest, Profile: "personal", OutDir: out})
	if err != nil || !first.Ok {
		t.Fatalf("build personal: err=%v report=%#v", err, first)
	}
	second, err := Build(Options{Manifest: manifest, Profile: "minimal", OutDir: out})
	if err != nil || !second.Ok {
		t.Fatalf("build minimal: err=%v report=%#v", err, second)
	}

	if first.Package.Archive == second.Package.Archive {
		t.Fatalf("both profiles produced %q", first.Package.Archive)
	}
	for _, name := range []string{
		first.Package.Archive, first.Package.Manifest, first.Package.Lock, first.Package.SHA256,
		second.Package.Archive, second.Package.Manifest, second.Package.Lock, second.Package.SHA256,
	} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("artifact missing after the other profile built: %s (%v)", name, err)
		}
	}

	// The surviving personal artifacts must still describe the personal build:
	// two assets, not the one minimal packs.
	raw, err := os.ReadFile(filepath.Join(out, first.Package.Lock))
	if err != nil {
		t.Fatalf("read personal lock: %v", err)
	}
	var lock LockFile
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("decode personal lock: %v", err)
	}
	if lock.Profile != "personal" || len(lock.Files) != 2 {
		t.Fatalf("personal lock = profile %q with %d files", lock.Profile, len(lock.Files))
	}
}
