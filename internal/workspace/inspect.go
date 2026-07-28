package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/content"
)

// FileBlob is a packable regular file discovered under the workspace.
type FileBlob struct {
	Path   string
	SHA256 string
	Body   []byte
}

// Inspection is the single filesystem pass shared by validate and build.
type Inspection struct {
	Document Document
	Root     string
	Findings []Finding
	// Files is keyed by workspace-relative logical path for packable regular files.
	Files map[string]FileBlob
	// AssetFiles lists packable files per asset id (stable order).
	AssetFiles map[string][]string
	Checked    int
}

// Options configures workspace loading and inspection.
type Options struct {
	Manifest string
	Root     string
	// CollectBodies stores file bytes in Files for packaging. Validate can leave this false.
	CollectBodies bool
}

// Inspect loads the manifest, validates schema, checks references, and walks
// asset trees once. Symlinks are never packable; escaping or broken links error.
func Inspect(options Options) (Inspection, error) {
	if options.Manifest == "" {
		return Inspection{}, ErrInvalidOptions
	}
	manifestPath, err := filepath.Abs(options.Manifest)
	if err != nil {
		return Inspection{}, ErrInvalidOptions
	}
	info, err := os.Stat(manifestPath)
	if err != nil || info.IsDir() {
		return Inspection{}, ErrInvalidOptions
	}
	root, err := ResolveRoot(manifestPath, options.Root)
	if err != nil {
		return Inspection{}, err
	}

	result := Inspection{
		Root:       root,
		Findings:   make([]Finding, 0),
		Files:      make(map[string]FileBlob),
		AssetFiles: make(map[string][]string),
	}

	document, normalized, err := LoadManifest(manifestPath)
	if err != nil {
		result.Findings = append(result.Findings, Finding{
			Code:     CodeInvalidManifest,
			Severity: SeverityError,
			Message:  "Manifest could not be parsed.",
			Paths:    []string{LogicalManifestPath(root, manifestPath)},
		})
		return finishInspection(result), nil
	}
	result.Document = document

	if err := ValidateManifestSchema(normalized); err != nil {
		result.Findings = append(result.Findings, Finding{
			Code:     CodeInvalidManifest,
			Severity: SeverityError,
			Message:  "Manifest does not match schema v1.",
			Paths:    []string{LogicalManifestPath(root, manifestPath)},
		})
		// Stop after schema failure — typed fields are not trustworthy.
		return finishInspection(result), nil
	}

	result.Findings = append(result.Findings, checkReferences(document)...)
	if HasError(result.Findings) {
		return finishInspection(result), nil
	}

	for _, asset := range document.Assets {
		checked, findings, files := inspectAssetTree(root, asset, options.CollectBodies)
		result.Checked += checked
		result.Findings = append(result.Findings, findings...)
		paths := make([]string, 0, len(files))
		for _, file := range files {
			result.Files[file.Path] = file
			paths = append(paths, file.Path)
		}
		sort.Strings(paths)
		result.AssetFiles[asset.ID] = paths
	}

	return finishInspection(result), nil
}

func finishInspection(result Inspection) Inspection {
	sort.SliceStable(result.Findings, func(i, j int) bool {
		left, right := result.Findings[i], result.Findings[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.Severity != right.Severity {
			return left.Severity < right.Severity
		}
		return strings.Join(left.Paths, "\x00") < strings.Join(right.Paths, "\x00")
	})
	return result
}

func checkReferences(document Document) []Finding {
	findings := make([]Finding, 0)
	ids := make(map[string]int, len(document.Assets))
	for _, asset := range document.Assets {
		ids[asset.ID]++
	}
	for id, count := range ids {
		if count > 1 {
			findings = append(findings, Finding{
				Code:     CodeDuplicateAssetID,
				Severity: SeverityError,
				Message:  "Asset id is declared more than once.",
				Paths:    []string{id},
			})
		}
	}
	for _, asset := range document.Assets {
		for _, dep := range asset.Dependencies {
			if ids[dep] == 0 {
				findings = append(findings, Finding{
					Code:     CodeMissingDependency,
					Severity: SeverityError,
					Message:  "Dependency references an unknown asset id.",
					Paths:    []string{asset.ID, dep},
				})
			}
		}
		for _, conflict := range asset.Conflicts {
			if ids[conflict] == 0 {
				findings = append(findings, Finding{
					Code:     CodeMissingConflictRef,
					Severity: SeverityError,
					Message:  "Conflict references an unknown asset id.",
					Paths:    []string{asset.ID, conflict},
				})
			}
		}
	}
	profileNames := make([]string, 0, len(document.Profiles))
	for name := range document.Profiles {
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)
	for _, name := range profileNames {
		profile := document.Profiles[name]
		for _, ref := range append(append([]string{}, profile.Include...), profile.Exclude...) {
			if ids[ref] == 0 {
				findings = append(findings, Finding{
					Code:     CodeMissingProfileRef,
					Severity: SeverityError,
					Message:  "Profile references an unknown asset id.",
					Paths:    []string{"profiles/" + name, ref},
				})
			}
		}
	}
	return findings
}

func inspectAssetTree(root string, asset ManifestAsset, collectBodies bool) (int, []Finding, []FileBlob) {
	findings := make([]Finding, 0)
	files := make([]FileBlob, 0)
	if !SafeRelativePath(asset.Path) {
		return 0, []Finding{{
			Code:     CodePathEscape,
			Severity: SeverityError,
			Message:  "Asset path is absolute or escapes the workspace root.",
			Paths:    []string{asset.ID, asset.Path},
		}}, nil
	}

	full := filepath.Join(root, filepath.FromSlash(asset.Path))
	info, err := os.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, []Finding{{
			Code:     CodeMissingAssetPath,
			Severity: SeverityError,
			Message:  "Asset path does not exist.",
			Paths:    []string{asset.ID, asset.Path},
		}}, nil
	}
	if err != nil {
		return 0, []Finding{{
			Code:     CodeUnreadable,
			Severity: SeverityError,
			Message:  "Asset path is unreadable.",
			Paths:    []string{asset.ID, asset.Path},
		}}, nil
	}

	checked := 0
	if info.Mode()&os.ModeSymlink != 0 {
		checked++
		findings = append(findings, inspectSymlink(root, asset.ID, asset.Path, full)...)
		return checked, findings, files
	}
	if !info.IsDir() {
		checked++
		fileFindings, blob := inspectFile(asset.ID, asset.Path, full, info, collectBodies)
		findings = append(findings, fileFindings...)
		if blob != nil {
			files = append(files, *blob)
		}
		return checked, findings, files
	}

	_ = filepath.WalkDir(full, func(current string, entry fs.DirEntry, walkErr error) error {
		rel, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return nil
		}
		logical := filepath.ToSlash(rel)
		if walkErr != nil {
			findings = append(findings, Finding{
				Code:     CodeUnreadable,
				Severity: SeverityError,
				Message:  "Path is unreadable.",
				Paths:    []string{asset.ID, logical},
			})
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		currentInfo, infoErr := entry.Info()
		if infoErr != nil {
			findings = append(findings, Finding{
				Code:     CodeUnreadable,
				Severity: SeverityError,
				Message:  "Path is unreadable.",
				Paths:    []string{asset.ID, logical},
			})
			return nil
		}
		if currentInfo.Mode()&os.ModeSymlink != 0 {
			checked++
			findings = append(findings, inspectSymlink(root, asset.ID, logical, current)...)
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		checked++
		fileFindings, blob := inspectFile(asset.ID, logical, current, currentInfo, collectBodies)
		findings = append(findings, fileFindings...)
		if blob != nil {
			files = append(files, *blob)
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return checked, findings, files
}

func inspectSymlink(root, assetID, logical, full string) []Finding {
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return []Finding{{
			Code:     CodeBrokenSymlink,
			Severity: SeverityError,
			Message:  "Broken symlink was not followed.",
			Paths:    []string{assetID, logical},
		}}
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil || !Within(root, resolved) {
		return []Finding{{
			Code:     CodeSymlinkEscape,
			Severity: SeverityError,
			Message:  "Symlink escapes the workspace root and was not followed.",
			Paths:    []string{assetID, logical},
		}}
	}
	return []Finding{{
		Code:     CodeSymlinkNotPackable,
		Severity: SeverityError,
		Message:  "Symlinks are not packable; replace with regular files.",
		Paths:    []string{assetID, logical},
	}}
}

func inspectFile(assetID, logical, full string, info fs.FileInfo, collectBodies bool) ([]Finding, *FileBlob) {
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	if info.Mode().Perm()&0444 == 0 {
		return []Finding{{
			Code:     CodeUnreadable,
			Severity: SeverityError,
			Message:  "File is unreadable.",
			Paths:    []string{assetID, logical},
		}}, nil
	}
	if info.Size() > content.MaxScannedFileSize {
		return []Finding{{
			Code:     CodeTooLarge,
			Severity: SeverityError,
			Message:  "File exceeds the content inspection size limit.",
			Paths:    []string{assetID, logical},
		}}, nil
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return []Finding{{
			Code:     CodeUnreadable,
			Severity: SeverityError,
			Message:  "File is unreadable.",
			Paths:    []string{assetID, logical},
		}}, nil
	}
	if content.IsBinary(body) {
		return []Finding{{
			Code:     CodeBinary,
			Severity: SeverityError,
			Message:  "Binary file is not allowed in the asset workspace.",
			Paths:    []string{assetID, logical},
		}}, nil
	}
	if content.ContainsSecret(body) {
		return []Finding{{
			Code:     CodeSuspectedSecret,
			Severity: SeverityError,
			Message:  "Possible secret detected; content was not included in the report.",
			Paths:    []string{assetID, logical},
		}}, nil
	}
	sum := sha256.Sum256(body)
	blob := &FileBlob{
		Path:   logical,
		SHA256: hex.EncodeToString(sum[:]),
	}
	if collectBodies {
		blob.Body = body
	}
	return nil, blob
}
