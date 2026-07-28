package inventory

import (
	"path"
	"sort"
	"strings"
)

type assetUnit struct {
	logicalPath string
	source      Source
	scope       Scope
	assetType   AssetType
}

func assembleInventory(inspections []inspection) ([]Asset, []Entry, []detectedIssue) {
	validSkills := make(map[string]bool)
	validPlugins := make(map[string]bool)
	validProjectSources := make(map[string]bool)
	for _, result := range inspections {
		entry := result.entry
		if isSkillManifest(entry) {
			validSkills[path.Dir(entry.LogicalPath)] = true
		}
		if entry.FileType != FileSymlink && entry.Type == TypePlugin {
			if root := rootAfterSegment(entry.LogicalPath, "plugins"); root != "" {
				validPlugins[root] = true
			}
		}
		if entry.FileType != FileSymlink && entry.Type == TypeProjectSource {
			if root := prefixThrough(entry.LogicalPath, "scripts/claude-setup"); root != "" {
				validProjectSources[root] = true
			}
		}
	}

	entries := make([]Entry, 0, len(inspections))
	issues := make([]detectedIssue, 0)
	groups := make(map[string][]Entry)
	units := make(map[string]assetUnit)
	incompleteSkills := make(map[string]bool)
	for _, result := range inspections {
		entry := result.entry
		entry.Features = append([]Feature(nil), entry.Features...)
		for _, code := range result.issues {
			issues = append(issues, detectedIssue{code: code, path: entry.LogicalPath})
		}

		if unit, ok := unitKey(entry, validSkills, validPlugins, validProjectSources); ok {
			entry.AssetPath = unit.logicalPath
			groups[unit.logicalPath] = append(groups[unit.logicalPath], entry)
			units[unit.logicalPath] = unit
		} else if root := incompleteSkillRoot(entry, validSkills); root != "" {
			if entry.Status == EntryIncluded {
				entry.Status = EntryReported
				entry.Portability = PortabilityUnknown
			}
			if !incompleteSkills[root] {
				issues = append(issues, detectedIssue{
					code: FindingIncompleteSkill,
					path: root,
				})
				incompleteSkills[root] = true
			}
		}
		entries = append(entries, entry)
	}

	assets := make([]Asset, 0, len(groups))
	for logicalPath, groupedEntries := range groups {
		assets = append(assets, reduceAsset(units[logicalPath], groupedEntries))
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].LogicalPath < assets[j].LogicalPath
	})
	return assets, entries, issues
}

func isSkillManifest(entry Entry) bool {
	if entry.Type != TypeSkill || entry.FileType != FileRegular ||
		path.Base(entry.LogicalPath) != "SKILL.md" {
		return false
	}
	root := rootAfterSegment(entry.LogicalPath, "skills")
	return root != "" && path.Dir(entry.LogicalPath) == root
}

func unitKey(
	entry Entry,
	validSkills, validPlugins, validProjectSources map[string]bool,
) (assetUnit, bool) {
	if root := prefixThrough(entry.LogicalPath, "scripts/claude-setup"); validProjectSources[root] {
		return unitForRoot(root, TypeProjectSource), true
	}

	root, containerType := firstPackageRoot(entry.LogicalPath)
	switch containerType {
	case TypeSkill:
		if validSkills[root] {
			return unitForRoot(root, TypeSkill), true
		}
		return assetUnit{}, false
	case TypePlugin:
		if validPlugins[root] {
			return unitForRoot(root, TypePlugin), true
		}
		return assetUnit{}, false
	}

	if entry.FileType == FileSymlink {
		return assetUnit{}, false
	}
	switch entry.Type {
	case TypeRules, TypeAgent, TypeHook, TypeConfig, TypeMCPConfig:
		return assetUnit{
			logicalPath: entry.LogicalPath,
			source:      entry.Source,
			scope:       entry.Scope,
			assetType:   entry.Type,
		}, true
	default:
		return assetUnit{}, false
	}
}

func firstPackageRoot(logicalPath string) (string, AssetType) {
	parts := strings.Split(logicalPath, "/")
	for index, part := range parts {
		if index+1 >= len(parts) {
			break
		}
		switch part {
		case "skills":
			return strings.Join(parts[:index+2], "/"), TypeSkill
		case "plugins":
			return strings.Join(parts[:index+2], "/"), TypePlugin
		}
	}
	return "", TypeUnknown
}

func unitForRoot(logicalPath string, assetType AssetType) assetUnit {
	rootID, relative := splitLogicalPath(logicalPath)
	source := sourceFor(rootID, relative)
	if assetType == TypeProjectSource {
		source = SourceProject
	}
	return assetUnit{
		logicalPath: logicalPath,
		source:      source,
		scope:       scopeFor(rootID),
		assetType:   assetType,
	}
}

func splitLogicalPath(logicalPath string) (RootID, string) {
	root, relative, _ := strings.Cut(logicalPath, "/")
	return RootID(root), relative
}

func incompleteSkillRoot(entry Entry, validSkills map[string]bool) string {
	if entry.FileType == FileSymlink || entry.Type != TypeSkill {
		return ""
	}
	root, containerType := firstPackageRoot(entry.LogicalPath)
	if containerType != TypeSkill || validSkills[root] {
		return ""
	}
	return root
}

func reduceAsset(unit assetUnit, entries []Entry) Asset {
	asset := Asset{
		LogicalPath: unit.logicalPath,
		Source:      unit.source,
		Scope:       unit.scope,
		Type:        unit.assetType,
		Portability: PortabilityAdapterRequired,
		Sensitivity: SensitivityPublic,
		Status:      AssetCandidate,
		Files:       make([]string, 0, len(entries)),
	}
	features := make(map[Feature]bool)
	for _, entry := range entries {
		asset.Files = append(asset.Files, entry.LogicalPath)
		asset.Sensitivity = higherSensitivity(asset.Sensitivity, entry.Sensitivity)
		if entry.ConfigState != "" {
			asset.ConfigState = entry.ConfigState
		}
		for _, feature := range entry.Features {
			features[feature] = true
		}
		switch entry.Status {
		case EntryUnreadable:
			asset.Status = AssetUnreadable
			asset.Portability = PortabilityExcluded
		case EntryExcluded:
			if blocksAsset(entry.ExclusionReason) && asset.Status != AssetUnreadable {
				asset.Status = AssetExcluded
				asset.Portability = PortabilityExcluded
			}
		}
	}
	for feature := range features {
		asset.Features = append(asset.Features, feature)
	}
	sort.Strings(asset.Files)
	sort.Slice(asset.Features, func(i, j int) bool {
		return asset.Features[i] < asset.Features[j]
	})
	return asset
}

func higherSensitivity(left, right Sensitivity) Sensitivity {
	if sensitivityRank(right) > sensitivityRank(left) {
		return right
	}
	return left
}

func sensitivityRank(value Sensitivity) int {
	switch value {
	case SensitivitySecret:
		return 4
	case SensitivitySensitive:
		return 3
	case SensitivityPrivate:
		return 2
	case SensitivityUnknown:
		return 1
	default:
		return 0
	}
}

func rootAfterSegment(logicalPath, segment string) string {
	parts := strings.Split(logicalPath, "/")
	for index, part := range parts {
		if part == segment && index+1 < len(parts) {
			return strings.Join(parts[:index+2], "/")
		}
	}
	return ""
}

func prefixThrough(logicalPath, prefix string) string {
	index := strings.Index(logicalPath, prefix)
	if index < 0 {
		return ""
	}
	return logicalPath[:index+len(prefix)]
}

func blocksAsset(reason ExclusionReason) bool {
	switch reason {
	case ExcludeCache, ExcludeDatabase, ExcludeNativeSession, ExcludeNativeMemory, ExcludeDeviceState:
		return false
	default:
		return true
	}
}
