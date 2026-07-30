package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/inventory"
)

// LibraryState is the relationship between one discovered source asset and
// the editable copy in the asset library.
type LibraryState string

const (
	LibraryUnmanaged     LibraryState = "unmanaged"
	LibraryManaged       LibraryState = "managed"
	LibrarySourceChanged LibraryState = "source-changed"
	LibraryOnly          LibraryState = "library-only"
	LibraryBlocked       LibraryState = "blocked"
)

type CatalogOptions struct {
	WorkspaceRoot string
	ManifestPath  string
	Home          string
	Project       string
	Assets        []inventory.Asset
}

type CatalogSummary struct {
	Unmanaged     int `json:"unmanaged"`
	Managed       int `json:"managed"`
	SourceChanged int `json:"sourceChanged"`
	LibraryOnly   int `json:"libraryOnly"`
	Blocked       int `json:"blocked"`
}

type CatalogItem struct {
	ID          string           `json:"id,omitempty"`
	LogicalPath string           `json:"logicalPath,omitempty"`
	LibraryPath string           `json:"libraryPath,omitempty"`
	Type        string           `json:"type"`
	Source      inventory.Source `json:"source,omitempty"`
	State       LibraryState     `json:"state"`
	Targets     []string         `json:"targets,omitempty"`
	Asset       *inventory.Asset `json:"-"`
	Manifest    *ManifestAsset   `json:"-"`
	Findings    []ComposeFinding `json:"findings,omitempty"`
}

type CatalogReport struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	Ok            bool             `json:"ok"`
	ManifestPath  string           `json:"manifestPath"`
	Summary       CatalogSummary   `json:"summary"`
	Items         []CatalogItem    `json:"items"`
	Findings      []ComposeFinding `json:"findings"`
}

// Catalog computes a unified, read-only view of discovered and managed assets.
// It never writes the workspace or any tool directory.
func Catalog(options CatalogOptions) (CatalogReport, error) {
	report := CatalogReport{
		SchemaVersion: 1,
		Kind:          "asset-catalog",
		Findings:      []ComposeFinding{},
		Items:         []CatalogItem{},
	}
	root, err := requireDirectory(options.WorkspaceRoot)
	if err != nil {
		return report, fmt.Errorf("%w: workspace root is not an accessible directory", ErrInvalidOptions)
	}
	manifestPath := options.ManifestPath
	if strings.TrimSpace(manifestPath) == "" {
		manifestPath = filepath.Join(root, "manifest.yaml")
	}
	if !withinRoot(root, manifestPath) {
		return report, fmt.Errorf("%w: manifest is outside the workspace", ErrInvalidOptions)
	}
	report.ManifestPath = manifestPath

	document := Document{}
	if _, err := os.Stat(manifestPath); err == nil {
		var normalized any
		document, normalized, err = LoadManifest(manifestPath)
		if err != nil {
			return report, err
		}
		if err := ValidateManifestSchema(normalized); err != nil {
			return report, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return report, fmt.Errorf("inspect manifest: %w", err)
	}

	manifestByID := make(map[string]ManifestAsset, len(document.Assets))
	for _, entry := range document.Assets {
		manifestByID[entry.ID] = entry
	}
	seen := make(map[string]bool, len(document.Assets))

	for index := range options.Assets {
		asset := options.Assets[index]
		entry, findings := proposeEntry(asset)
		item := CatalogItem{
			LogicalPath: asset.LogicalPath,
			Type:        string(asset.Type),
			Source:      asset.Source,
			State:       LibraryBlocked,
			Asset:       &asset,
			Findings:    findings,
		}
		if len(findings) > 0 {
			report.Items = append(report.Items, item)
			report.Findings = append(report.Findings, findings...)
			report.Summary.Blocked++
			continue
		}
		item.ID = entry.ID
		managed, ok := manifestByID[entry.ID]
		if !ok {
			item.State = LibraryUnmanaged
			item.LibraryPath = entry.Path
			item.Targets = entry.Targets
			report.Summary.Unmanaged++
			report.Items = append(report.Items, item)
			continue
		}
		seen[entry.ID] = true
		item.LibraryPath = managed.Path
		item.Targets = append([]string(nil), managed.Targets...)
		item.Manifest = manifestPointer(managed)
		matches, compareFindings := contentMatches(asset, managed, root, options.Home, options.Project)
		item.Findings = append(item.Findings, compareFindings...)
		report.Findings = append(report.Findings, compareFindings...)
		if len(compareFindings) > 0 {
			item.State = LibraryBlocked
			report.Summary.Blocked++
		} else if matches {
			item.State = LibraryManaged
			report.Summary.Managed++
		} else {
			item.State = LibrarySourceChanged
			report.Summary.SourceChanged++
		}
		report.Items = append(report.Items, item)
	}

	for _, entry := range document.Assets {
		if seen[entry.ID] {
			continue
		}
		report.Items = append(report.Items, CatalogItem{
			ID:          entry.ID,
			LibraryPath: entry.Path,
			Type:        entry.Type,
			State:       LibraryOnly,
			Targets:     append([]string(nil), entry.Targets...),
			Manifest:    manifestPointer(entry),
		})
		report.Summary.LibraryOnly++
	}
	sort.SliceStable(report.Items, func(i, j int) bool {
		left, right := report.Items[i], report.Items[j]
		if left.State != right.State {
			return left.State < right.State
		}
		if left.LogicalPath != right.LogicalPath {
			return left.LogicalPath < right.LogicalPath
		}
		return left.ID < right.ID
	})
	report.Ok = true
	return report, nil
}

func manifestPointer(entry ManifestAsset) *ManifestAsset {
	copy := entry
	return &copy
}

func contentMatches(
	asset inventory.Asset, entry ManifestAsset, root, home, project string,
) (bool, []ComposeFinding) {
	files, findings := planFiles(asset, entry, root, home, project)
	if len(findings) > 0 {
		return false, findings
	}
	expected := make(map[string]plannedFile, len(files))
	for _, file := range files {
		expected[filepath.Clean(file.target)] = file
	}

	target := filepath.Join(root, filepath.FromSlash(entry.Path))
	actual := make(map[string]bool, len(expected))
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, []ComposeFinding{{
			Code: "library_asset_unreadable", Path: entry.Path, Message: err.Error(),
		}}
	}
	if info.Mode().IsRegular() {
		actual[filepath.Clean(target)] = true
	} else if info.IsDir() {
		err = filepath.WalkDir(target, func(candidate string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if candidate == target || entry.IsDir() {
				return nil
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("non-regular file %s", candidate)
			}
			actual[filepath.Clean(candidate)] = true
			return nil
		})
		if err != nil {
			return false, []ComposeFinding{{
				Code: "library_asset_unreadable", Path: entry.Path, Message: err.Error(),
			}}
		}
	} else {
		return false, []ComposeFinding{{
			Code: "library_asset_not_regular", Path: entry.Path,
			Message: "asset library content must be a regular file or directory",
		}}
	}

	if len(actual) != len(expected) {
		return false, nil
	}
	for targetPath, planned := range expected {
		if !actual[targetPath] {
			return false, nil
		}
		sourceBody, sourceErr := os.ReadFile(planned.source)
		targetBody, targetErr := os.ReadFile(targetPath)
		targetInfo, statErr := os.Stat(targetPath)
		if sourceErr != nil || targetErr != nil || statErr != nil ||
			!bytes.Equal(sourceBody, targetBody) || targetInfo.Mode().Perm() != planned.mode.Perm() {
			return false, nil
		}
	}
	return true, nil
}
