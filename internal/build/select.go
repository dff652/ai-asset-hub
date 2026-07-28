package build

import (
	"sort"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func selectAssets(document workspace.Document, profile workspace.Profile) ([]workspace.ManifestAsset, []Finding) {
	byID := make(map[string]workspace.ManifestAsset, len(document.Assets))
	for _, asset := range document.Assets {
		byID[asset.ID] = asset
	}

	excluded := make(map[string]bool, len(profile.Exclude))
	for _, id := range profile.Exclude {
		excluded[id] = true
	}

	selected := make(map[string]bool)
	for _, id := range profile.Include {
		if !excluded[id] {
			selected[id] = true
		}
	}

	changed := true
	for changed {
		changed = false
		for id := range selected {
			asset, ok := byID[id]
			if !ok {
				continue
			}
			for _, dep := range asset.Dependencies {
				if selected[dep] {
					continue
				}
				if excluded[dep] {
					return nil, []Finding{{
						Code:     codeMissingDependency,
						Severity: workspace.SeverityError,
						Message:  "Selected asset depends on an excluded asset id.",
						Paths:    []string{id, dep},
					}}
				}
				if _, exists := byID[dep]; !exists {
					return nil, []Finding{{
						Code:     codeMissingDependency,
						Severity: workspace.SeverityError,
						Message:  "Selected asset depends on an unknown id.",
						Paths:    []string{id, dep},
					}}
				}
				selected[dep] = true
				changed = true
			}
		}
	}

	if len(selected) == 0 {
		return nil, []Finding{{
			Code:     codeEmptySelection,
			Severity: workspace.SeverityError,
			Message:  "Profile selection is empty after include/exclude.",
			Paths:    []string{"profile"},
		}}
	}

	findings := make([]Finding, 0)
	ids := sortedKeys(selected)
	for _, id := range ids {
		asset := byID[id]
		for _, conflict := range asset.Conflicts {
			if selected[conflict] {
				findings = append(findings, Finding{
					Code:     codeConflictSelected,
					Severity: workspace.SeverityError,
					Message:  "Selected assets declare a conflict.",
					Paths:    []string{id, conflict},
				})
			}
		}
	}
	if len(findings) > 0 {
		return nil, findings
	}

	assets := make([]workspace.ManifestAsset, 0, len(selected))
	for _, id := range ids {
		asset := byID[id]
		targets := filterTargets(asset.Targets, profile.Targets)
		if len(targets) == 0 {
			findings = append(findings, Finding{
				Code:     codeEmptyTargets,
				Severity: workspace.SeverityError,
				Message:  "Selected asset has no targets after profile intersection.",
				Paths:    []string{id},
			})
			continue
		}
		asset.Targets = targets
		assets = append(assets, asset)
	}
	if len(findings) > 0 {
		return nil, findings
	}
	if len(assets) == 0 {
		return nil, []Finding{{
			Code:     codeEmptySelection,
			Severity: workspace.SeverityError,
			Message:  "Profile selection is empty after target filtering.",
			Paths:    []string{"profile"},
		}}
	}
	return assets, nil
}

func filterTargets(assetTargets, profileTargets []string) []string {
	if len(profileTargets) == 0 {
		out := append([]string{}, assetTargets...)
		sort.Strings(out)
		return out
	}
	allowed := make(map[string]bool, len(profileTargets))
	for _, target := range profileTargets {
		allowed[target] = true
	}
	out := make([]string, 0, len(assetTargets))
	for _, target := range assetTargets {
		if allowed[target] {
			out = append(out, target)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
