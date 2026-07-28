package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dff652/ai-asset-hub/internal/inventory"
)

// Compose turns selected inventory assets into workspace content: it copies the
// asset files under <workspace>/assets/... and registers them in manifest.yaml.
//
// It writes the workspace and nothing else. The tool directories it reads from
// (.claude, .codex, .grok) are never modified -- see ADR-0006 §2.

// ErrComposeBlocked means nothing was written because the request could not be
// carried out safely.
var ErrComposeBlocked = errors.New("compose blocked")

// assetIDPattern mirrors the manifest schema; an id we cannot derive cleanly is
// a skip, not a guess.
var assetIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)

// manifestContainer maps a manifest asset type to its workspace directory. The
// names match what internal/adapter expects to find under assets/.
var manifestContainer = map[string]string{
	"skill":  "skills",
	"rules":  "rules",
	"agent":  "agents",
	"hook":   "hooks",
	"mcp":    "mcp",
	"memory": "memory",
}

// composableType maps an inventory type to a manifest type. Anything absent is
// not a migratable asset (config, credentials, sessions, caches, device state).
var composableType = map[inventory.AssetType]string{
	inventory.TypeSkill:  "skill",
	inventory.TypeRules:  "rules",
	inventory.TypeAgent:  "agent",
	inventory.TypeHook:   "hook",
	inventory.TypeMemory: "memory",
}

// ComposeOptions describes one compose request.
type ComposeOptions struct {
	// WorkspaceRoot is the only directory Compose may write.
	WorkspaceRoot string
	// ManifestPath defaults to <WorkspaceRoot>/manifest.yaml.
	ManifestPath string
	// Home and Project resolve the logical paths carried by inventory assets.
	Home    string
	Project string
	// Assets are the selected inventory assets.
	Assets []inventory.Asset
	// Profile receives the new asset ids; defaults to "personal".
	Profile string
	// Name and Version seed a manifest that does not exist yet.
	Name    string
	Version string
}

// ComposeResult reports what reached the workspace.
type ComposeResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	Ok            bool             `json:"ok"`
	ManifestPath  string           `json:"manifestPath"`
	Created       []string         `json:"created"`
	Registered    []string         `json:"registered"`
	Skipped       []string         `json:"skipped"`
	Findings      []ComposeFinding `json:"findings"`
}

// ComposeFinding explains one asset that was skipped or one condition the user
// has to resolve by hand.
type ComposeFinding struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type plannedFile struct {
	source string // absolute source path
	target string // absolute destination inside the workspace
	rel    string // workspace-relative destination, for reporting
	mode   fs.FileMode
}

type plannedAsset struct {
	entry ManifestAsset
	files []plannedFile
}

// Compose copies the selected assets into the workspace and registers them.
//
// Ordering is transactional (ADR-0006 §5): plan, copy, write a temporary
// manifest, validate it via the caller's validator, then rename. If validation
// fails, the temporary manifest and every file this call created are removed,
// leaving pre-existing workspace content untouched.
func Compose(options ComposeOptions, validateManifest func(manifestPath, root string) error) (ComposeResult, error) {
	result := ComposeResult{SchemaVersion: 1, Kind: "compose", Findings: []ComposeFinding{}}

	root, err := requireDirectory(options.WorkspaceRoot)
	if err != nil {
		return result, fmt.Errorf("%w: workspace root is not an accessible directory", ErrComposeBlocked)
	}
	manifestPath := options.ManifestPath
	if strings.TrimSpace(manifestPath) == "" {
		manifestPath = filepath.Join(root, "manifest.yaml")
	}
	if !withinRoot(root, manifestPath) {
		return result, fmt.Errorf("%w: manifest is outside the workspace", ErrComposeBlocked)
	}
	result.ManifestPath = manifestPath

	profile := strings.TrimSpace(options.Profile)
	if profile == "" {
		profile = "personal"
	}

	existing, existingIDs, err := loadExistingManifest(manifestPath)
	if err != nil {
		return result, err
	}

	planned := make([]plannedAsset, 0, len(options.Assets))
	for _, asset := range options.Assets {
		entry, findings := proposeEntry(asset)
		result.Findings = append(result.Findings, findings...)
		if len(findings) > 0 {
			result.Skipped = append(result.Skipped, asset.LogicalPath)
			continue
		}
		if existingIDs[entry.ID] {
			result.Findings = append(result.Findings, ComposeFinding{
				Code:    "asset_already_registered",
				Path:    asset.LogicalPath,
				Message: "manifest already declares " + entry.ID + "; edit it by hand to change the entry",
			})
			result.Skipped = append(result.Skipped, asset.LogicalPath)
			continue
		}
		files, findings := planFiles(asset, entry, root, options.Home, options.Project)
		result.Findings = append(result.Findings, findings...)
		if len(findings) > 0 {
			result.Skipped = append(result.Skipped, asset.LogicalPath)
			continue
		}
		planned = append(planned, plannedAsset{entry: entry, files: files})
		existingIDs[entry.ID] = true
	}

	if len(planned) == 0 {
		result.Ok = len(result.Findings) == 0
		return result, nil
	}

	document, err := renderManifest(existing, manifestPath, planned, profile, options)
	if err != nil {
		return result, err
	}

	writes, err := copyPlanned(root, planned)
	if err != nil {
		rollback(writes)
		return result, err
	}

	if err := commitManifest(manifestPath, root, document, validateManifest); err != nil {
		rollback(writes)
		return result, err
	}

	for _, file := range writes.files {
		result.Created = append(result.Created, file.rel)
	}
	for _, item := range planned {
		result.Registered = append(result.Registered, item.entry.ID)
	}
	sort.Strings(result.Created)
	sort.Strings(result.Registered)
	result.Ok = true
	return result, nil
}

// proposeEntry derives a manifest entry. Every unmappable attribute is a skip
// with a finding rather than a default that quietly changes what gets deployed.
func proposeEntry(asset inventory.Asset) (ManifestAsset, []ComposeFinding) {
	reject := func(code, message string) (ManifestAsset, []ComposeFinding) {
		return ManifestAsset{}, []ComposeFinding{{Code: code, Path: asset.LogicalPath, Message: message}}
	}

	if asset.Status != inventory.AssetCandidate {
		return reject("asset_not_candidate", "only candidate assets can be registered")
	}
	if asset.Sensitivity == inventory.SensitivitySecret {
		return reject("asset_secret", "assets holding secrets are never packaged")
	}
	if asset.Scope == inventory.ScopeDevicePrivate {
		return reject("asset_device_scope", "device-scoped assets are never applied, so registering them is meaningless")
	}
	manifestType, ok := composableType[asset.Type]
	if !ok {
		return reject("asset_type_not_composable", string(asset.Type)+" is not a migratable asset type")
	}
	targets := targetsFor(asset.Source)
	if len(targets) == 0 {
		return reject("asset_targets_unknown", "cannot derive targets for source "+string(asset.Source))
	}
	scope := "global"
	if asset.Scope == inventory.ScopeProject {
		scope = "project"
	}
	name := path.Base(asset.LogicalPath)
	id := deriveID(manifestType, name)
	if !assetIDPattern.MatchString(id) {
		return reject("asset_id_underivable", "cannot derive a valid manifest id from "+name)
	}

	return ManifestAsset{
		ID:          id,
		Type:        manifestType,
		Path:        path.Join("assets", manifestContainer[manifestType], name),
		Targets:     targets,
		Scope:       scope,
		Portability: composedPortability,
		Sensitivity: sensitivityFor(asset.Sensitivity),
	}, nil
}

func targetsFor(source inventory.Source) []string {
	switch source {
	case inventory.SourceClaude:
		return []string{"claude"}
	case inventory.SourceCodex:
		return []string{"codex"}
	case inventory.SourceGrok:
		return []string{"grok"}
	case inventory.SourceShared:
		return []string{"claude", "codex", "grok"}
	default:
		return nil
	}
}

// composedPortability is always adapter-required.
//
// Inventory only ever reports adapter-required, excluded or unknown: "portable"
// is reserved for manifest assets that validate/build have confirmed (see
// docs/asset-model.md §3). Proposing adapter-required is also the conservative
// half of that choice, since it routes the asset through the adapter instead of
// installing it verbatim into every target. Promoting an entry to portable is a
// deliberate edit the user makes in the manifest.
const composedPortability = "adapter-required"

// sensitivityFor defaults unknown to private rather than public.
func sensitivityFor(value inventory.Sensitivity) string {
	switch value {
	case inventory.SensitivityPublic:
		return "public"
	case inventory.SensitivitySensitive:
		return "sensitive"
	default:
		return "private"
	}
}

func deriveID(manifestType, name string) string {
	slug := strings.ToLower(name)
	slug = strings.TrimSuffix(slug, ".md")
	slug = strings.TrimSuffix(slug, ".json")
	slug = strings.TrimSuffix(slug, ".sh")
	var builder strings.Builder
	previousSeparator := true
	for _, char := range slug {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			previousSeparator = false
		case !previousSeparator:
			builder.WriteRune('-')
			previousSeparator = true
		}
	}
	slug = strings.Trim(builder.String(), "-")
	if slug == "" {
		return ""
	}
	return manifestType + "." + slug
}

// planFiles resolves the asset's logical file list to real paths and their
// destinations. A logical path that does not resolve is a skip, never a guess.
func planFiles(
	asset inventory.Asset, entry ManifestAsset, root, home, project string,
) ([]plannedFile, []ComposeFinding) {
	if _, ok := resolveLogical(asset.LogicalPath, home, project); !ok {
		return nil, []ComposeFinding{{
			Code: "asset_root_unresolved", Path: asset.LogicalPath,
			Message: "cannot resolve the logical path against the scanned roots",
		}}
	}

	sources := asset.Files
	if len(sources) == 0 {
		sources = []string{asset.LogicalPath}
	}
	files := make([]plannedFile, 0, len(sources))
	for _, logical := range sources {
		source, ok := resolveLogical(logical, home, project)
		if !ok {
			return nil, []ComposeFinding{{
				Code: "asset_file_unresolved", Path: logical,
				Message: "cannot resolve the logical path against the scanned roots",
			}}
		}
		info, err := os.Lstat(source)
		if err != nil {
			// Distinct from the non-regular case below: a file the scan
			// reported but that is no longer there means a stale report, and
			// saying "not a regular file" would send the user looking for the
			// wrong problem.
			return nil, []ComposeFinding{{
				Code: "asset_file_missing", Path: logical,
				Message: "the scan reported this file but it is no longer readable; rescan with r",
			}}
		}
		if !info.Mode().IsRegular() {
			// Symlinks and specials are never packable; the scanner already
			// reports them, so refuse rather than dereference.
			return nil, []ComposeFinding{{
				Code: "asset_file_not_regular", Path: logical,
				Message: "only regular files can be composed into the workspace",
			}}
		}

		relative := ""
		if logical != asset.LogicalPath {
			relative = strings.TrimPrefix(logical, asset.LogicalPath+"/")
			if relative == logical {
				return nil, []ComposeFinding{{
					Code: "asset_file_outside_asset", Path: logical,
					Message: "file is not under its own asset root",
				}}
			}
		}

		targetRel := entry.Path
		if relative != "" {
			targetRel = path.Join(entry.Path, relative)
		}
		target := filepath.Join(root, filepath.FromSlash(targetRel))
		if !withinRoot(root, target) {
			return nil, []ComposeFinding{{
				Code: "workspace_path_escape", Path: targetRel,
				Message: "refusing to write outside the workspace",
			}}
		}
		files = append(files, plannedFile{
			source: source, target: target, rel: targetRel, mode: info.Mode().Perm(),
		})
	}
	return files, nil
}

func resolveLogical(logical, home, project string) (string, bool) {
	switch {
	case strings.HasPrefix(logical, "home/"):
		if home == "" {
			return "", false
		}
		return filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(logical, "home/"))), true
	case strings.HasPrefix(logical, "project/"):
		if project == "" {
			return "", false
		}
		return filepath.Join(project, filepath.FromSlash(strings.TrimPrefix(logical, "project/"))), true
	default:
		return "", false
	}
}

// composeWrites records exactly what one call put on disk, so a rollback can
// undo precisely that and nothing else.
type composeWrites struct {
	files       []plannedFile
	directories []string // in creation order, deepest last
}

// copyPlanned writes the planned files create-only.
func copyPlanned(root string, planned []plannedAsset) (composeWrites, error) {
	writes := composeWrites{files: make([]plannedFile, 0)}
	for _, item := range planned {
		for _, file := range item.files {
			if _, err := os.Lstat(file.target); err == nil {
				// create-only: an existing workspace file is the user's, and a
				// tick in a list is not consent to overwrite it.
				continue
			}
			if err := makeDirectories(root, filepath.Dir(file.target), &writes); err != nil {
				return writes, fmt.Errorf("%w: cannot create %s", ErrComposeBlocked, file.rel)
			}
			body, err := os.ReadFile(file.source)
			if err != nil {
				return writes, fmt.Errorf("%w: cannot read %s", ErrComposeBlocked, file.rel)
			}
			if err := os.WriteFile(file.target, body, file.mode); err != nil {
				return writes, fmt.Errorf("%w: cannot write %s", ErrComposeBlocked, file.rel)
			}
			writes.files = append(writes.files, file)
		}
	}
	return writes, nil
}

// makeDirectories creates the missing levels between root and target one at a
// time, recording only the ones it actually made. os.MkdirAll cannot report
// that, and a rollback that removed a directory the user already had would be
// destroying their content.
func makeDirectories(root, target string, writes *composeWrites) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if _, err := os.Lstat(current); err == nil {
			continue
		}
		if err := os.Mkdir(current, 0o755); err != nil {
			return err
		}
		writes.directories = append(writes.directories, current)
	}
	return nil
}

// rollback undoes one compose call: the files it wrote, then the directories it
// created, deepest first. Anything that was already there is left alone.
func rollback(writes composeWrites) {
	for index := len(writes.files) - 1; index >= 0; index-- {
		_ = os.Remove(writes.files[index].target)
	}
	for index := len(writes.directories) - 1; index >= 0; index-- {
		// os.Remove refuses non-empty directories, so a directory that gained
		// unrelated content survives on its own.
		_ = os.Remove(writes.directories[index])
	}
}

// commitManifest writes the document to a temporary file next to the manifest,
// validates it, and only then renames it into place.
func commitManifest(
	manifestPath, root string, document []byte, validateManifest func(manifestPath, root string) error,
) error {
	directory := filepath.Dir(manifestPath)
	temporary, err := os.CreateTemp(directory, ".aiah-manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("%w: cannot stage the manifest", ErrComposeBlocked)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if _, err := temporary.Write(document); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%w: cannot stage the manifest", ErrComposeBlocked)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%w: cannot stage the manifest", ErrComposeBlocked)
	}
	if validateManifest != nil {
		if err := validateManifest(temporaryPath, root); err != nil {
			return fmt.Errorf("%w: %s", ErrComposeBlocked, err.Error())
		}
	}
	if err := os.Rename(temporaryPath, manifestPath); err != nil {
		return fmt.Errorf("%w: cannot install the manifest", ErrComposeBlocked)
	}
	return nil
}

func loadExistingManifest(manifestPath string) ([]byte, map[string]bool, error) {
	ids := make(map[string]bool)
	raw, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ids, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%w: cannot read the existing manifest", ErrComposeBlocked)
	}
	if strings.EqualFold(filepath.Ext(manifestPath), ".json") {
		return nil, nil, fmt.Errorf("%w: composing into a JSON manifest is not supported; convert it to YAML first", ErrComposeBlocked)
	}
	document, _, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: the existing manifest does not parse", ErrComposeBlocked)
	}
	for _, asset := range document.Assets {
		ids[asset.ID] = true
	}
	return raw, ids, nil
}

func requireDirectory(candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", os.ErrInvalid
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", os.ErrInvalid
	}
	return absolute, nil
}

func withinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// renderManifest produces the new manifest bytes. An existing document is
// edited as a yaml.Node tree so comments, key order and fields this version
// does not know about all survive (ADR-0006 §4).
func renderManifest(
	existing []byte, manifestPath string, planned []plannedAsset, profile string, options ComposeOptions,
) ([]byte, error) {
	if len(existing) == 0 {
		return renderFreshManifest(manifestPath, planned, profile, options)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(existing, &root); err != nil {
		return nil, fmt.Errorf("%w: the existing manifest does not parse", ErrComposeBlocked)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: the existing manifest is not a mapping; fix it by hand", ErrComposeBlocked)
	}
	body := root.Content[0]

	assets := mappingValue(body, "assets")
	if assets == nil || assets.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%w: cannot locate the assets sequence; fix it by hand", ErrComposeBlocked)
	}
	profiles := mappingValue(body, "profiles")
	if profiles == nil || profiles.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: cannot locate the profiles mapping; fix it by hand", ErrComposeBlocked)
	}
	include := profileInclude(profiles, profile)
	if include == nil {
		return nil, fmt.Errorf("%w: cannot locate profile %q include list; fix it by hand", ErrComposeBlocked, profile)
	}

	for _, item := range planned {
		var entry yaml.Node
		if err := entry.Encode(item.entry); err != nil {
			return nil, fmt.Errorf("%w: cannot encode manifest entry", ErrComposeBlocked)
		}
		entry.Style = 0
		assets.Content = append(assets.Content, &entry)
		include.Content = append(include.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: item.entry.ID,
		})
	}

	return encodeDocument(&root)
}

func renderFreshManifest(
	manifestPath string, planned []plannedAsset, profile string, options ComposeOptions,
) ([]byte, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = "workspace"
	}
	version := strings.TrimSpace(options.Version)
	if version == "" {
		version = "0.1.0"
	}
	if strings.EqualFold(filepath.Ext(manifestPath), ".json") {
		return nil, fmt.Errorf("%w: composing into a JSON manifest is not supported; use manifest.yaml", ErrComposeBlocked)
	}

	document := Document{SchemaVersion: 1, Name: name, Version: version}
	include := make([]string, 0, len(planned))
	for _, item := range planned {
		document.Assets = append(document.Assets, item.entry)
		include = append(include, item.entry.ID)
	}
	document.Profiles = map[string]Profile{profile: {Include: include}}

	var root yaml.Node
	if err := root.Encode(document); err != nil {
		return nil, fmt.Errorf("%w: cannot encode the manifest", ErrComposeBlocked)
	}
	return encodeDocument(&root)
}

func encodeDocument(root *yaml.Node) ([]byte, error) {
	var builder strings.Builder
	encoder := yaml.NewEncoder(&builder)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("%w: cannot encode the manifest", ErrComposeBlocked)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("%w: cannot encode the manifest", ErrComposeBlocked)
	}
	return []byte(builder.String()), nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func profileInclude(profiles *yaml.Node, profile string) *yaml.Node {
	entry := mappingValue(profiles, profile)
	if entry == nil {
		// A profile the manifest does not declare yet is added rather than
		// treated as an error: it is new content, not a structure we failed to
		// understand.
		entry = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		include := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "include"}, include)
		profiles.Content = append(profiles.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: profile}, entry)
		return include
	}
	if entry.Kind != yaml.MappingNode {
		return nil
	}
	include := mappingValue(entry, "include")
	if include == nil {
		include = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "include"}, include)
		return include
	}
	if include.Kind != yaml.SequenceNode {
		return nil
	}
	return include
}
