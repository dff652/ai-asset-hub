package inventory

import (
	"sort"
	"strings"
)

func deriveFindings(issues []detectedIssue, entries []Entry, assets []Asset) []Finding {
	findings := make([]Finding, 0, len(issues))
	for _, issue := range issues {
		findings = append(findings, findingFor(issue.code, issue.path))
	}

	hasClaude := hasAsset(assets, "project/CLAUDE.md", AssetCandidate)
	hasAgents := hasAsset(assets, "project/AGENTS.md", AssetCandidate)
	hasFallback := hasFeature(entries, FeatureCodexProjectDocFallback)
	if hasClaude && hasAgents {
		message := "Project has both CLAUDE.md and AGENTS.md; verify which file is the intended rule source."
		severity := SeverityInfo
		if hasFallback {
			message = "Project has both CLAUDE.md and AGENTS.md; Codex may shadow the configured CLAUDE.md fallback."
			severity = SeverityWarning
		}
		findings = append(findings, Finding{
			Code: FindingRulesShadowing, Severity: severity, Message: message,
			Paths: []string{"project/CLAUDE.md", "project/AGENTS.md"},
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return strings.Join(findings[i].Paths, "\x00") < strings.Join(findings[j].Paths, "\x00")
	})
	return findings
}

func findingFor(code FindingCode, logicalPath string) Finding {
	finding := Finding{Code: code, Paths: []string{logicalPath}}
	switch code {
	case FindingBrokenSymlink:
		finding.Severity = SeverityWarning
		finding.Message = "Broken symlink was not followed."
	case FindingIncompleteSkill:
		finding.Severity = SeverityWarning
		finding.Message = "Skill directory has files but no regular SKILL.md manifest."
	case FindingInvalidConfig:
		finding.Severity = SeverityWarning
		finding.Message = "Configuration is invalid; parser details were suppressed."
	case FindingLargeFile:
		finding.Severity = SeverityWarning
		finding.Message = "File exceeds the Phase 0 scan limit and was not read."
	case FindingSuspectedSecret:
		finding.Severity = SeverityError
		finding.Message = "Possible secret detected; content and hash were suppressed."
	case FindingSymlinkEscape:
		finding.Severity = SeverityError
		finding.Message = "Symlink escapes its logical root and was not followed."
	case FindingSymlinkedAsset:
		finding.Severity = SeverityWarning
		finding.Message = "Asset path is a symlink; it was not followed, so its content is absent from this inventory."
	case FindingUnreadable:
		finding.Severity = SeverityWarning
		finding.Message = "Path is unreadable; content was not inspected."
	}
	return finding
}

func summarize(entries []Entry, assets []Asset, findings []Finding, projectScanned bool) Summary {
	summary := Summary{CandidateByType: make(map[AssetType]int)}
	for _, asset := range assets {
		summary.TotalAssets++
		switch asset.Status {
		case AssetCandidate:
			summary.CandidateAssets++
			summary.CandidateByType[asset.Type]++
		case AssetExcluded:
			summary.ExcludedAssets++
		case AssetUnreadable:
			summary.UnreadableAssets++
		}
	}
	for _, entry := range entries {
		summary.TotalEntries++
		switch entry.Status {
		case EntryIncluded:
			summary.IncludedEntries++
		case EntryReported:
			summary.ReportedEntries++
		case EntryExcluded:
			summary.ExcludedEntries++
		case EntryUnreadable:
			summary.UnreadableEntries++
		}
	}

	if !projectScanned {
		summary.ProjectMigration = ProjectNotScanned
	} else {
		summary.ProjectMigration = projectMigration(assets, entries, findings)
	}
	summary.DeviceMigration = deviceMigration(assets)
	return summary
}

func deviceMigration(assets []Asset) DeviceMigration {
	blocked := false
	for _, asset := range assets {
		if asset.Scope != ScopeGlobal {
			continue
		}
		if asset.Status == AssetCandidate {
			return DeviceInventoryOnly
		}
		blocked = true
	}
	if blocked {
		return DeviceBlocked
	}
	return DeviceNoAssets
}

func projectMigration(assets []Asset, entries []Entry, findings []Finding) ProjectMigration {
	candidateSources := make(map[Source]bool)
	candidateCount := 0
	for _, asset := range assets {
		if asset.Scope == ScopeProject && asset.Status == AssetCandidate {
			candidateCount++
			candidateSources[asset.Source] = true
		}
	}
	if candidateCount == 0 {
		return ProjectNoAssets
	}
	if hasFinding(findings, FindingRulesShadowing) &&
		hasFeature(entries, FeatureCodexProjectDocFallback) {
		return ProjectConflicted
	}
	if (hasAsset(assets, "project/CLAUDE.md", AssetCandidate) &&
		hasFeature(entries, FeatureCodexProjectDocFallback)) ||
		(candidateSources[SourceClaude] && candidateSources[SourceCodex]) {
		return ProjectDualToolConfigured
	}
	return ProjectPartial
}

func hasAsset(assets []Asset, logicalPath string, status AssetStatus) bool {
	for _, asset := range assets {
		if asset.LogicalPath == logicalPath && asset.Status == status {
			return true
		}
	}
	return false
}

func hasFeature(entries []Entry, wanted Feature) bool {
	for _, entry := range entries {
		for _, feature := range entry.Features {
			if feature == wanted {
				return true
			}
		}
	}
	return false
}

func hasFinding(findings []Finding, wanted FindingCode) bool {
	for _, finding := range findings {
		if finding.Code == wanted {
			return true
		}
	}
	return false
}
