package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
)

func TestApplyDiffInstallIdempotentAndRollback(t *testing.T) {
	workspace := filepath.Join("..", "..", "testdata", "workspace-valid")
	manifest := filepath.Join(workspace, "manifest.yaml")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: manifest,
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: err=%v report=%#v", err, buildReport)
	}
	pkg := filepath.Join(pkgDir, buildReport.Package.Archive)
	home := t.TempDir()

	diff, err := Diff(Options{Package: pkg, Home: home, Targets: []string{"claude", "codex"}})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !diff.Ok || diff.Summary.Create == 0 {
		t.Fatalf("diff report = %#v", diff)
	}
	// dry-run must not create tool dirs
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatal("diff wrote files")
	}

	first, err := Apply(Options{Package: pkg, Home: home, Targets: []string{"claude", "codex"}})
	if err != nil {
		t.Fatalf("apply1: %v", err)
	}
	if !first.Ok || first.Summary.Written == 0 || first.BackupID == "" {
		t.Fatalf("apply1 = %#v", first)
	}

	skill := filepath.Join(home, ".claude", "skills", "shared-review", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("empty skill")
	}
	// shared T0 root
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "shared-review", "SKILL.md")); err != nil {
		t.Fatalf("shared skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "shared-review", "SKILL.md")); err != nil {
		t.Fatalf("codex skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "rules", "common.md")); err != nil {
		t.Fatalf("claude rules missing: %v", err)
	}

	// idempotent second apply
	second, err := Apply(Options{Package: pkg, Home: home, Targets: []string{"claude", "codex"}})
	if err != nil {
		t.Fatalf("apply2: %v", err)
	}
	if !second.Ok || second.Summary.Written != 0 || second.Summary.Unchanged == 0 {
		t.Fatalf("apply2 not idempotent: %#v", second)
	}

	// mutate then rollback
	if err := os.WriteFile(skill, []byte("mutated\n"), 0644); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	// Re-apply to create a backup of mutated? Actually rollback restores pre-first-apply
	// for files that existed before first apply. First apply created skill (no backup content).
	// So rollback should REMOVE created files.
	rb, err := Rollback(RollbackOptions{Home: home, BackupID: first.BackupID})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !rb.Ok {
		t.Fatalf("rollback = %#v", rb)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatalf("skill should be removed after rollback of create: %v", err)
	}
}

func TestApplyUpdatesExistingWithBackupRestore(t *testing.T) {
	workspace := filepath.Join("..", "..", "testdata", "workspace-valid")
	manifest := filepath.Join(workspace, "manifest.yaml")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: manifest,
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: %#v %v", buildReport, err)
	}
	pkg := filepath.Join(pkgDir, buildReport.Package.Archive)
	home := t.TempDir()

	// Pre-create a claude skill with different content.
	pre := filepath.Join(home, ".claude", "skills", "shared-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(pre), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(pre, []byte("old content\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	report, err := Apply(Options{Package: pkg, Home: home, Targets: []string{"claude"}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !report.Ok || report.Summary.Update == 0 {
		t.Fatalf("apply = %#v", report)
	}
	backup := filepath.Join(
		home, ".aiah", "backups", report.BackupID,
		"home", ".claude", "skills", "shared-review", "SKILL.md",
	)
	backupInfo, err := os.Stat(backup)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if backupInfo.Mode().Perm() != 0600 {
		t.Fatalf("backup mode = %o, want 600", backupInfo.Mode().Perm())
	}
	body, _ := os.ReadFile(pre)
	if string(body) == "old content\n" {
		t.Fatal("file was not updated")
	}

	rb, err := Rollback(RollbackOptions{Home: home})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !rb.Ok {
		t.Fatalf("rollback = %#v", rb)
	}
	restored, err := os.ReadFile(pre)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if string(restored) != "old content\n" {
		t.Fatalf("restored = %q", restored)
	}
	info, err := os.Stat(pre)
	if err != nil {
		t.Fatalf("stat restored: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("restored mode = %o, want 600", info.Mode().Perm())
	}
}

func TestApplyRejectsTargetOutsidePackage(t *testing.T) {
	workspace := filepath.Join("..", "..", "testdata", "workspace-valid")
	pkgDir := t.TempDir()
	buildReport, err := build.Build(build.Options{
		Manifest: filepath.Join(workspace, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !buildReport.Ok {
		t.Fatalf("build: %#v %v", buildReport, err)
	}
	report, err := Diff(Options{
		Package: filepath.Join(pkgDir, buildReport.Package.Archive),
		Home:    t.TempDir(),
		Targets: []string{"future"},
	})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if report.Ok || !hasFindingCode(report.Findings, codeInvalidTarget) {
		t.Fatalf("report = %#v", report)
	}
}
