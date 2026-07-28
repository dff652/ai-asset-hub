package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

// Every path here was taken from a real scan of one machine, where the
// device-state table only knew Claude's spelling and these trees were
// inventoried as assets. See docs/migrations/2026-07-25-dogfood-inventory.md.
func TestDeviceStateIsExcludedPerTarget(t *testing.T) {
	for _, path := range []string{
		// Codex: shell_snapshots with an underscore is the one that started this.
		".codex/shell_snapshots/019f8cc2.1784772959797048792.sh",
		".codex/.tmp/plugins/README.md",
		".codex/tmp/whatever",
		".codex/packages/standalone/bin/tool",
		".codex/attachments/9bfeef5a/pasted.txt",
		".codex/ipc/socket",
		".codex/session_index.jsonl",
		".codex/installation_id",
		".codex/version.json",
		// Codex maintains a sparse checkout of curated vendor skills here.
		".codex/vendor_imports/skills/skills/.curated/openai-docs/SKILL.md",
		// Codex ships its own system skills below the user skills directory.
		".codex/skills/.system/skill-installer/scripts/github_utils.py",
		// Claude: its own backups of .claude.json, and plugin bookkeeping.
		".claude/backups/.claude.json.backup.1784962637544",
		".claude/ide/24564.lock",
		".claude/plugins/marketplaces/claude-hud/dist/usage-api.js",
		".claude/plugins/data/codex-openai-codex/state",
		".claude/plugins/.last_inuse_sweep",
		// Grok: vendor documentation, plus the trees 2C.1 already covered.
		".grok/docs/user-guide/02-authentication.md",
		".grok/bundled/skills/imagine/SKILL.md",
		".grok/README.md",
	} {
		t.Run(path, func(t *testing.T) {
			policy := policyFor(RootHome, path)
			if policy.exclusionReason != ExcludeDeviceState {
				t.Errorf("policyFor(%q).exclusionReason = %q, want %q",
					path, policy.exclusionReason, ExcludeDeviceState)
			}
		})
	}
}

// The real vendor_imports tree is a sparse git checkout whose nested skills/
// segment looks exactly like a user asset container. Keep the fixture
// deliberately self-consistent: without the root-scoped device-state defense,
// the walker reaches a valid SKILL.md and the secret scanner reports it.
func TestScanExcludesCodexVendorImportsBeforeContentInspection(t *testing.T) {
	home := t.TempDir()
	vendorSkill := filepath.Join(
		home,
		".codex",
		"vendor_imports",
		"skills",
		"skills",
		".curated",
		"example",
		"SKILL.md",
	)
	if err := os.MkdirAll(filepath.Dir(vendorSkill), 0o755); err != nil {
		t.Fatalf("mkdir vendor skill: %v", err)
	}
	if err := os.WriteFile(
		vendorSkill,
		[]byte("# vendor fixture\napi_key = \"sk-test-vendor-import-must-not-be-scanned\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("write vendor skill: %v", err)
	}

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if countFindings(report, FindingSuspectedSecret) != 0 ||
		countFindings(report, FindingIncompleteSkill) != 0 {
		t.Fatalf("vendor import produced findings: %#v", report.Findings)
	}
	for _, asset := range report.Assets {
		if filepath.ToSlash(asset.LogicalPath) == "home/.codex/vendor_imports/skills/skills/.curated/example" {
			t.Fatalf("vendor import became an asset: %#v", asset)
		}
	}
	assertEntry(t, report, "home/.codex/vendor_imports", func(entry Entry) {
		if entry.ExclusionReason != ExcludeDeviceState || entry.Type != TypeDeviceState {
			t.Fatalf("vendor import entry = %#v", entry)
		}
	})
}

// The table is scoped per target and per location on purpose: data/ and docs/
// are ordinary directory names, and a user asset must not lose to them.
func TestDeviceStateTableDoesNotOverreach(t *testing.T) {
	for _, test := range []struct {
		path     string
		wantType AssetType
	}{
		{".claude/skills/my-skill/data/notes.md", TypeSkill},
		{".grok/skills/my-skill/docs/README.md", TypeSkill},
		{".codex/skills/my-skill/packages/list.md", TypeSkill},
		{".codex/rules/default.rules", TypeRules},
		{".codex/prompts/my-prompt.md", TypeUnknown},
		// Plan-mode artifacts are user-written content, deliberately not
		// classified as device state.
		{".claude/plans/some-plan.md", TypeUnknown},
	} {
		t.Run(test.path, func(t *testing.T) {
			policy := policyFor(RootHome, test.path)
			if policy.exclusionReason != "" {
				t.Errorf("policyFor(%q) excluded as %q", test.path, policy.exclusionReason)
			}
			if policy.assetType != test.wantType {
				t.Errorf("policyFor(%q).assetType = %q, want %q",
					test.path, policy.assetType, test.wantType)
			}
		})
	}
}

// A skill installed by symlinking its source repo must not disappear silently:
// not following the link is correct, saying nothing about it is not.
func TestSymlinkedAssetIsReported(t *testing.T) {
	home := t.TempDir()
	// The source repo lives elsewhere under the same home, exactly like a skill
	// installed by `ln -s ~/repos/<tool> ~/.agents/skills/<tool>`.
	source := filepath.Join(home, "repos", "linked-skill")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# linked\n"), 0o644); err != nil {
		t.Fatalf("write source skill: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".agents", "skills"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(source, filepath.Join(home, ".agents", "skills", "linked-skill")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// A symlink outside any asset container stays silent.
	if err := os.Symlink(source, filepath.Join(home, ".agents", "scratch")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	assertFinding(t, report, FindingSymlinkedAsset)
	if got := countFindings(report, FindingSymlinkedAsset); got != 1 {
		t.Fatalf("symlinked asset findings = %d, want 1 (non-asset symlinks stay silent)", got)
	}
	assertEntry(t, report, "home/.agents/skills/linked-skill", func(entry Entry) {
		if entry.ExclusionReason != ExcludeSymlink || entry.SymlinkState != SymlinkInternal {
			t.Fatalf("symlinked skill entry = %#v", entry)
		}
	})
}
