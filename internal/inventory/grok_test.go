package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrokProbeInventoryTypesAndExclusions(t *testing.T) {
	home := fixture(t, "home-grok")
	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	assertAsset(t, report, "home/.grok/skills/review", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Type != TypeSkill || asset.Status != AssetCandidate {
			t.Fatalf("grok skill = %#v", asset)
		}
	})
	assertAsset(t, report, "home/.grok/rules/default.md", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Type != TypeRules || asset.Status != AssetCandidate {
			t.Fatalf("grok rules = %#v", asset)
		}
	})
	assertAsset(t, report, "home/.grok/agents/explorer.md", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Type != TypeAgent {
			t.Fatalf("grok agent = %#v", asset)
		}
	})
	assertAsset(t, report, "home/.grok/hooks/session-start.json", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Type != TypeHook {
			t.Fatalf("grok hook = %#v", asset)
		}
	})
	assertAsset(t, report, "home/.grok/mcp/example.json", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Type != TypeMCPConfig {
			t.Fatalf("grok mcp = %#v", asset)
		}
	})
	assertAsset(t, report, "home/.grok/config.toml", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Type != TypeConfig || asset.ConfigState != ConfigValid {
			t.Fatalf("grok config = %#v", asset)
		}
		if !containsFeature(asset.Features, FeatureMCP) {
			t.Fatalf("grok config missing mcp feature: %#v", asset.Features)
		}
	})

	// Shared skill stays shared and is not attributed to grok.
	assertAsset(t, report, "home/.agents/skills/shared-review", func(asset Asset) {
		if asset.Source != SourceShared || asset.Type != TypeSkill {
			t.Fatalf("shared skill = %#v", asset)
		}
	})

	// Exactly one shared skill + one grok skill candidate (no double-count).
	if report.Summary.CandidateByType[TypeSkill] != 2 {
		t.Fatalf("skill candidates = %d, want 2 (shared+grok)", report.Summary.CandidateByType[TypeSkill])
	}
	sharedSkills, grokSkills := 0, 0
	for _, asset := range report.Assets {
		if asset.Type != TypeSkill || asset.Status != AssetCandidate {
			continue
		}
		switch asset.Source {
		case SourceShared:
			sharedSkills++
		case SourceGrok:
			grokSkills++
		}
	}
	if sharedSkills != 1 || grokSkills != 1 {
		t.Fatalf("shared=%d grok=%d, want 1 each", sharedSkills, grokSkills)
	}

	// Exclusions: auth, sessions, logs, downloads, bundled — no content leak.
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(encoded)
	for _, secret := range []string{
		"sk-test-grok-auth-must-never-appear",
		"sk-test-session-must-never-appear",
		"sk-test-log-must-never-appear",
		"sk-test-active-session-must-never-appear",
	} {
		if strings.Contains(out, secret) {
			t.Fatalf("report leaked %q", secret)
		}
	}
	assertEntry(t, report, "home/.grok/auth.json", func(entry Entry) {
		if entry.Status != EntryExcluded || entry.ExclusionReason != ExcludeCredential || entry.SHA256 != "" {
			t.Fatalf("auth entry = %#v", entry)
		}
	})
	assertEntry(t, report, "home/.grok/active_sessions.json", func(entry Entry) {
		if entry.Status != EntryExcluded || entry.ExclusionReason != ExcludeDeviceState || entry.SHA256 != "" {
			t.Fatalf("active_sessions entry = %#v", entry)
		}
	})
	assertEntry(t, report, "home/.grok/sessions", func(entry Entry) {
		if entry.Status != EntryExcluded || entry.ExclusionReason != ExcludeNativeSession {
			t.Fatalf("sessions entry = %#v", entry)
		}
	})
	assertEntry(t, report, "home/.grok/logs", func(entry Entry) {
		if entry.Status != EntryExcluded || entry.ExclusionReason != ExcludeDeviceState {
			t.Fatalf("logs entry = %#v", entry)
		}
	})
	assertEntry(t, report, "home/.grok/downloads", func(entry Entry) {
		if entry.Status != EntryExcluded || entry.ExclusionReason != ExcludeDeviceState {
			t.Fatalf("downloads entry = %#v", entry)
		}
	})
	assertEntry(t, report, "home/.grok/bundled", func(entry Entry) {
		if entry.Status != EntryExcluded || entry.ExclusionReason != ExcludeDeviceState {
			t.Fatalf("bundled entry = %#v", entry)
		}
	})
	// Bundled skill must not become a candidate asset.
	for _, asset := range report.Assets {
		if strings.Contains(asset.LogicalPath, "bundled") {
			t.Fatalf("bundled asset leaked: %#v", asset)
		}
	}
}

func TestGrokEmptyDirectoryProducesNoInventory(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".grok", "skills"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(report.Assets) != 0 {
		t.Fatalf("empty grok tree created assets: %#v", report.Assets)
	}
	// Empty dirs are not entries either (normal directories omitted).
	for _, entry := range report.Entries {
		if entry.FileType == FileDirectory && entry.Status != EntryExcluded {
			t.Fatalf("unexpected directory entry: %#v", entry)
		}
	}
}

func TestGrokSymlinkEscapeNotFollowed(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, []byte("outside secret sk-test-symlink-must-never-appear\n"))
	linkDir := filepath.Join(home, ".grok", "skills")
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(linkDir, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertEntry(t, report, "home/.grok/skills/escape", func(entry Entry) {
		if entry.SymlinkState != SymlinkEscape || entry.ExclusionReason != ExcludeSymlinkEscape {
			t.Fatalf("escape = %#v", entry)
		}
		if entry.Source != SourceGrok {
			t.Fatalf("source = %s", entry.Source)
		}
	})
	assertFinding(t, report, FindingSymlinkEscape)
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "sk-test-symlink-must-never-appear") {
		t.Fatal("symlink content leaked")
	}
	assertNoAsset(t, report, "home/.grok/skills/escape")
}

func TestGrokProjectProbe(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(fixture(t, "home-grok-project"), "workspace")
	report, err := Scan(Options{Home: home, Project: project})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertAsset(t, report, "project/.grok/skills/proj", func(asset Asset) {
		if asset.Source != SourceGrok || asset.Scope != ScopeProject || asset.Type != TypeSkill {
			t.Fatalf("project grok skill = %#v", asset)
		}
	})
	assertAsset(t, report, "project/AGENTS.md", func(asset Asset) {
		if asset.Source != SourceCodex || asset.Type != TypeRules {
			t.Fatalf("agents.md = %#v", asset)
		}
	})
}

func TestProbeTableListsGrok(t *testing.T) {
	homePaths := probesFor(RootHome)
	projectPaths := probesFor(RootProject)
	if !containsString(homePaths, ".grok") {
		t.Fatalf("home probes missing .grok: %#v", homePaths)
	}
	if !containsString(projectPaths, ".grok") {
		t.Fatalf("project probes missing .grok: %#v", projectPaths)
	}
	// Project must not scan home-only .claude.json
	if containsString(projectPaths, ".claude.json") {
		t.Fatal("project probes should not include .claude.json")
	}
	if !containsString(homePaths, ".claude.json") {
		t.Fatal("home probes should include .claude.json")
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
