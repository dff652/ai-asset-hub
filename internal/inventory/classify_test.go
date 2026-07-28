package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

// Cache exclusion must match whole path segments only. Basename substring
// matching (removed in b274ce1) dropped whole asset trees whose directory
// name merely contained "cache".
func TestCacheExclusionMatchesWholePathSegments(t *testing.T) {
	tests := []struct {
		name       string
		rootID     RootID
		relative   string
		wantReason ExclusionReason
		wantType   AssetType
	}{
		// Assets whose name merely contains "cache" stay assets. The directory
		// cases are the regression that mattered: an excluded directory makes
		// the walker skip the whole subtree.
		{"skill dir named like cache", RootHome, ".claude/skills/cache-warmer", "", TypeSkill},
		{"skill file under cache-named dir", RootHome, ".claude/skills/cache-warmer/SKILL.md", "", TypeSkill},
		{"shared skill dir with cache suffix", RootHome, ".agents/skills/go-cache", "", TypeSkill},
		{"agent named like cache", RootHome, ".claude/agents/cache-mgr.md", "", TypeAgent},
		{"agent named with cache suffix", RootHome, ".claude/agents/build-cache.md", "", TypeAgent},
		{"rule named with cache suffix", RootHome, ".codex/rules/write-cache.md", "", TypeRules},
		{"project doc about caching", RootProject, "docs/cache-invalidation-guide.md", "", TypeUnknown},
		{"project doc named caching", RootProject, "docs/caching.md", "", TypeUnknown},
		// Real finds from a $HOME scan: cache-named source and docs inside an
		// asset tree are content, not harness state.
		{"plugin source named cache", RootHome, ".claude/plugins/claude-hud/src/context-cache.ts", "", TypePlugin},
		{"skill reference named CACHE.md", RootHome, ".claude/skills/transformers/references/CACHE.md", "", TypeSkill},

		// Real cache containers stay excluded, with the cache asset type.
		{"codex cache dir", RootHome, ".codex/cache", ExcludeCache, TypeCache},
		{"nested cache file", RootHome, ".codex/cache/nested/data.json", ExcludeCache, TypeCache},
		{"cache dir under plugins", RootHome, ".claude/plugins/cache/entries.json", ExcludeCache, TypeCache},
		{"plural caches segment", RootHome, ".claude/caches/something", ExcludeCache, TypeCache},

		// Names harnesses actually produce. Segment-exact matching alone missed
		// every one of these, and transcript-cache was even promoted to an
		// includable plugin asset.
		{"dot-prefixed cache dir", RootHome, ".claude/.cache/entries", ExcludeCache, TypeCache},
		{"paste cache dir", RootHome, ".claude/paste-cache", ExcludeCache, TypeCache},
		{"paste cache content", RootHome, ".claude/paste-cache/img1.json", ExcludeCache, TypeCache},
		{"plugin transcript cache", RootHome, ".claude/plugins/claude-hud/transcript-cache/t.json", ExcludeCache, TypeCache},
		{"plugin catalog cache file", RootHome, ".claude/plugins/plugin-catalog-cache.json", ExcludeCache, TypeCache},
		{"claude stats cache file", RootHome, ".claude/stats-cache.json", ExcludeCache, TypeCache},
		{"codex models cache file", RootHome, ".codex/models_cache.json", ExcludeCache, TypeCache},
		{"grok marketplace cache", RootHome, ".grok/marketplace-cache/abc/dist/x.js", ExcludeCache, TypeCache},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := policyFor(tt.rootID, tt.relative)
			if policy.exclusionReason != tt.wantReason {
				t.Errorf("policyFor(%q, %q).exclusionReason = %q, want %q",
					tt.rootID, tt.relative, policy.exclusionReason, tt.wantReason)
			}
			if policy.assetType != tt.wantType {
				t.Errorf("policyFor(%q, %q).assetType = %q, want %q",
					tt.rootID, tt.relative, policy.assetType, tt.wantType)
			}
		})
	}
}

// End-to-end anchor: a skill directory named like a cache must survive the
// walk. policyFor alone cannot prove this, because exclusion also triggers
// filepath.SkipDir and silently drops every file below it.
func TestScanKeepsSkillTreeNamedLikeCache(t *testing.T) {
	home := t.TempDir()
	skillDir := filepath.Join(home, ".claude", "skills", "cache-warmer")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# cache warmer\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	report, err := Scan(Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	assertAsset(t, report, "home/.claude/skills/cache-warmer", func(asset Asset) {
		if asset.Type != TypeSkill || asset.Status != AssetCandidate {
			t.Fatalf("skill asset = %#v", asset)
		}
	})
	assertEntry(t, report, "home/.claude/skills/cache-warmer/SKILL.md", func(entry Entry) {
		if entry.ExclusionReason != "" || entry.Type != TypeSkill {
			t.Fatalf("skill entry = %#v", entry)
		}
	})
}
