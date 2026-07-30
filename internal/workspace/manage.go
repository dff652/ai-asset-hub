package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dff652/ai-asset-hub/internal/inventory"
)

var ErrManageBlocked = errors.New("asset library change blocked")

type UpdateOptions struct {
	WorkspaceRoot string
	ManifestPath  string
	Home          string
	Project       string
	Assets        []inventory.Asset
}

type UpdateResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	Ok            bool             `json:"ok"`
	ManifestPath  string           `json:"manifestPath"`
	Updated       []string         `json:"updated"`
	Unchanged     []string         `json:"unchanged"`
	Findings      []ComposeFinding `json:"findings"`
}

type RemoveOptions struct {
	WorkspaceRoot string
	ManifestPath  string
	AssetIDs      []string
}

type RemoveResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	Ok            bool             `json:"ok"`
	ManifestPath  string           `json:"manifestPath"`
	Removed       []string         `json:"removed"`
	Findings      []ComposeFinding `json:"findings"`
}

type replacement struct {
	id       string
	target   string
	staged   string
	previous string
	existed  bool
}

// UpdateAssets replaces selected managed asset contents with the latest files
// from their discovered source. It never writes the source tool directory and
// restores the previous library contents when validation fails.
func UpdateAssets(
	options UpdateOptions, validateManifest func(manifestPath, root string) error,
) (UpdateResult, error) {
	result := UpdateResult{
		SchemaVersion: 1, Kind: "update-assets", Findings: []ComposeFinding{},
	}
	root, manifestPath, document, err := loadManagedWorkspace(
		options.WorkspaceRoot, options.ManifestPath,
	)
	if err != nil {
		return result, err
	}
	result.ManifestPath = manifestPath
	if managed, managedErr := overlapsManagedToolDirectory(root, options.Home, options.Project); managedErr != nil || managed {
		return result, fmt.Errorf("%w: workspace root overlaps a managed tool directory", ErrManageBlocked)
	}
	byID := make(map[string]ManifestAsset, len(document.Assets))
	for _, entry := range document.Assets {
		byID[entry.ID] = entry
	}

	temporary, err := os.MkdirTemp(root, ".aiah-manage-*")
	if err != nil {
		return result, fmt.Errorf("%w: cannot stage updates", ErrManageBlocked)
	}
	defer func() { _ = os.RemoveAll(temporary) }()

	replacements := make([]replacement, 0, len(options.Assets))
	seen := make(map[string]bool, len(options.Assets))
	for index, asset := range options.Assets {
		proposed, findings := proposeEntry(asset)
		if len(findings) > 0 {
			result.Findings = append(result.Findings, findings...)
			continue
		}
		entry, ok := byID[proposed.ID]
		if !ok {
			result.Findings = append(result.Findings, ComposeFinding{
				Code: "asset_not_managed", Path: asset.LogicalPath,
				Message: "asset is not registered in manifest.yaml; add it before updating",
			})
			continue
		}
		if seen[entry.ID] {
			continue
		}
		seen[entry.ID] = true
		if conflict := overlappingAsset(document.Assets, entry.ID, entry.Path); conflict != "" {
			result.Findings = append(result.Findings, ComposeFinding{
				Code: "asset_path_overlap", Path: entry.Path,
				Message: "asset path overlaps " + conflict + "; resolve the manifest by hand",
			})
			continue
		}
		matches, compareFindings := contentMatches(
			asset, entry, root, options.Home, options.Project,
		)
		result.Findings = append(result.Findings, compareFindings...)
		if len(compareFindings) > 0 {
			continue
		}
		if matches {
			result.Unchanged = append(result.Unchanged, entry.ID)
			continue
		}
		files, planFindings := planFiles(asset, entry, root, options.Home, options.Project)
		result.Findings = append(result.Findings, planFindings...)
		if len(planFindings) > 0 {
			continue
		}
		target, safeErr := safeAssetTarget(root, entry.Path)
		if safeErr != nil {
			result.Findings = append(result.Findings, ComposeFinding{
				Code: "unsafe_asset_path", Path: entry.Path, Message: safeErr.Error(),
			})
			continue
		}
		staged := filepath.Join(temporary, "staged", strconv.Itoa(index))
		if err := stageAsset(staged, entry.Path, files); err != nil {
			return result, fmt.Errorf("%w: stage %s: %v", ErrManageBlocked, entry.ID, err)
		}
		replacements = append(replacements, replacement{
			id: entry.ID, target: target, staged: staged,
			previous: filepath.Join(temporary, "previous", strconv.Itoa(index)),
		})
	}
	if len(result.Findings) > 0 {
		return result, fmt.Errorf("%w: resolve reported findings before updating", ErrManageBlocked)
	}
	if len(replacements) == 0 {
		sort.Strings(result.Unchanged)
		result.Ok = true
		return result, nil
	}

	applied := make([]replacement, 0, len(replacements))
	for _, item := range replacements {
		if err := os.MkdirAll(filepath.Dir(item.target), 0o755); err != nil {
			rollbackReplacements(applied)
			return result, fmt.Errorf("%w: prepare target for %s", ErrManageBlocked, item.id)
		}
		if _, err := os.Lstat(item.target); err == nil {
			item.existed = true
			if err := os.MkdirAll(filepath.Dir(item.previous), 0o700); err != nil {
				rollbackReplacements(applied)
				return result, fmt.Errorf("%w: stage previous %s", ErrManageBlocked, item.id)
			}
			if err := os.Rename(item.target, item.previous); err != nil {
				rollbackReplacements(applied)
				return result, fmt.Errorf("%w: preserve previous %s", ErrManageBlocked, item.id)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			rollbackReplacements(applied)
			return result, fmt.Errorf("%w: inspect %s", ErrManageBlocked, item.id)
		}
		if err := os.Rename(item.staged, item.target); err != nil {
			if item.existed {
				_ = os.Rename(item.previous, item.target)
			}
			rollbackReplacements(applied)
			return result, fmt.Errorf("%w: install %s", ErrManageBlocked, item.id)
		}
		applied = append(applied, item)
	}
	if validateManifest != nil {
		if err := validateManifest(manifestPath, root); err != nil {
			rollbackReplacements(applied)
			return result, fmt.Errorf("%w: %s", ErrManageBlocked, err.Error())
		}
	}
	for _, item := range replacements {
		result.Updated = append(result.Updated, item.id)
	}
	sort.Strings(result.Updated)
	sort.Strings(result.Unchanged)
	result.Ok = true
	return result, nil
}

// RemoveAssets removes selected entries and their library content. References
// from profiles are cleaned up, but dependency/conflict references from other
// assets block the operation because silently changing those relations would
// alter user intent.
func RemoveAssets(
	options RemoveOptions, validateManifest func(manifestPath, root string) error,
) (RemoveResult, error) {
	result := RemoveResult{
		SchemaVersion: 1, Kind: "remove-assets", Findings: []ComposeFinding{},
	}
	root, manifestPath, document, err := loadManagedWorkspace(
		options.WorkspaceRoot, options.ManifestPath,
	)
	if err != nil {
		return result, err
	}
	result.ManifestPath = manifestPath
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return result, fmt.Errorf("%w: cannot read manifest", ErrManageBlocked)
	}
	selected := make(map[string]bool, len(options.AssetIDs))
	for _, id := range options.AssetIDs {
		if value := strings.TrimSpace(id); value != "" {
			selected[value] = true
		}
	}
	if len(selected) == 0 {
		return result, fmt.Errorf("%w: no asset ids selected", ErrManageBlocked)
	}
	byID := make(map[string]ManifestAsset, len(document.Assets))
	for _, entry := range document.Assets {
		byID[entry.ID] = entry
	}
	for id := range selected {
		if _, ok := byID[id]; !ok {
			result.Findings = append(result.Findings, ComposeFinding{
				Code: "asset_not_managed", Path: id, Message: "manifest does not declare this asset",
			})
		}
	}
	for _, entry := range document.Assets {
		if selected[entry.ID] {
			continue
		}
		for _, reference := range append(append([]string{}, entry.Dependencies...), entry.Conflicts...) {
			if selected[reference] {
				result.Findings = append(result.Findings, ComposeFinding{
					Code: "asset_still_referenced", Path: entry.ID,
					Message: entry.ID + " still references " + reference,
				})
			}
		}
		for id := range selected {
			if pathsOverlap(entry.Path, byID[id].Path) {
				result.Findings = append(result.Findings, ComposeFinding{
					Code: "asset_path_overlap", Path: byID[id].Path,
					Message: "selected path overlaps remaining asset " + entry.ID,
				})
			}
		}
	}
	if len(result.Findings) > 0 {
		return result, fmt.Errorf("%w: resolve reported findings before removing", ErrManageBlocked)
	}
	documentBytes, err := renderRemovedManifest(raw, selected)
	if err != nil {
		return result, err
	}

	temporary, err := os.MkdirTemp(root, ".aiah-manage-*")
	if err != nil {
		return result, fmt.Errorf("%w: cannot stage removal", ErrManageBlocked)
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	moved := make([]replacement, 0, len(selected))
	index := 0
	for _, entry := range document.Assets {
		if !selected[entry.ID] {
			continue
		}
		target, safeErr := safeAssetTarget(root, entry.Path)
		if safeErr != nil {
			rollbackReplacements(moved)
			return result, fmt.Errorf("%w: unsafe path for %s: %v", ErrManageBlocked, entry.ID, safeErr)
		}
		item := replacement{
			id: entry.ID, target: target,
			previous: filepath.Join(temporary, "previous", strconv.Itoa(index)),
		}
		index++
		if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
			moved = append(moved, item)
			continue
		} else if err != nil {
			rollbackReplacements(moved)
			return result, fmt.Errorf("%w: inspect %s", ErrManageBlocked, entry.ID)
		}
		item.existed = true
		if err := os.MkdirAll(filepath.Dir(item.previous), 0o700); err != nil {
			rollbackReplacements(moved)
			return result, fmt.Errorf("%w: stage %s", ErrManageBlocked, entry.ID)
		}
		if err := os.Rename(target, item.previous); err != nil {
			rollbackReplacements(moved)
			return result, fmt.Errorf("%w: stage %s", ErrManageBlocked, entry.ID)
		}
		moved = append(moved, item)
	}
	if err := commitManifest(manifestPath, root, documentBytes, validateManifest); err != nil {
		rollbackReplacements(moved)
		return result, fmt.Errorf("%w: %s", ErrManageBlocked, err.Error())
	}
	for id := range selected {
		result.Removed = append(result.Removed, id)
	}
	sort.Strings(result.Removed)
	result.Ok = true
	return result, nil
}

func loadManagedWorkspace(workspaceRoot, candidateManifest string) (string, string, Document, error) {
	root, err := requireDirectory(workspaceRoot)
	if err != nil {
		return "", "", Document{}, fmt.Errorf("%w: workspace root is not an accessible directory", ErrManageBlocked)
	}
	manifestPath := candidateManifest
	if strings.TrimSpace(manifestPath) == "" {
		manifestPath = filepath.Join(root, "manifest.yaml")
	}
	if !withinRoot(root, manifestPath) || strings.EqualFold(filepath.Ext(manifestPath), ".json") {
		return "", "", Document{}, fmt.Errorf("%w: use a YAML manifest inside the workspace", ErrManageBlocked)
	}
	document, normalized, err := LoadManifest(manifestPath)
	if err != nil {
		return "", "", Document{}, fmt.Errorf("%w: manifest is not readable: %v", ErrManageBlocked, err)
	}
	if err := ValidateManifestSchema(normalized); err != nil {
		return "", "", Document{}, fmt.Errorf("%w: manifest schema is invalid: %v", ErrManageBlocked, err)
	}
	return root, manifestPath, document, nil
}

func safeAssetTarget(root, relative string) (string, error) {
	if !SafeRelativePath(relative) {
		return "", errors.New("manifest path is not a safe relative path")
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	resolved, err := resolveBoundaryPath(target)
	if err != nil || !Within(root, resolved) {
		return "", errors.New("manifest path escapes the workspace")
	}
	if info, statErr := os.Lstat(target); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("asset path is a symbolic link")
	}
	return target, nil
}

func pathsOverlap(left, right string) bool {
	left = strings.Trim(filepath.ToSlash(filepath.Clean(left)), "/")
	right = strings.Trim(filepath.ToSlash(filepath.Clean(right)), "/")
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func overlappingAsset(entries []ManifestAsset, currentID, currentPath string) string {
	for _, entry := range entries {
		if entry.ID != currentID && pathsOverlap(entry.Path, currentPath) {
			return entry.ID
		}
	}
	return ""
}

func stageAsset(staged, entryPath string, files []plannedFile) error {
	for _, file := range files {
		relative := strings.TrimPrefix(filepath.ToSlash(file.rel), strings.TrimSuffix(entryPath, "/")+"/")
		destination := staged
		if relative != filepath.ToSlash(file.rel) {
			destination = filepath.Join(staged, filepath.FromSlash(relative))
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		body, err := os.ReadFile(file.source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destination, body, file.mode); err != nil {
			return err
		}
	}
	return nil
}

func rollbackReplacements(items []replacement) {
	for index := len(items) - 1; index >= 0; index-- {
		item := items[index]
		_ = os.RemoveAll(item.target)
		if item.existed {
			_ = os.MkdirAll(filepath.Dir(item.target), 0o755)
			_ = os.Rename(item.previous, item.target)
		}
	}
}

func renderRemovedManifest(raw []byte, selected map[string]bool) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("%w: manifest does not parse", ErrManageBlocked)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 ||
		root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: manifest is not a mapping", ErrManageBlocked)
	}
	body := root.Content[0]
	assets := mappingValue(body, "assets")
	profiles := mappingValue(body, "profiles")
	if assets == nil || assets.Kind != yaml.SequenceNode ||
		profiles == nil || profiles.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: manifest assets or profiles have an unsupported shape", ErrManageBlocked)
	}
	filteredAssets := make([]*yaml.Node, 0, len(assets.Content))
	for _, entry := range assets.Content {
		id := mappingValue(entry, "id")
		if id == nil || !selected[id.Value] {
			filteredAssets = append(filteredAssets, entry)
		}
	}
	assets.Content = filteredAssets
	for index := 1; index < len(profiles.Content); index += 2 {
		profile := profiles.Content[index]
		if profile.Kind != yaml.MappingNode {
			continue
		}
		for _, field := range []string{"include", "exclude"} {
			values := mappingValue(profile, field)
			if values == nil || values.Kind != yaml.SequenceNode {
				continue
			}
			filtered := make([]*yaml.Node, 0, len(values.Content))
			for _, value := range values.Content {
				if !selected[value.Value] {
					filtered = append(filtered, value)
				}
			}
			values.Content = filtered
		}
	}
	return encodeDocument(&root)
}
