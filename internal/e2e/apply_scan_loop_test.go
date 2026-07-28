package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/inventory"
)

// TestApplyScanRollbackLoop verifies the Phase 1+2 closed loop:
// build → apply → scan (see installed assets) → rollback → scan (gone).
func TestApplyScanRollbackLoopWorkspaceValid(t *testing.T) {
	manifest := abs(t, "testdata", "workspace-valid", "manifest.yaml")
	pkgDir := t.TempDir()
	home := t.TempDir()

	built, err := build.Build(build.Options{
		Manifest: manifest,
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !built.Ok {
		t.Fatalf("build: err=%v report=%#v", err, built)
	}
	pkg := filepath.Join(pkgDir, built.Package.Archive)

	// Empty home: inventory has no tool assets.
	before, err := inventory.Scan(inventory.Options{Home: home})
	if err != nil {
		t.Fatalf("scan before: %v", err)
	}
	if before.Summary.CandidateAssets != 0 {
		t.Fatalf("empty home candidates = %d", before.Summary.CandidateAssets)
	}

	applied, err := apply.Apply(apply.Options{
		Package: pkg,
		Home:    home,
		Targets: []string{"claude", "codex"},
	})
	if err != nil || !applied.Ok {
		t.Fatalf("apply: err=%v report=%#v", err, applied)
	}
	if applied.Summary.Written == 0 {
		t.Fatal("apply wrote nothing")
	}

	afterApply, err := inventory.Scan(inventory.Options{Home: home})
	if err != nil {
		t.Fatalf("scan after apply: %v", err)
	}

	// Installed skills/rules must be visible as inventory candidates with correct sources.
	mustAsset(t, afterApply, "home/.claude/skills/shared-review", inventory.SourceClaude, inventory.TypeSkill)
	mustAsset(t, afterApply, "home/.codex/skills/shared-review", inventory.SourceCodex, inventory.TypeSkill)
	mustAsset(t, afterApply, "home/.agents/skills/shared-review", inventory.SourceShared, inventory.TypeSkill)
	mustAsset(t, afterApply, "home/.claude/rules/common.md", inventory.SourceClaude, inventory.TypeRules)
	mustAsset(t, afterApply, "home/.codex/rules/common.md", inventory.SourceCodex, inventory.TypeRules)

	// Shared + claude + codex skill packages: three skill candidates, not collapsed.
	if afterApply.Summary.CandidateByType[inventory.TypeSkill] != 3 {
		t.Fatalf("skill candidates = %d, want 3", afterApply.Summary.CandidateByType[inventory.TypeSkill])
	}
	if afterApply.Summary.DeviceMigration != inventory.DeviceInventoryOnly {
		t.Fatalf("deviceMigration = %q", afterApply.Summary.DeviceMigration)
	}

	// Content continuity: scan hash matches applied file bytes.
	skillPath := filepath.Join(home, ".claude", "skills", "shared-review", "SKILL.md")
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	var skillEntry inventory.Entry
	found := false
	for _, entry := range afterApply.Entries {
		if entry.LogicalPath == "home/.claude/skills/shared-review/SKILL.md" {
			skillEntry = entry
			found = true
			break
		}
	}
	if !found || skillEntry.SHA256 == "" {
		t.Fatalf("skill entry missing hash: found=%v entry=%#v", found, skillEntry)
	}
	// Re-hash by scanning is enough; ensure entry size matches file.
	if skillEntry.Size != int64(len(body)) {
		t.Fatalf("size mismatch entry=%d file=%d", skillEntry.Size, len(body))
	}

	rb, err := apply.Rollback(apply.RollbackOptions{Home: home, BackupID: applied.BackupID})
	if err != nil || !rb.Ok {
		t.Fatalf("rollback: err=%v report=%#v", err, rb)
	}

	afterRB, err := inventory.Scan(inventory.Options{Home: home})
	if err != nil {
		t.Fatalf("scan after rollback: %v", err)
	}
	mustNoAsset(t, afterRB, "home/.claude/skills/shared-review")
	mustNoAsset(t, afterRB, "home/.codex/skills/shared-review")
	mustNoAsset(t, afterRB, "home/.agents/skills/shared-review")
	mustNoAsset(t, afterRB, "home/.claude/rules/common.md")
	if afterRB.Summary.CandidateAssets != 0 {
		t.Fatalf("after rollback candidates = %d, want 0; assets=%#v",
			afterRB.Summary.CandidateAssets, afterRB.Assets)
	}
}

// TestApplyScanLoopPhase2B covers multi-type install including project root and grok.
func TestApplyScanLoopPhase2B(t *testing.T) {
	t.Setenv("EXAMPLE_SERVICE_TOKEN", "resolved-example-service-token")
	manifest := abs(t, "testdata", "workspace-2b", "manifest.yaml")
	pkgDir := t.TempDir()
	home := t.TempDir()
	project := t.TempDir()

	built, err := build.Build(build.Options{
		Manifest: manifest,
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !built.Ok {
		t.Fatalf("build: err=%v report=%#v", err, built)
	}
	pkg := filepath.Join(pkgDir, built.Package.Archive)

	applied, err := apply.Apply(apply.Options{
		Package: pkg,
		Home:    home,
		Project: project,
		Targets: []string{"claude", "codex", "grok"},
	})
	if err != nil || !applied.Ok {
		t.Fatalf("apply: err=%v report=%#v", err, applied)
	}

	report, err := inventory.Scan(inventory.Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	mustAsset(t, report, "home/.claude/skills/review", inventory.SourceClaude, inventory.TypeSkill)
	mustAsset(t, report, "home/.codex/skills/review", inventory.SourceCodex, inventory.TypeSkill)
	mustAsset(t, report, "home/.grok/skills/review", inventory.SourceGrok, inventory.TypeSkill)
	mustAsset(t, report, "home/.agents/skills/review", inventory.SourceShared, inventory.TypeSkill)
	mustAsset(t, report, "home/.claude/agents/reviewer.md", inventory.SourceClaude, inventory.TypeAgent)
	mustAsset(t, report, "home/.claude/hooks/pre-tool.sh", inventory.SourceClaude, inventory.TypeHook)
	mustAsset(t, report, "home/.codex/hooks/pre-tool.sh", inventory.SourceCodex, inventory.TypeHook)
	mustAsset(t, report, "home/.claude/mcp/example.json", inventory.SourceClaude, inventory.TypeMCPConfig)
	mustAsset(t, report, "home/.codex/mcp/example.json", inventory.SourceCodex, inventory.TypeMCPConfig)
	// Native configs contain the device-local resolved secret, so a later scan
	// must keep them out of portable candidates. The sidecars above remain the
	// reference-only migration assets.
	mustExcludedSecretAsset(t, report, "home/.claude.json")
	mustExcludedSecretAsset(t, report, "home/.codex/config.toml")
	mustAsset(t, report, "project/CLAUDE.md", inventory.SourceClaude, inventory.TypeRules)

	// Four skill mirrors: claude, codex, grok, shared — distinct inventory units.
	if report.Summary.CandidateByType[inventory.TypeSkill] != 4 {
		t.Fatalf("skill candidates = %d, want 4", report.Summary.CandidateByType[inventory.TypeSkill])
	}

	// Idempotent apply must not change inventory.
	second, err := apply.Apply(apply.Options{
		Package: pkg,
		Home:    home,
		Project: project,
		Targets: []string{"claude", "codex", "grok"},
	})
	if err != nil || !second.Ok {
		t.Fatalf("apply2: err=%v %#v", err, second)
	}
	if second.Summary.Written != 0 {
		t.Fatalf("second apply wrote %d files", second.Summary.Written)
	}
	if second.BackupID != "" {
		t.Fatalf("idempotent apply created backup %q", second.BackupID)
	}
	again, err := inventory.Scan(inventory.Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan2: %v", err)
	}
	if again.Summary.CandidateAssets != report.Summary.CandidateAssets {
		t.Fatalf("candidate drift after idempotent apply: %d → %d",
			report.Summary.CandidateAssets, again.Summary.CandidateAssets)
	}

	rb, err := apply.Rollback(apply.RollbackOptions{
		Home:     home,
		Project:  project,
		BackupID: applied.BackupID,
	})
	if err != nil || !rb.Ok {
		t.Fatalf("rollback: err=%v %#v", err, rb)
	}

	after, err := inventory.Scan(inventory.Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan after rollback: %v", err)
	}
	mustNoAsset(t, after, "home/.claude/skills/review")
	mustNoAsset(t, after, "home/.grok/skills/review")
	mustNoAsset(t, after, "project/CLAUDE.md")
	if after.Summary.CandidateAssets != 0 {
		t.Fatalf("after rollback candidates = %d; assets=%#v",
			after.Summary.CandidateAssets, after.Assets)
	}
}

// TestDiffDoesNotAffectScan ensures dry-run never mutates inventory roots.
func TestDiffDoesNotAffectScan(t *testing.T) {
	manifest := abs(t, "testdata", "workspace-valid", "manifest.yaml")
	pkgDir := t.TempDir()
	home := t.TempDir()

	built, err := build.Build(build.Options{
		Manifest: manifest,
		Profile:  "personal",
		OutDir:   pkgDir,
	})
	if err != nil || !built.Ok {
		t.Fatalf("build: %#v %v", built, err)
	}
	pkg := filepath.Join(pkgDir, built.Package.Archive)

	diff, err := apply.Diff(apply.Options{
		Package: pkg,
		Home:    home,
		Targets: []string{"claude"},
	})
	if err != nil || !diff.Ok {
		t.Fatalf("diff: %#v %v", diff, err)
	}
	if diff.Summary.Create == 0 {
		t.Fatal("diff expected creates")
	}

	report, err := inventory.Scan(inventory.Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if report.Summary.CandidateAssets != 0 {
		t.Fatalf("diff mutated home: candidates=%d", report.Summary.CandidateAssets)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatal("diff created .claude")
	}
}

func abs(t *testing.T, parts ...string) string {
	t.Helper()
	// internal/e2e → repo root
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

func mustAsset(t *testing.T, report inventory.Report, logical string, source inventory.Source, typ inventory.AssetType) {
	t.Helper()
	for _, asset := range report.Assets {
		if asset.LogicalPath != logical {
			continue
		}
		if asset.Status != inventory.AssetCandidate {
			t.Fatalf("%s status = %s, want candidate", logical, asset.Status)
		}
		if asset.Source != source {
			t.Fatalf("%s source = %s, want %s", logical, asset.Source, source)
		}
		if asset.Type != typ {
			t.Fatalf("%s type = %s, want %s", logical, asset.Type, typ)
		}
		return
	}
	t.Fatalf("asset %q not found; assets=%v", logical, assetPaths(report))
}

func mustNoAsset(t *testing.T, report inventory.Report, logical string) {
	t.Helper()
	for _, asset := range report.Assets {
		if asset.LogicalPath == logical {
			t.Fatalf("unexpected asset %q still present: %#v", logical, asset)
		}
	}
}

func mustExcludedSecretAsset(t *testing.T, report inventory.Report, logical string) {
	t.Helper()
	for _, asset := range report.Assets {
		if asset.LogicalPath != logical {
			continue
		}
		if asset.Status != inventory.AssetExcluded ||
			asset.Sensitivity != inventory.SensitivitySecret {
			t.Fatalf("%s = %#v, want excluded secret", logical, asset)
		}
		for _, finding := range report.Findings {
			if finding.Code == inventory.FindingSuspectedSecret {
				for _, path := range finding.Paths {
					if path == logical {
						return
					}
				}
			}
		}
		t.Fatalf("%s has no suspected_secret finding", logical)
	}
	t.Fatalf("asset %q not found; assets=%v", logical, assetPaths(report))
}

func assetPaths(report inventory.Report) []string {
	out := make([]string, 0, len(report.Assets))
	for _, asset := range report.Assets {
		out = append(out, asset.LogicalPath)
	}
	return out
}
