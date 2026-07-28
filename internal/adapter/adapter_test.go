package adapter

import (
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
)

func TestCompileTargetsPhase2BTypes(t *testing.T) {
	manifest := build.PackageManifest{
		SchemaVersion: 1,
		Name:          "t",
		Version:       "1",
		Profile:       "personal",
		Targets:       []string{"claude", "codex", "grok"},
		Assets: []build.PackageAsset{
			{
				ID: "skill.demo", Type: "skill", Path: "assets/skills/demo", Scope: "global",
				Targets: []string{"claude", "codex", "grok"},
				Files:   []build.PackageFile{{Path: "assets/skills/demo/SKILL.md", SHA256: "a"}},
			},
			{
				ID: "rules.root", Type: "rules", Path: "assets/rules/CLAUDE.md", Scope: "project",
				Targets: []string{"claude"},
				Files:   []build.PackageFile{{Path: "assets/rules/CLAUDE.md", SHA256: "b"}},
			},
			{
				ID: "agent.r", Type: "agent", Path: "assets/agents/r.md", Scope: "global",
				Targets: []string{"claude"},
				Files:   []build.PackageFile{{Path: "assets/agents/r.md", SHA256: "c"}},
			},
			{
				ID: "hook.p", Type: "hook", Path: "assets/hooks/p.sh", Scope: "global",
				Targets: []string{"claude", "codex"},
				Files:   []build.PackageFile{{Path: "assets/hooks/p.sh", SHA256: "d"}},
			},
			{
				ID: "mcp.e", Type: "mcp", Path: "assets/mcp/e.json", Scope: "global",
				Targets: []string{"claude", "codex"},
				Files:   []build.PackageFile{{Path: "assets/mcp/e.json", SHA256: "e"}},
			},
		},
	}
	files := map[string][]byte{
		"assets/skills/demo/SKILL.md": []byte("# Demo\n"),
		"assets/rules/CLAUDE.md":      []byte("# Claude\n"),
		"assets/agents/r.md":          []byte("# Agent\n"),
		"assets/hooks/p.sh":           []byte("#!/bin/sh\n"),
		"assets/mcp/e.json":           []byte(`{"name":"e"}`),
	}
	staged, reports := CompileTargets(manifest, files, []string{"claude", "codex", "grok"})
	paths := map[string]bool{}
	degraded := false
	for _, file := range staged {
		paths[file.RelPath] = true
		if file.Degraded != "" {
			degraded = true
		}
	}
	for _, want := range []string{
		".claude/skills/demo/SKILL.md",
		".codex/skills/demo/SKILL.md",
		".grok/skills/demo/SKILL.md",
		".agents/skills/demo/SKILL.md",
		"CLAUDE.md", // project-scoped root rules
		".claude/agents/r.md",
		".claude/hooks/p.sh",
		".codex/hooks/p.sh",
		".claude/mcp/e.json",
		".codex/mcp/e.json",
	} {
		if !paths[want] {
			t.Fatalf("missing staged path %s in %#v", want, paths)
		}
	}
	if degraded {
		t.Fatalf("unexpected degraded staged file: %#v", staged)
	}
	if len(reports) != 4 {
		t.Fatalf("reports = %#v, want claude/codex/grok/shared", reports)
	}
	for _, report := range reports {
		if len(report.Degraded) != 0 {
			t.Fatalf("unexpected degraded report: %#v", report)
		}
	}
}

func TestResolveTargetsRequiresSupportedPackageIntersection(t *testing.T) {
	selected, rejected := ResolveTargets(
		[]string{"claude", "grok", "future"},
		[]string{"claude", "codex"},
	)
	if len(selected) != 1 || selected[0] != "claude" {
		t.Fatalf("selected = %#v", selected)
	}
	if len(rejected) != 2 || rejected[0] != "future" || rejected[1] != "grok" {
		t.Fatalf("rejected = %#v", rejected)
	}
}

func TestSharedSkillsRespectSelectedTargets(t *testing.T) {
	manifest := build.PackageManifest{
		Targets: []string{"claude", "grok"},
		Assets: []build.PackageAsset{
			{
				ID: "skill.claude", Type: "skill", Scope: "global",
				Targets: []string{"claude"},
				Files: []build.PackageFile{{
					Path: "assets/skills/claude/SKILL.md",
				}},
			},
			{
				ID: "skill.grok", Type: "skill", Scope: "global",
				Targets: []string{"grok"},
				Files: []build.PackageFile{{
					Path: "assets/skills/grok/SKILL.md",
				}},
			},
			{
				ID: "skill.shared", Type: "skill", Scope: "global",
				Targets: []string{"shared"},
				Files: []build.PackageFile{{
					Path: "assets/skills/shared/SKILL.md",
				}},
			},
		},
	}
	files := map[string][]byte{
		"assets/skills/claude/SKILL.md": []byte("# Claude\n"),
		"assets/skills/grok/SKILL.md":   []byte("# Grok\n"),
		"assets/skills/shared/SKILL.md": []byte("# Shared\n"),
	}

	staged, _ := CompileTargets(manifest, files, []string{"claude"})
	paths := make(map[string]bool)
	for _, file := range staged {
		paths[file.RelPath] = true
	}
	if !paths[".agents/skills/claude/SKILL.md"] {
		t.Fatalf("selected shared skill missing: %#v", paths)
	}
	if paths[".agents/skills/grok/SKILL.md"] {
		t.Fatalf("unselected shared skill emitted: %#v", paths)
	}
	if !paths[".agents/skills/shared/SKILL.md"] {
		t.Fatalf("explicit shared skill missing: %#v", paths)
	}
}

// P7: a manifest saying targets: [codex] must not put a Codex-format asset into
// Grok. Grok may still receive assets declared portable, but never silently:
// the spread is recorded as degraded.
func TestGrokOnlyReceivesPortableAssetsItDoesNotList(t *testing.T) {
	manifest := build.PackageManifest{
		SchemaVersion: 1, Name: "p", Version: "1", Profile: "personal",
		Targets: []string{"codex", "grok"},
		Assets: []build.PackageAsset{
			{
				ID: "rules.codex-only", Type: "rules", Path: "assets/rules/default.rules",
				Scope: "global", Portability: "adapter-required", Sensitivity: "private",
				Targets: []string{"codex"},
				Files:   []build.PackageFile{{Path: "assets/rules/default.rules", SHA256: "a"}},
			},
			{
				ID: "skill.portable", Type: "skill", Path: "assets/skills/shared",
				Scope: "global", Portability: "portable", Sensitivity: "private",
				Targets: []string{"codex"},
				Files:   []build.PackageFile{{Path: "assets/skills/shared/SKILL.md", SHA256: "b"}},
			},
		},
	}
	files := map[string][]byte{
		"assets/rules/default.rules":    []byte("prefix_rule(pattern=[\"pwd\"], decision=\"allow\")\n"),
		"assets/skills/shared/SKILL.md": []byte("# Shared\n"),
	}

	staged, reports := CompileTargets(manifest, files, []string{"codex", "grok"})
	paths := map[string]bool{}
	for _, file := range staged {
		paths[file.RelPath] = true
	}

	if paths[".grok/rules/default.rules"] {
		t.Error("adapter-required Codex rules were installed into grok")
	}
	if !paths[".codex/rules/default.rules"] {
		t.Error("Codex rules missing from its own target")
	}
	if !paths[".grok/skills/shared/SKILL.md"] {
		t.Error("portable skill missing from grok")
	}

	var grokReport CompileReport
	for _, report := range reports {
		if report.Target == TargetGrok {
			grokReport = report
		}
	}
	joined := strings.Join(grokReport.Degraded, "\n")
	if !strings.Contains(joined, "skill.portable") {
		t.Errorf("spread into grok was not recorded as degraded: %#v", grokReport.Degraded)
	}
	if strings.Contains(joined, "rules.codex-only") {
		t.Errorf("codex-only rules should not appear in grok's report: %#v", grokReport.Degraded)
	}
}
