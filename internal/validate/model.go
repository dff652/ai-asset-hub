package validate

import "github.com/dff652/ai-asset-hub/internal/workspace"

// Report is validation schema v1. Findings are sorted for deterministic JSON.
type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Kind          string    `json:"kind"`
	ProducedBy    string    `json:"producedBy"`
	Ok            bool      `json:"ok"`
	Summary       Summary   `json:"summary"`
	Findings      []Finding `json:"findings"`
}

type Summary struct {
	AssetCount   int `json:"assetCount"`
	ProfileCount int `json:"profileCount"`
	CheckedPaths int `json:"checkedPaths"`
	ErrorCount   int `json:"errorCount"`
	WarningCount int `json:"warningCount"`
}

// Finding is the validation report finding shape.
type Finding = workspace.Finding

// Re-export document types used by callers and tests.
type (
	Document      = workspace.Document
	ManifestAsset = workspace.ManifestAsset
	Profile       = workspace.Profile
)

var (
	ErrInvalidManifest = workspace.ErrInvalidManifest
	ErrInvalidOptions  = workspace.ErrInvalidOptions
)

// LoadManifest parses a YAML or JSON manifest.
func LoadManifest(path string) (Document, any, error) {
	return workspace.LoadManifest(path)
}
