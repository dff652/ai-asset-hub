package build

import (
	"errors"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

var (
	// ErrInvalidOptions means build flags or paths are unusable.
	ErrInvalidOptions = errors.New("invalid build options")
)

// Options configures a deterministic package build.
type Options struct {
	Manifest string
	Root     string
	Profile  string
	// OutDir receives package artifacts. Required.
	OutDir string
}

// Build validates the workspace once, selects a profile, and publishes a
// deterministic package atomically. It never modifies the source workspace.
func Build(options Options) (Report, error) {
	if options.Manifest == "" || options.Profile == "" || options.OutDir == "" {
		return Report{}, ErrInvalidOptions
	}

	report := Report{
		SchemaVersion: 1,
		Kind:          "build",
		ProducedBy:    version.ProducedBy(),
		Profile:       options.Profile,
		Findings:      make([]Finding, 0),
		Capabilities:  make(map[string][]string),
		Summary:       Summary{Targets: []string{}},
	}

	inspection, err := workspace.Inspect(workspace.Options{
		Manifest:      options.Manifest,
		Root:          options.Root,
		CollectBodies: true,
	})
	if err != nil {
		if errors.Is(err, workspace.ErrInvalidOptions) {
			return Report{}, ErrInvalidOptions
		}
		return Report{}, err
	}

	errorCount := 0
	for _, finding := range inspection.Findings {
		if finding.Severity == workspace.SeverityError {
			errorCount++
		}
	}
	report.Validation = &ValidationSummary{Ok: errorCount == 0, ErrorCount: errorCount}
	if errorCount > 0 {
		report.Findings = append(report.Findings, Finding{
			Code:     codeValidationFailed,
			Severity: workspace.SeverityError,
			Message:  "Workspace validation failed; package was not written.",
			Paths:    []string{"manifest"},
		})
		// Surface the underlying workspace findings for operators.
		report.Findings = append(report.Findings, inspection.Findings...)
		return finish(report), nil
	}

	document := inspection.Document
	profile, ok := document.Profiles[options.Profile]
	if !ok {
		report.Findings = append(report.Findings, Finding{
			Code:     codeUnknownProfile,
			Severity: workspace.SeverityError,
			Message:  "Profile is not defined in the manifest.",
			Paths:    []string{"profiles/" + options.Profile},
		})
		return finish(report), nil
	}

	selected, findings := selectAssets(document, profile)
	report.Findings = append(report.Findings, findings...)
	if workspace.HasError(report.Findings) {
		return finish(report), nil
	}

	pkgAssets := make([]PackageAsset, 0, len(selected))
	lockFiles := make([]LockEntry, 0)
	filePayloads := make(map[string][]byte)
	targetsSet := make(map[string]bool)
	capSet := make(map[string]map[string]bool)

	for _, asset := range selected {
		for _, target := range asset.Targets {
			targetsSet[target] = true
			if capSet[target] == nil {
				capSet[target] = make(map[string]bool)
			}
			capSet[target][asset.Type] = true
		}

		filePaths := inspection.AssetFiles[asset.ID]
		pkgFiles := make([]PackageFile, 0, len(filePaths))
		for _, path := range filePaths {
			blob, ok := inspection.Files[path]
			if !ok {
				continue
			}
			pkgFiles = append(pkgFiles, PackageFile{Path: blob.Path, SHA256: blob.SHA256})
			lockFiles = append(lockFiles, LockEntry{Path: blob.Path, SHA256: blob.SHA256})
			filePayloads[blob.Path] = blob.Body
		}
		pkgAssets = append(pkgAssets, PackageAsset{
			ID:          asset.ID,
			Type:        asset.Type,
			Path:        asset.Path,
			Targets:     asset.Targets,
			Scope:       asset.Scope,
			Portability: asset.Portability,
			Sensitivity: asset.Sensitivity,
			Files:       pkgFiles,
		})
	}

	targets := sortedKeys(targetsSet)
	for target, types := range capSet {
		list := make([]string, 0, len(types))
		for typ := range types {
			list = append(list, typ)
		}
		sort.Strings(list)
		report.Capabilities[target] = list
	}

	artifacts, err := buildArtifacts(document, options.Profile, pkgAssets, lockFiles, filePayloads, targets)
	if err != nil {
		report.Findings = append(report.Findings, ioFinding("Cannot create package archive."))
		return finish(report), nil
	}
	if err := publishArtifacts(options.OutDir, artifacts); err != nil {
		report.Findings = append(report.Findings, ioFinding("Cannot write package artifacts."))
		return finish(report), nil
	}

	report.Package = &PackageInfo{
		Name:     document.Name,
		Version:  document.Version,
		Archive:  artifacts.ArchiveName,
		Manifest: artifacts.ManifestName,
		Lock:     artifacts.LockName,
		SHA256:   artifacts.SHAName,
		Digest:   artifacts.Digest,
	}
	report.Summary = Summary{
		AssetCount: len(pkgAssets),
		FileCount:  len(lockFiles),
		Targets:    targets,
	}
	return finish(report), nil
}

func finish(report Report) Report {
	sort.SliceStable(report.Findings, func(i, j int) bool {
		left, right := report.Findings[i], report.Findings[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return strings.Join(left.Paths, "\x00") < strings.Join(right.Paths, "\x00")
	})
	report.Ok = !workspace.HasError(report.Findings)
	if report.Summary.Targets == nil {
		report.Summary.Targets = []string{}
	}
	return report
}

func ioFinding(message string) Finding {
	return Finding{
		Code:     codeBuildIO,
		Severity: workspace.SeverityError,
		Message:  message,
		Paths:    []string{"out"},
	}
}
