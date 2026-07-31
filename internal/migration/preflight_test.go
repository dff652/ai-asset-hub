package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
)

func TestInspectPreflightReportsBlockersWarningsAndDevicePrivateWithoutWriting(t *testing.T) {
	const missingSecret = "AIAH_PREFLIGHT_TEST_MISSING"
	if err := os.Unsetenv(missingSecret); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	writePreflightFile(t, filepath.Join(root, "assets", "skills", "review", "SKILL.md"), "# Review\n")
	writePreflightFile(t, filepath.Join(root, "assets", "rules", "CLAUDE.md"), "# Claude rules\n")
	writePreflightFile(t, filepath.Join(root, "assets", "memory", "notes.md"), "# Memory\n")
	writePreflightFile(t, filepath.Join(root, "assets", "mcp", "example.json"),
		`{"name":"example","command":"example-mcp","env":{"TOKEN":"${ENV:`+missingSecret+`}"}}`)
	writePreflightFile(t, filepath.Join(root, "manifest.yaml"), `
schemaVersion: 1
name: preflight-fixture
version: "1"
assets:
  - id: skill.review
    type: skill
    path: assets/skills/review
    targets: [claude, codex]
    scope: global
    portability: portable
    sensitivity: private
  - id: rules.claude
    type: rules
    path: assets/rules/CLAUDE.md
    targets: [codex]
    scope: global
    portability: portable
    sensitivity: private
  - id: memory.notes
    type: memory
    path: assets/memory/notes.md
    targets: [codex]
    scope: global
    portability: adapter-required
    sensitivity: private
  - id: mcp.example
    type: mcp
    path: assets/mcp/example.json
    targets: [claude]
    scope: global
    portability: adapter-required
    sensitivity: sensitive
  - id: skill.future
    type: skill
    path: assets/skills/review
    targets: [future]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [skill.review, rules.claude, memory.notes, mcp.example, skill.future]
`)
	home := t.TempDir()
	writePreflightFile(t, filepath.Join(home, ".codex", "auth.json"), `{"token":"local-only"}`)
	before := treeDigest(t, root, home)

	report, err := InspectPreflight(PreflightOptions{
		WorkspaceRoot: root,
		Profile:       "personal",
		Home:          home,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Ok ||
		report.Summary.UnsupportedTargets != 1 ||
		report.Summary.DroppedItems != 1 ||
		report.Summary.DegradedItems != 1 ||
		report.Summary.MissingSecrets != 1 ||
		report.Summary.DevicePrivateItems != 1 {
		t.Fatalf("summary=%#v ok=%v", report.Summary, report.Ok)
	}
	for _, code := range []string{
		codePreflightUnsupportedTarget,
		codePreflightAdapterDropped,
		codePreflightAdapterDegraded,
		codePreflightMissingSecret,
	} {
		if !preflightHasFinding(report, code) {
			t.Fatalf("missing finding %q in %#v", code, report.Findings)
		}
	}
	if len(report.Secrets) != 1 ||
		report.Secrets[0].Name != missingSecret ||
		report.Secrets[0].Available {
		t.Fatalf("secrets=%#v", report.Secrets)
	}
	if len(report.DevicePrivate) != 1 ||
		report.DevicePrivate[0].LogicalPath != "home/.codex/auth.json" {
		t.Fatalf("device private=%#v", report.DevicePrivate)
	}
	if after := treeDigest(t, root, home); after != before {
		t.Fatalf("preflight changed files:\nbefore=%s\nafter=%s", before, after)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "local-only") {
		t.Fatalf("preflight leaked device-private content: %s", encoded)
	}
}

func TestInspectPreflightAllowsIntentionalDevicePrivateExclusions(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "workspace-valid"))); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	writePreflightFile(t, filepath.Join(home, ".codex", "auth.json"), `{"token":"local-only"}`)

	report, err := InspectPreflight(PreflightOptions{
		WorkspaceRoot: root,
		Profile:       "personal",
		Home:          home,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ok || report.Summary.DevicePrivateItems != 1 ||
		report.Summary.UnsupportedTargets != 0 ||
		report.Summary.DroppedItems != 0 ||
		report.Summary.MissingSecrets != 0 {
		t.Fatalf("report=%#v", report)
	}
}

func TestInspectPackagePreflightBindsTheSelectedReleaseWithoutWriting(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "workspace-valid"))); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	built, err := build.Build(build.Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Root:     root,
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil || !built.Ok || built.Package == nil {
		t.Fatalf("build: err=%v report=%#v", err, built)
	}
	packagePath := filepath.Join(out, built.Package.Archive)
	home := t.TempDir()
	writePreflightFile(t, filepath.Join(home, ".codex", "auth.json"), `{"token":"local-only"}`)
	before := treeDigest(t, out, home)

	report, err := InspectPackagePreflight(PackagePreflightOptions{
		Package: packagePath,
		Home:    home,
		Expected: ReleaseIdentity{
			Name: built.Package.Name, Version: built.Package.Version,
			Profile: "personal", SHA256: built.Package.Digest,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ok ||
		report.Subject.Source != "package" ||
		report.Subject.Name != built.Package.Name ||
		report.Subject.Version != built.Package.Version ||
		report.Subject.Profile != "personal" ||
		report.Subject.Package != filepath.Base(packagePath) ||
		report.Subject.SHA256 != built.Package.Digest {
		t.Fatalf("report=%#v", report)
	}
	if after := treeDigest(t, out, home); after != before {
		t.Fatalf("package preflight changed files:\nbefore=%s\nafter=%s", before, after)
	}

	mismatch, err := InspectPackagePreflight(PackagePreflightOptions{
		Package: packagePath,
		Home:    home,
		Expected: ReleaseIdentity{
			Name: built.Package.Name, Version: built.Package.Version,
			Profile: "personal", SHA256: strings.Repeat("0", 64),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.Ok || !preflightHasFinding(mismatch, codePreflightReleaseMismatch) {
		t.Fatalf("a package outside the selected digest was accepted: %#v", mismatch)
	}
}

func writePreflightFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func preflightHasFinding(report PreflightReport, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
