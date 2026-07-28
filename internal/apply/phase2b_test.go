package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/version"
)

func TestPhase2BApplyHomeProjectAndGrok(t *testing.T) {
	t.Setenv("EXAMPLE_SERVICE_TOKEN", "resolved-example-service-token")
	workspace := filepath.Join("..", "..", "testdata", "workspace-2b")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: filepath.Join(workspace, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: err=%v report=%#v", err, buildReport)
	}
	pkg := filepath.Join(pkgDir, buildReport.Package.Archive)
	home := t.TempDir()
	project := t.TempDir()

	report, err := Apply(Options{
		Package: pkg,
		Home:    home,
		Project: project,
		Targets: []string{"claude", "codex", "grok"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !report.Ok {
		t.Fatalf("apply failed: %#v", report)
	}

	checks := []string{
		filepath.Join(home, ".claude", "skills", "review", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "review", "SKILL.md"),
		filepath.Join(home, ".grok", "skills", "review", "SKILL.md"),
		filepath.Join(home, ".agents", "skills", "review", "SKILL.md"),
		filepath.Join(home, ".claude", "agents", "reviewer.md"),
		filepath.Join(home, ".claude", "hooks", "pre-tool.sh"),
		filepath.Join(home, ".codex", "hooks", "pre-tool.sh"),
		filepath.Join(home, ".claude", "mcp", "example.json"),
		filepath.Join(home, ".codex", "mcp", "example.json"),
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".codex", "config.toml"),
		filepath.Join(project, "CLAUDE.md"),
	}
	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}

	// Project CLAUDE.md should not also be under home/.claude when scope is project.
	if _, err := os.Stat(filepath.Join(home, ".claude", "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("project CLAUDE.md incorrectly installed under home/.claude")
	}

	settings, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.Contains(string(settings), "example-mcp") || !strings.Contains(string(settings), "mcpServers") {
		t.Fatalf(".claude.json missing native mcp: %s", settings)
	}
	codexCfg, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(codexCfg), "example-mcp") || !strings.Contains(string(codexCfg), "mcp_servers") {
		t.Fatalf("config.toml missing native MCP entry: %s", codexCfg)
	}
	// Native config receives the resolved value; the package and sidecar retain
	// only the portable reference.
	if !strings.Contains(string(settings), "resolved-example-service-token") ||
		strings.Contains(string(settings), "${ENV:EXAMPLE_SERVICE_TOKEN}") {
		t.Fatalf("settings did not resolve secret ref: %s", settings)
	}
	if strings.Contains(string(settings), "sk-") {
		t.Fatal("settings leaked sk- secret shape")
	}

	// Script hooks must be executable after apply.
	for _, hook := range []string{
		filepath.Join(home, ".claude", "hooks", "pre-tool.sh"),
		filepath.Join(home, ".codex", "hooks", "pre-tool.sh"),
	} {
		info, err := os.Stat(hook)
		if err != nil {
			t.Fatalf("stat hook %s: %v", hook, err)
		}
		if info.Mode().Perm()&0111 == 0 {
			t.Fatalf("hook not executable: %s mode=%o", hook, info.Mode().Perm())
		}
	}

	// Project-only without home should fail global assets.
	bad, err := Apply(Options{
		Package: pkg,
		Project: t.TempDir(),
		Targets: []string{"claude"},
	})
	if err != nil {
		t.Fatalf("project-only apply err: %v", err)
	}
	if bad.Ok {
		t.Fatal("expected failure without home for global assets")
	}
	if !hasFindingCode(bad.Findings, codeMissingHomeRoot) ||
		hasFindingCode(bad.Findings, codeMCPNativeFailed) {
		t.Fatalf("missing root used the wrong finding boundary: %#v", bad.Findings)
	}
}

func TestPhase2BApplyDoesNotRewriteOrBackupExistingNativeConfig(t *testing.T) {
	t.Setenv("EXAMPLE_SERVICE_TOKEN", "resolved-example-service-token")
	workspace := filepath.Join("..", "..", "testdata", "workspace-2b")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: filepath.Join(workspace, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: err=%v report=%#v", err, buildReport)
	}
	pkg := filepath.Join(pkgDir, buildReport.Package.Archive)
	home := t.TempDir()
	nativePath := filepath.Join(home, ".codex", "config.toml")
	original := []byte("# keep exact bytes\napi_token = \"existing-user-secret\"\n")
	mustWrite(t, nativePath, original)

	report, err := Apply(Options{
		Package: pkg,
		Home:    home,
		Targets: []string{"codex"},
	})
	if err != nil || !report.Ok {
		t.Fatalf("apply: err=%v report=%#v", err, report)
	}
	if !hasFindingCode(report.Findings, codeMCPNativeSkipped) {
		t.Fatalf("missing native skip finding: %#v", report.Findings)
	}
	after, err := os.ReadFile(nativePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("native config changed:\n%s", after)
	}
	backupPath := filepath.Join(
		home, ".aiah", "backups", report.BackupID,
		"home", ".codex", "config.toml",
	)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("existing native config entered backup: %v", err)
	}
}

func TestProjectMCPPackageBootstrapAndConflict(t *testing.T) {
	workspace := filepath.Join("..", "..", "testdata", "workspace-mcp-project")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: filepath.Join(workspace, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: err=%v report=%#v", err, buildReport)
	}
	pkg := filepath.Join(pkgDir, buildReport.Package.Archive)

	t.Run("bootstrap", func(t *testing.T) {
		project := t.TempDir()
		report, err := Apply(Options{
			Package: pkg,
			Project: project,
			Targets: []string{"claude"},
		})
		if err != nil || !report.Ok {
			t.Fatalf("apply: err=%v report=%#v", err, report)
		}
		for _, rel := range []string{".mcp.json", ".claude/mcp/project-example.json"} {
			if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(rel))); err != nil {
				t.Fatalf("missing project MCP output %s: %v", rel, err)
			}
		}
	})

	t.Run("conflict fails whole apply", func(t *testing.T) {
		project := t.TempDir()
		nativePath := filepath.Join(project, ".mcp.json")
		original := []byte(`{"mcpServers":{"project-example":{"command":"other-command"}}}`)
		mustWrite(t, nativePath, original)

		report, err := Apply(Options{
			Package: pkg,
			Project: project,
			Targets: []string{"claude"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if report.Ok || !hasFindingCode(report.Findings, codeMCPNativeFailed) {
			t.Fatalf("conflict did not fail closed: %#v", report)
		}
		if _, err := os.Stat(filepath.Join(project, ".claude", "mcp", "project-example.json")); !os.IsNotExist(err) {
			t.Fatalf("sidecar written despite whole-apply failure: %v", err)
		}
		after, err := os.ReadFile(nativePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(original) {
			t.Fatal("conflicting native config changed")
		}
	})

	t.Run("missing project root uses plan finding", func(t *testing.T) {
		report, err := Apply(Options{
			Package: pkg,
			Home:    t.TempDir(),
			Targets: []string{"claude"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if report.Ok || !hasFindingCode(report.Findings, codeMissingProjectRoot) ||
			hasFindingCode(report.Findings, codeMCPNativeFailed) {
			t.Fatalf("missing project root used the wrong finding boundary: %#v", report)
		}
	})
}

// A deployment record that cannot name the binary that wrote it cannot be
// traced back to the adapter and classification rules in force at the time.
func TestDeployRecordNamesTheBinary(t *testing.T) {
	t.Setenv("EXAMPLE_SERVICE_TOKEN", "resolved-example-service-token")
	workspace := filepath.Join("..", "..", "testdata", "workspace-2b")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: filepath.Join(workspace, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: err=%v report=%#v", err, buildReport)
	}
	home := t.TempDir()
	project := t.TempDir()
	report, err := Apply(Options{
		Package: filepath.Join(pkgDir, buildReport.Package.Archive),
		Home:    home,
		Project: project,
		Targets: []string{"claude"},
	})
	if err != nil || !report.Ok {
		t.Fatalf("apply: err=%v report=%#v", err, report)
	}
	if report.ProducedBy != version.ProducedBy() {
		t.Fatalf("apply report producedBy = %q, want %q", report.ProducedBy, version.ProducedBy())
	}

	// Read it the way rollback does, so the on-disk layout stays owned by one
	// place rather than being pinned by a test about producedBy.
	deploy, err := readCurrentDeploy(home)
	if err != nil {
		t.Fatalf("read deploy record: %v", err)
	}
	if deploy.ProducedBy != version.ProducedBy() {
		t.Fatalf("deploy record producedBy = %q, want %q", deploy.ProducedBy, version.ProducedBy())
	}
	if len(deploy.FileStates) == 0 {
		t.Fatal("deploy record has no file hashes or modes for doctor")
	}
	doctor, err := Doctor(DoctorOptions{Home: home, Project: project})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if !doctor.Ok || doctor.Summary.Unchanged != len(deploy.FileStates) {
		t.Fatalf("doctor did not verify fresh deployment: %#v", doctor)
	}
}
