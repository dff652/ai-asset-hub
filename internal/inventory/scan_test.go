package inventory

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScanBuildsLogicalAssetsFromFileFacts(t *testing.T) {
	home := fixture(t, "home-basic")
	first, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	second, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated scans returned different reports")
	}

	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(encoded), home) {
		t.Fatalf("report leaked absolute fixture path %q", home)
	}
	if first.Summary.CandidateAssets != 14 {
		t.Fatalf("candidate assets = %d, want 14", first.Summary.CandidateAssets)
	}
	if first.Summary.CandidateByType[TypeSkill] != 3 {
		t.Fatalf("skill assets = %d, want 3", first.Summary.CandidateByType[TypeSkill])
	}
	if first.Summary.DeviceMigration != DeviceInventoryOnly {
		t.Fatalf("device migration = %q, want inventory-only", first.Summary.DeviceMigration)
	}
	for _, entry := range first.Entries {
		if entry.FileType == FileDirectory {
			t.Fatalf("normal directory leaked into entries: %#v", entry)
		}
	}

	assertAsset(t, first, "home/.claude/skills/review", func(asset Asset) {
		if asset.Type != TypeSkill || asset.Status != AssetCandidate ||
			asset.Portability != PortabilityAdapterRequired {
			t.Fatalf("skill asset = %#v", asset)
		}
		if !reflect.DeepEqual(asset.Files, []string{"home/.claude/skills/review/SKILL.md"}) {
			t.Fatalf("skill files = %#v", asset.Files)
		}
	})
	assertEntry(t, first, "home/.claude/skills/review/SKILL.md", func(entry Entry) {
		if entry.AssetPath != "home/.claude/skills/review" || len(entry.SHA256) != 64 {
			t.Fatalf("skill entry = %#v", entry)
		}
	})
	assertEntry(t, first, "home/.claude/mystery.txt", func(entry Entry) {
		if entry.Type != TypeUnknown || entry.Status != EntryReported || entry.AssetPath != "" {
			t.Fatalf("unknown entry = %#v", entry)
		}
	})
	assertAsset(t, first, "home/.codex/config.toml", func(asset Asset) {
		if asset.ConfigState != ConfigValid || !containsFeature(asset.Features, FeatureMCP) ||
			!containsFeature(asset.Features, FeatureCodexProjectDocFallback) {
			t.Fatalf("Codex config asset = %#v", asset)
		}
	})
}

func TestEmptyAssetDirectoriesDoNotChangeProjectMigration(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	for _, path := range []string{
		filepath.Join(project, ".claude", "skills"),
		filepath.Join(project, ".codex", "plugins"),
		filepath.Join(project, ".agents", "skills", "empty-skill"),
	} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}

	report, err := Scan(Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(report.Assets) != 0 || len(report.Entries) != 0 {
		t.Fatalf("empty directories created inventory: %#v", report)
	}
	if report.Summary.ProjectMigration != ProjectNoAssets {
		t.Fatalf("project migration = %q, want no-assets", report.Summary.ProjectMigration)
	}
}

func TestSkillGroupsAllFilesAndSecretBlocksWholeAsset(t *testing.T) {
	home := t.TempDir()
	skillRoot := filepath.Join(home, ".agents", "skills", "example")
	mustWrite(t, filepath.Join(skillRoot, "SKILL.md"), []byte("# Example\n"))
	mustWrite(t, filepath.Join(skillRoot, "references", "guide.md"), []byte("# Guide\n"))
	mustWrite(t, filepath.Join(skillRoot, "config.toml"), []byte("enabled = true\n"))
	mustWrite(t, filepath.Join(skillRoot, ".env"), []byte("SECRET_VALUE=never-read\n"))

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertAsset(t, report, "home/.agents/skills/example", func(asset Asset) {
		if asset.Type != TypeSkill || asset.Status != AssetExcluded ||
			asset.Portability != PortabilityExcluded ||
			asset.Sensitivity != SensitivitySecret {
			t.Fatalf("skill asset = %#v", asset)
		}
		if len(asset.Files) != 4 {
			t.Fatalf("skill files = %#v, want four files", asset.Files)
		}
	})
	if report.Summary.CandidateByType[TypeSkill] != 0 ||
		report.Summary.ExcludedAssets != 1 {
		t.Fatalf("skill summary = %#v", report.Summary)
	}
	assertEntry(t, report, "home/.agents/skills/example/config.toml", func(entry Entry) {
		if entry.Type != TypeSkill || entry.AssetPath != "home/.agents/skills/example" {
			t.Fatalf("nested config escaped skill asset: %#v", entry)
		}
	})
}

func TestSkillWithoutManifestIsReportedButNotAnAsset(t *testing.T) {
	home := t.TempDir()
	mustWrite(t,
		filepath.Join(home, ".agents", "skills", "orphan", "notes.md"),
		[]byte("# Notes without a skill manifest\n"),
	)

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertNoAsset(t, report, "home/.agents/skills/orphan")
	assertEntry(t, report, "home/.agents/skills/orphan/notes.md", func(entry Entry) {
		if entry.Type != TypeSkill || entry.Status != EntryReported ||
			entry.Portability != PortabilityUnknown || entry.AssetPath != "" {
			t.Fatalf("orphan skill entry = %#v", entry)
		}
	})
	assertFinding(t, report, FindingIncompleteSkill)
	if report.Summary.DeviceMigration != DeviceNoAssets {
		t.Fatalf("device migration = %q, want no-assets", report.Summary.DeviceMigration)
	}
}

func TestClaudeSetupTreeIsOneProjectSourceAsset(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	mustWrite(t,
		filepath.Join(project, "scripts", "claude-setup", "templates", "skills", "review", "SKILL.md"),
		[]byte("# Review\n"),
	)
	mustWrite(t,
		filepath.Join(project, "scripts", "claude-setup", "install.sh"),
		[]byte("#!/bin/sh\n"),
	)

	report, err := Scan(Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertAsset(t, report, "project/scripts/claude-setup", func(asset Asset) {
		if asset.Type != TypeProjectSource || len(asset.Files) != 2 {
			t.Fatalf("project source asset = %#v", asset)
		}
	})
	if report.Summary.CandidateByType[TypeProjectSource] != 1 {
		t.Fatalf("project source count = %#v", report.Summary.CandidateByType)
	}
}

func TestSensitiveFixtureNeverLeaksValuesOrHashes(t *testing.T) {
	home := fixture(t, "home-sensitive")
	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	output := string(encoded)
	for _, secret := range []string{
		"sk-test-credential-must-never-appear",
		"github_pat_test_auth_must_never_appear",
		"sk-test-inline-secret-must-never-appear",
		"fixture-token-that-must-never-appear",
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("report leaked fixture secret %q", secret)
		}
	}

	for _, logicalPath := range []string{
		"home/.claude/.credentials.json",
		"home/.codex/auth.json",
		"home/.claude/CLAUDE.md",
		"home/.codex/config.toml",
		"home/.codex/state_1.sqlite",
	} {
		assertEntry(t, report, logicalPath, func(entry Entry) {
			if entry.Status != EntryExcluded || entry.SHA256 != "" {
				t.Fatalf("sensitive entry was not safely excluded: %#v", entry)
			}
		})
	}
	assertAsset(t, report, "home/.claude/CLAUDE.md", func(asset Asset) {
		if asset.Status != AssetExcluded || asset.Sensitivity != SensitivitySecret {
			t.Fatalf("secret rule asset = %#v", asset)
		}
	})
	assertFinding(t, report, FindingSuspectedSecret)
	if report.Summary.DeviceMigration != DeviceBlocked {
		t.Fatalf("device migration = %q, want blocked", report.Summary.DeviceMigration)
	}
}

func TestProjectRulesShadowConfiguredFallback(t *testing.T) {
	home := fixture(t, "home-conflicts")
	project := filepath.Join(home, "workspace")
	report, err := Scan(Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertFinding(t, report, FindingRulesShadowing)
	if report.Summary.ProjectMigration != ProjectConflicted {
		t.Fatalf("project migration = %q, want conflicted", report.Summary.ProjectMigration)
	}
}

func TestAbsentToolDirectoriesProduceEmptyInventory(t *testing.T) {
	report, err := Scan(Options{Home: t.TempDir()})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(report.Assets) != 0 || len(report.Entries) != 0 || len(report.Findings) != 0 {
		t.Fatalf("empty home report = %#v", report)
	}
	if report.Summary.DeviceMigration != DeviceNoAssets {
		t.Fatalf("device migration = %q, want no-assets", report.Summary.DeviceMigration)
	}
}

func TestUnreadableFileIsNotOpened(t *testing.T) {
	home := t.TempDir()
	file := filepath.Join(home, ".claude", "CLAUDE.md")
	mustWrite(t, file, []byte("private instructions\n"))
	if err := os.Chmod(file, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(file, 0600) })

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertEntry(t, report, "home/.claude/CLAUDE.md", func(entry Entry) {
		if entry.Status != EntryUnreadable || entry.SHA256 != "" ||
			entry.ExclusionReason != ExcludeUnreadable {
			t.Fatalf("unreadable entry = %#v", entry)
		}
	})
	assertAsset(t, report, "home/.claude/CLAUDE.md", func(asset Asset) {
		if asset.Status != AssetUnreadable {
			t.Fatalf("unreadable asset = %#v", asset)
		}
	})
	assertFinding(t, report, FindingUnreadable)
}

func TestSymlinkEscapeAndBrokenSymlinkAreNotFollowed(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, []byte("outside root\n"))
	linkDir := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(linkDir, "escape")); err != nil {
		t.Fatalf("symlink escape: %v", err)
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(linkDir, "broken")); err != nil {
		t.Fatalf("symlink broken: %v", err)
	}

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertEntry(t, report, "home/.agents/skills/escape", func(entry Entry) {
		if entry.SymlinkState != SymlinkEscape || entry.ExclusionReason != ExcludeSymlinkEscape ||
			entry.Size != 0 {
			t.Fatalf("escaping symlink = %#v", entry)
		}
	})
	assertEntry(t, report, "home/.agents/skills/broken", func(entry Entry) {
		if entry.SymlinkState != SymlinkBroken || entry.ExclusionReason != ExcludeBrokenSymlink {
			t.Fatalf("broken symlink = %#v", entry)
		}
	})
	assertNoAsset(t, report, "home/.agents/skills/escape")
	assertNoAsset(t, report, "home/.agents/skills/broken")
	assertFinding(t, report, FindingSymlinkEscape)
	assertFinding(t, report, FindingBrokenSymlink)
}

func TestInventoryAssemblyDoesNotMutateInspections(t *testing.T) {
	input := []inspection{{
		entry: Entry{
			LogicalPath: "home/.claude/CLAUDE.md",
			Source:      SourceClaude,
			Scope:       ScopeGlobal,
			Type:        TypeRules,
			FileType:    FileRegular,
			Portability: PortabilityAdapterRequired,
			Sensitivity: SensitivityPrivate,
			Status:      EntryIncluded,
		},
	}}
	before := append([]inspection(nil), input...)

	assets, entries, _ := assembleInventory(input)

	if !reflect.DeepEqual(input, before) {
		t.Fatalf("assembly mutated input: before=%#v after=%#v", before, input)
	}
	if len(assets) != 1 || len(entries) != 1 ||
		entries[0].AssetPath != "home/.claude/CLAUDE.md" {
		t.Fatalf("assembly output assets=%#v entries=%#v", assets, entries)
	}
}

func TestMalformedJSONAndTOMLUseSanitizedFindings(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{"hooks":`))
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), []byte("[mcp_servers.invalid\n"))

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if countFindings(report, FindingInvalidConfig) != 2 {
		t.Fatalf("invalid config findings = %#v", report.Findings)
	}
	for _, finding := range report.Findings {
		if finding.Code == FindingInvalidConfig &&
			(strings.Contains(finding.Message, "hooks") || strings.Contains(finding.Message, "mcp_servers")) {
			t.Fatalf("parser finding leaked input: %#v", finding)
		}
	}
}

func TestConfigFeaturesUseExactKnownFields(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{
  "webhook_url": "fixture",
  "microplugin": true,
  "hooksDisabled": true
}`))

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertAsset(t, report, "home/.claude/settings.json", func(asset Asset) {
		if len(asset.Features) != 0 {
			t.Fatalf("unrecognized keys produced features: %#v", asset.Features)
		}
	})
}

func TestExcludedDirectorySizeIsCanonicalZero(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex", "cache", "nested"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertEntry(t, report, "home/.codex/cache", func(entry Entry) {
		if entry.FileType != FileDirectory || entry.Size != 0 ||
			entry.ExclusionReason != ExcludeCache {
			t.Fatalf("excluded directory = %#v", entry)
		}
	})
}

func TestScanDoesNotModifyFiles(t *testing.T) {
	home := fixture(t, "home-basic")
	file := filepath.Join(home, ".claude", "CLAUDE.md")
	before, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	beforeContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	if _, err := Scan(Options{Home: home}); err != nil {
		t.Fatalf("scan: %v", err)
	}

	after, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	afterContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode() ||
		!reflect.DeepEqual(beforeContent, afterContent) {
		t.Fatal("scan modified fixture metadata or content")
	}
}

func TestInvalidRootReturnsSanitizedError(t *testing.T) {
	_, err := Scan(Options{Home: filepath.Join(t.TempDir(), "missing")})
	if !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("error = %v, want ErrInvalidRoot", err)
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	file, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	return file
}

func mustWrite(t *testing.T, file string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", file, err)
	}
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

func assertAsset(t *testing.T, report Report, logicalPath string, check func(Asset)) {
	t.Helper()
	for _, asset := range report.Assets {
		if asset.LogicalPath == logicalPath {
			check(asset)
			return
		}
	}
	t.Fatalf("asset %q not found", logicalPath)
}

func assertNoAsset(t *testing.T, report Report, logicalPath string) {
	t.Helper()
	for _, asset := range report.Assets {
		if asset.LogicalPath == logicalPath {
			t.Fatalf("unexpected asset %q: %#v", logicalPath, asset)
		}
	}
}

func assertEntry(t *testing.T, report Report, logicalPath string, check func(Entry)) {
	t.Helper()
	for _, entry := range report.Entries {
		if entry.LogicalPath == logicalPath {
			check(entry)
			return
		}
	}
	t.Fatalf("entry %q not found", logicalPath)
}

func assertFinding(t *testing.T, report Report, code FindingCode) {
	t.Helper()
	if countFindings(report, code) == 0 {
		t.Fatalf("finding %q not found in %#v", code, report.Findings)
	}
}

func countFindings(report Report, code FindingCode) int {
	count := 0
	for _, finding := range report.Findings {
		if finding.Code == code {
			count++
		}
	}
	return count
}

func containsFeature(values []Feature, wanted Feature) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
