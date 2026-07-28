package workspace

// Finding is a deterministic, path-safe workspace issue.
type Finding struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Paths    []string `json:"paths"`
}

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	// SeverityInfo is reserved for non-blocking notes (may be serialized as warning
	// by older report consumers if not listed in schema).
	SeverityInfo = "info"

	CodeInvalidManifest    = "invalid_manifest"
	CodeDuplicateAssetID   = "duplicate_asset_id"
	CodeMissingDependency  = "missing_dependency"
	CodeMissingConflictRef = "missing_conflict_ref"
	CodeMissingProfileRef  = "missing_profile_ref"
	CodeMissingAssetPath   = "missing_asset_path"
	CodePathEscape         = "path_escape"
	CodeSymlinkEscape      = "symlink_escape"
	CodeBrokenSymlink      = "broken_symlink"
	CodeSymlinkNotPackable = "symlink_not_packable"
	CodeSuspectedSecret    = "suspected_secret"
	CodeBinary             = "binary"
	CodeTooLarge           = "too-large"
	CodeUnreadable         = "unreadable"
)

// HasError reports whether any finding is severity error.
func HasError(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == SeverityError {
			return true
		}
	}
	return false
}
