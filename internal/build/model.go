package build

import "github.com/dff652/ai-asset-hub/internal/workspace"

// Report is the deterministic JSON report for `aiah build`.
type Report struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Kind          string              `json:"kind"`
	ProducedBy    string              `json:"producedBy"`
	Ok            bool                `json:"ok"`
	Profile       string              `json:"profile,omitempty"`
	Package       *PackageInfo        `json:"package,omitempty"`
	Summary       Summary             `json:"summary"`
	Capabilities  map[string][]string `json:"capabilities,omitempty"`
	Findings      []Finding           `json:"findings"`
	Validation    *ValidationSummary  `json:"validation,omitempty"`
}

type PackageInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Archive  string `json:"archive"`
	Manifest string `json:"manifest"`
	Lock     string `json:"lock"`
	SHA256   string `json:"sha256File"`
	Digest   string `json:"archiveSha256"`
}

type Summary struct {
	AssetCount int      `json:"assetCount"`
	FileCount  int      `json:"fileCount"`
	Targets    []string `json:"targets"`
}

type Finding = workspace.Finding

type ValidationSummary struct {
	Ok         bool `json:"ok"`
	ErrorCount int  `json:"errorCount"`
}

// PackageManifest is the resolved, hashed manifest written into the package.
type PackageManifest struct {
	SchemaVersion int            `json:"schemaVersion"`
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Profile       string         `json:"profile"`
	Targets       []string       `json:"targets"`
	Assets        []PackageAsset `json:"assets"`
}

type PackageAsset struct {
	ID          string        `json:"id"`
	Type        string        `json:"type"`
	Path        string        `json:"path"`
	Targets     []string      `json:"targets"`
	Scope       string        `json:"scope"`
	Portability string        `json:"portability"`
	Sensitivity string        `json:"sensitivity"`
	Files       []PackageFile `json:"files"`
}

type PackageFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// LockFile lists every packed file hash for audit and future apply.
type LockFile struct {
	SchemaVersion int         `json:"schemaVersion"`
	Name          string      `json:"name"`
	Version       string      `json:"version"`
	Profile       string      `json:"profile"`
	Files         []LockEntry `json:"files"`
}

type LockEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

const (
	codeValidationFailed  = "validation_failed"
	codeUnknownProfile    = "unknown_profile"
	codeEmptySelection    = "empty_selection"
	codeConflictSelected  = "conflict_selected"
	codeMissingDependency = "missing_dependency"
	codeEmptyTargets      = "empty_targets"
	codeBuildIO           = "build_io"
)
