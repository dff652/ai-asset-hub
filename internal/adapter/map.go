package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/build"
)

// portabilityPortable marks an asset that any target can load as-is. Only such
// assets are spread to a target their manifest entry does not list.
const portabilityPortable = "portable"

// mapResult is one destination mapping for a package file.
type mapResult struct {
	rel      string // relative to tool root (e.g. skills/x/SKILL.md)
	degraded string
}

func mapAssetFile(assetType, filePath, target string) (mapResult, bool) {
	switch assetType {
	case "skill":
		rel, ok := mapUnder(filePath, "assets/skills/", "skills")
		return mapResult{rel: rel}, ok
	case "rules":
		return mapRulesFile(filePath, target)
	case "agent":
		rel, ok := mapUnder(filePath, "assets/agents/", "agents")
		return mapResult{rel: rel}, ok
	case "hook":
		rel, ok := mapUnder(filePath, "assets/hooks/", "hooks")
		return mapResult{rel: rel}, ok
	case "mcp":
		return mapMCPFile(filePath, target)
	default:
		return mapResult{}, false
	}
}

func mapUnder(filePath, prefix, destRoot string) (string, bool) {
	if !strings.HasPrefix(filePath, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(filePath, prefix)
	if rest == "" || strings.Contains(rest, "..") {
		return "", false
	}
	return path.Join(destRoot, rest), true
}

// mapRulesFile places CLAUDE.md / AGENTS.md at the tool root when names match;
// other rules stay under rules/.
func mapRulesFile(filePath, target string) (mapResult, bool) {
	const prefix = "assets/rules/"
	if !strings.HasPrefix(filePath, prefix) {
		return mapResult{}, false
	}
	rest := strings.TrimPrefix(filePath, prefix)
	if rest == "" || strings.Contains(rest, "..") {
		return mapResult{}, false
	}
	base := path.Base(rest)
	switch strings.ToLower(base) {
	case "claude.md":
		if target == TargetClaude || target == TargetGrok {
			return mapResult{rel: "CLAUDE.md"}, true
		}
		if target == TargetCodex {
			return mapResult{
				rel:      "rules/" + rest,
				degraded: "claude.md installed under rules/ for codex (not AGENTS.md)",
			}, true
		}
	case "agents.md":
		if target == TargetCodex || target == TargetGrok {
			return mapResult{rel: "AGENTS.md"}, true
		}
		if target == TargetClaude {
			return mapResult{
				rel:      "rules/" + rest,
				degraded: "agents.md installed under rules/ for claude (not CLAUDE.md)",
			}, true
		}
	}
	return mapResult{rel: path.Join("rules", rest)}, true
}

// mapMCPFile stages portable templates under mcp/*.json. Apply may create a
// missing harness-native config (see BuildNativeMCPConfigFiles).
func mapMCPFile(filePath, target string) (mapResult, bool) {
	rel, ok := mapUnder(filePath, "assets/mcp/", "mcp")
	if !ok {
		return mapResult{}, false
	}
	_ = target
	return mapResult{rel: rel}, true
}

func compileToolTree(
	target, toolDir string,
	manifest build.PackageManifest,
	files map[string][]byte,
	includeTypes map[string]bool,
) ([]StagedFile, CompileReport) {
	report := CompileReport{Target: target}
	out := make([]StagedFile, 0)

	for _, asset := range manifest.Assets {
		if !shouldIncludeAsset(asset, target) {
			continue
		}
		if !includeTypes[asset.Type] {
			report.Dropped = append(report.Dropped, asset.ID+": unsupported type "+asset.Type)
			continue
		}
		if target == TargetGrok && !assetHasTarget(asset, TargetGrok) {
			// Visible rather than silent: the asset is installed somewhere its
			// own targets list does not mention. Recorded after the type filter
			// so a dropped asset is never also reported as installed.
			report.Degraded = append(report.Degraded,
				asset.ID+": installed into grok as a portable "+asset.Type+
					" although its targets do not list grok")
		}
		for _, file := range asset.Files {
			body, ok := files[file.Path]
			if !ok {
				report.Dropped = append(report.Dropped, asset.ID+": missing "+file.Path)
				continue
			}
			mapped, ok := mapAssetFile(asset.Type, file.Path, target)
			if !ok {
				report.Dropped = append(report.Dropped, asset.ID+": unmapped "+file.Path)
				continue
			}
			sha := file.SHA256
			if sha == "" {
				sum := sha256.Sum256(body)
				sha = hex.EncodeToString(sum[:])
			}
			staged := StagedFile{
				RelPath:  path.Join(toolDir, mapped.rel),
				Body:     body,
				SHA256:   sha,
				AssetID:  asset.ID,
				Target:   target,
				Source:   file.Path,
				Degraded: mapped.degraded,
				Scope:    asset.Scope,
			}
			// Project-scoped rules that map to CLAUDE.md/AGENTS.md install at project root,
			// not under .claude/CLAUDE.md — handled by apply when Scope==project.
			if asset.Scope == "project" && (mapped.rel == "CLAUDE.md" || mapped.rel == "AGENTS.md") {
				staged.RelPath = mapped.rel
			}
			out = append(out, staged)
			report.Emitted++
			if mapped.degraded != "" {
				report.Degraded = append(report.Degraded, asset.ID+": "+mapped.degraded)
			}
		}
	}
	sort.Strings(report.Dropped)
	sort.Strings(report.Degraded)
	return out, report
}

func shouldIncludeAsset(asset build.PackageAsset, target string) bool {
	switch target {
	case TargetShared:
		if asset.Type != "skill" {
			return false
		}
		return assetHasTarget(asset, TargetClaude) ||
			assetHasTarget(asset, TargetCodex) ||
			assetHasTarget(asset, TargetGrok) ||
			assetHasTarget(asset, TargetShared)
	case TargetGrok:
		if assetHasTarget(asset, TargetGrok) {
			return true
		}
		// Grok reads skills and rules from its own tree, so an asset declared
		// portable can land there without listing grok. An adapter-required one
		// cannot: it is written in one harness's format, and Codex's
		// rules/default.rules is a command-approval DSL Grok has no use for.
		// Spreading it there contradicted what `targets: [codex]` plainly says.
		if asset.Type == "skill" || asset.Type == "rules" {
			return asset.Portability == portabilityPortable &&
				(assetHasTarget(asset, TargetClaude) || assetHasTarget(asset, TargetCodex))
		}
		return false
	default:
		return assetHasTarget(asset, target)
	}
}
