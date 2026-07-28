package adapter

import (
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/build"
)

// Target IDs supported in Phase 2.
const (
	TargetClaude = "claude"
	TargetCodex  = "codex"
	TargetGrok   = "grok"
	TargetShared = "shared" // T0 .agents skill root
)

// StagedFile is one file ready to install under a logical root (home or project).
type StagedFile struct {
	// RelPath is relative to the install root, using forward slashes.
	RelPath  string
	Body     []byte
	SHA256   string
	AssetID  string
	Target   string
	Source   string // package path or generated-output origin
	Degraded string // optional reason if capability was reduced
	Scope    string // global | project | device
	// Mode is unix permission bits for install (0 → 0644). Hooks scripts use 0755.
	Mode uint32
}

// CompileReport summarizes adapter output for one target.
type CompileReport struct {
	Target   string   `json:"target"`
	Emitted  int      `json:"emitted"`
	Dropped  []string `json:"dropped,omitempty"`
	Degraded []string `json:"degraded,omitempty"`
}

// Adapter maps package assets into staged files for one harness.
type Adapter interface {
	ID() string
	Compile(manifest build.PackageManifest, files map[string][]byte) ([]StagedFile, CompileReport)
}

// Registry returns the built-in Phase 2 adapters.
func Registry() map[string]Adapter {
	return map[string]Adapter{
		TargetClaude: claudeAdapter{},
		TargetCodex:  codexAdapter{},
		TargetGrok:   grokAdapter{},
		TargetShared: sharedAdapter{},
	}
}

// CompileTargets compiles selected targets and merges staged files.
// Selecting any of claude/codex/grok also emits shared skills once (T0).
func CompileTargets(manifest build.PackageManifest, files map[string][]byte, targets []string) ([]StagedFile, []CompileReport) {
	targets, _ = ResolveTargets(targets, manifest.Targets)
	registry := Registry()
	reports := make([]CompileReport, 0, len(targets)+1)
	staged := make([]StagedFile, 0)
	wantShared := false

	for _, target := range targets {
		ad, ok := registry[target]
		if !ok {
			reports = append(reports, CompileReport{
				Target:  target,
				Dropped: []string{"unsupported target"},
			})
			continue
		}
		filesOut, report := ad.Compile(manifest, files)
		staged = append(staged, filesOut...)
		reports = append(reports, report)
		if target == TargetClaude || target == TargetCodex || target == TargetGrok {
			wantShared = true
		}
	}

	if wantShared {
		if _, already := indexOfTarget(targets, TargetShared); !already {
			sharedManifest := manifestForShared(manifest, targets)
			filesOut, report := registry[TargetShared].Compile(sharedManifest, files)
			staged = append(staged, filesOut...)
			reports = append(reports, report)
		}
	}

	sort.SliceStable(staged, func(i, j int) bool {
		if staged[i].RelPath != staged[j].RelPath {
			return staged[i].RelPath < staged[j].RelPath
		}
		return staged[i].Target < staged[j].Target
	})
	return staged, reports
}

// ResolveTargets returns supported targets present in the package. Rejected
// contains unsupported targets and requested targets outside the package.
func ResolveTargets(requested, packageTargets []string) ([]string, []string) {
	packageSet := make(map[string]bool, len(packageTargets))
	for _, target := range packageTargets {
		packageSet[target] = true
	}
	candidates := requested
	if len(candidates) == 0 {
		candidates = packageTargets
	}
	registry := Registry()
	selected := make([]string, 0, len(candidates))
	rejected := make([]string, 0)
	seen := make(map[string]bool)
	for _, target := range candidates {
		target = strings.TrimSpace(target)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		_, supported := registry[target]
		if !supported || !packageSet[target] {
			rejected = append(rejected, target)
			continue
		}
		selected = append(selected, target)
	}
	sort.Strings(selected)
	sort.Strings(rejected)
	return selected, rejected
}

func manifestForShared(manifest build.PackageManifest, targets []string) build.PackageManifest {
	selected := make(map[string]bool, len(targets))
	for _, target := range targets {
		selected[target] = true
	}
	filtered := manifest
	filtered.Assets = make([]build.PackageAsset, 0)
	for _, asset := range manifest.Assets {
		if asset.Type != "skill" {
			continue
		}
		if assetHasTarget(asset, TargetShared) {
			filtered.Assets = append(filtered.Assets, asset)
			continue
		}
		for _, target := range asset.Targets {
			if selected[target] {
				filtered.Assets = append(filtered.Assets, asset)
				break
			}
		}
	}
	return filtered
}

func indexOfTarget(targets []string, want string) (int, bool) {
	for i, target := range targets {
		if target == want {
			return i, true
		}
	}
	return -1, false
}

func assetHasTarget(asset build.PackageAsset, target string) bool {
	for _, value := range asset.Targets {
		if value == target {
			return true
		}
	}
	return false
}
