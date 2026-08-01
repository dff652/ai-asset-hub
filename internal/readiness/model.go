// Package readiness aggregates the existing build and migration preflight
// reports into one read-only answer for cross-device preparation.
package readiness

import "github.com/dff652/ai-asset-hub/internal/workspace"

const (
	LevelBlocked   = "blocked"
	LevelAttention = "attention"
	LevelReady     = "ready"

	StatusReady     = "ready"
	StatusAttention = "attention"
	StatusBlocked   = "blocked"
	StatusMissing   = "missing"
	StatusRecorded  = "recorded"
	StatusMismatch  = "mismatch"
	StatusInvalid   = "invalid"
	StatusUnchecked = "unchecked"
	StatusPassed    = "passed"
	StatusFailed    = "failed"
)

// Options selects one asset-library profile and optional evidence files.
// Evidence paths are constrained to <workspace>/.aiah/evidence by Inspect.
type Options struct {
	WorkspaceRoot       string
	ManifestPath        string
	Profile             string
	Home                string
	Project             string
	BackupEvidencePath  string
	RestoreExercisePath string
}

type Subject struct {
	Name            string `json:"name,omitempty"`
	Version         string `json:"version,omitempty"`
	Profile         string `json:"profile"`
	SelectionSHA256 string `json:"selectionSHA256,omitempty"`
}

type PackageReadiness struct {
	Status     string `json:"status"`
	AssetCount int    `json:"assetCount"`
	FileCount  int    `json:"fileCount"`
}

type MigrationPreflight struct {
	Status             string   `json:"status"`
	Targets            []string `json:"targets"`
	UnsupportedTargets int      `json:"unsupportedTargets"`
	DroppedItems       int      `json:"droppedItems"`
	DegradedItems      int      `json:"degradedItems"`
	SecretReferences   int      `json:"secretReferences"`
	MissingSecrets     int      `json:"missingSecrets"`
	DevicePrivateItems int      `json:"devicePrivateItems"`
}

type BackupEvidence struct {
	Status          string `json:"status"`
	Type            string `json:"type,omitempty"`
	RecordedAt      string `json:"recordedAt,omitempty"`
	ReferenceDigest string `json:"referenceDigest,omitempty"`
}

type RestoreExercise struct {
	Status        string   `json:"status"`
	PackageSHA256 string   `json:"packageSHA256,omitempty"`
	Targets       []string `json:"targets"`
	ProducedBy    string   `json:"producedBy,omitempty"`
	CompletedAt   string   `json:"completedAt,omitempty"`
}

// Report keeps completion evidence separate from operational success. Ok is
// false only when package preparation or migration preflight is blocked;
// missing evidence produces level=attention without turning a safe inspection
// into a command failure.
type Report struct {
	SchemaVersion      int                 `json:"schemaVersion"`
	Kind               string              `json:"kind"`
	ProducedBy         string              `json:"producedBy"`
	Ok                 bool                `json:"ok"`
	Level              string              `json:"level"`
	Subject            Subject             `json:"subject"`
	PackageReadiness   PackageReadiness    `json:"packageReadiness"`
	MigrationPreflight MigrationPreflight  `json:"migrationPreflight"`
	BackupEvidence     BackupEvidence      `json:"backupEvidence"`
	RestoreExercise    RestoreExercise     `json:"restoreExercise"`
	Findings           []workspace.Finding `json:"findings"`
}
