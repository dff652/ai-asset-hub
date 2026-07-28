package inventory

type RootID string
type Source string
type Scope string
type AssetType string
type FileType string
type Portability string
type Sensitivity string
type AssetStatus string
type EntryStatus string
type ConfigState string
type Feature string
type FindingCode string
type Severity string
type ProjectMigration string
type DeviceMigration string
type ExclusionReason string
type SymlinkState string

const (
	RootHome    RootID = "home"
	RootProject RootID = "project"

	SourceClaude  Source = "claude"
	SourceCodex   Source = "codex"
	SourceGrok    Source = "grok"
	SourceShared  Source = "shared"
	SourceProject Source = "project"

	ScopeGlobal        Scope = "global"
	ScopeProject       Scope = "project"
	ScopeDevicePrivate Scope = "device-private"

	TypeRules         AssetType = "rules"
	TypeSkill         AssetType = "skill"
	TypeAgent         AssetType = "agent"
	TypeHook          AssetType = "hook"
	TypePlugin        AssetType = "plugin"
	TypeConfig        AssetType = "config"
	TypeMCPConfig     AssetType = "mcp-config"
	TypeMemory        AssetType = "memory"
	TypeCredential    AssetType = "credential"
	TypeSession       AssetType = "session"
	TypeCache         AssetType = "cache"
	TypeDatabase      AssetType = "database"
	TypeDeviceState   AssetType = "device-state"
	TypeProjectSource AssetType = "project-source"
	TypeUnknown       AssetType = "unknown"

	FileRegular   FileType = "file"
	FileDirectory FileType = "directory"
	FileSymlink   FileType = "symlink"
	FileSpecial   FileType = "special"
	FileUnknown   FileType = "unknown"

	PortabilityAdapterRequired Portability = "adapter-required"
	PortabilityExcluded        Portability = "excluded"
	PortabilityUnknown         Portability = "unknown"

	SensitivityPublic    Sensitivity = "public"
	SensitivityPrivate   Sensitivity = "private"
	SensitivitySensitive Sensitivity = "sensitive"
	SensitivitySecret    Sensitivity = "secret"
	SensitivityUnknown   Sensitivity = "unknown"

	AssetCandidate  AssetStatus = "candidate"
	AssetExcluded   AssetStatus = "excluded"
	AssetUnreadable AssetStatus = "unreadable"

	EntryIncluded   EntryStatus = "included"
	EntryReported   EntryStatus = "reported"
	EntryExcluded   EntryStatus = "excluded"
	EntryUnreadable EntryStatus = "unreadable"

	ConfigValid   ConfigState = "valid"
	ConfigInvalid ConfigState = "invalid"

	FeatureCodexProjectDocFallback Feature = "codex-project-doc-fallback"
	FeatureHooks                   Feature = "hooks"
	FeatureMCP                     Feature = "mcp"
	FeaturePlugins                 Feature = "plugins"

	FindingBrokenSymlink   FindingCode = "broken_symlink"
	FindingIncompleteSkill FindingCode = "incomplete_skill"
	FindingInvalidConfig   FindingCode = "invalid_config"
	FindingLargeFile       FindingCode = "large_file"
	FindingRulesShadowing  FindingCode = "rules_shadowing"
	FindingSuspectedSecret FindingCode = "suspected_secret"
	FindingSymlinkEscape   FindingCode = "symlink_escape"
	FindingSymlinkedAsset  FindingCode = "symlinked_asset"
	FindingUnreadable      FindingCode = "unreadable"

	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"

	ProjectNotScanned         ProjectMigration = "not-scanned"
	ProjectNoAssets           ProjectMigration = "no-assets"
	ProjectPartial            ProjectMigration = "partial"
	ProjectDualToolConfigured ProjectMigration = "dual-tool-configured"
	ProjectConflicted         ProjectMigration = "conflicted"

	DeviceNoAssets      DeviceMigration = "no-assets"
	DeviceBlocked       DeviceMigration = "blocked"
	DeviceInventoryOnly DeviceMigration = "inventory-only"

	ExcludeCredential      ExclusionReason = "credential"
	ExcludeDatabase        ExclusionReason = "database"
	ExcludeNativeSession   ExclusionReason = "native-session"
	ExcludeCache           ExclusionReason = "cache"
	ExcludeDeviceState     ExclusionReason = "device-state"
	ExcludeNativeMemory    ExclusionReason = "native-memory"
	ExcludeSymlink         ExclusionReason = "symlink"
	ExcludeBrokenSymlink   ExclusionReason = "broken-symlink"
	ExcludeSymlinkEscape   ExclusionReason = "symlink-escape"
	ExcludeSpecialFile     ExclusionReason = "special-file"
	ExcludeUnreadable      ExclusionReason = "unreadable"
	ExcludeTooLarge        ExclusionReason = "too-large"
	ExcludeBinary          ExclusionReason = "binary"
	ExcludeSuspectedSecret ExclusionReason = "suspected-secret"

	SymlinkInternal SymlinkState = "internal"
	SymlinkEscape   SymlinkState = "escape"
	SymlinkBroken   SymlinkState = "broken"
)

// Report is inventory schema v1. Struct fields define stable JSON key ordering;
// assets, entries, findings, feature lists, and file lists are sorted.
type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Kind          string    `json:"kind"`
	ProducedBy    string    `json:"producedBy"`
	Roots         []Root    `json:"roots"`
	Summary       Summary   `json:"summary"`
	Assets        []Asset   `json:"assets"`
	Entries       []Entry   `json:"entries"`
	Findings      []Finding `json:"findings"`
}

type Root struct {
	ID      RootID `json:"id"`
	Kind    RootID `json:"kind"`
	Present bool   `json:"present"`
}

type Summary struct {
	TotalAssets       int               `json:"totalAssets"`
	CandidateAssets   int               `json:"candidateAssets"`
	ExcludedAssets    int               `json:"excludedAssets"`
	UnreadableAssets  int               `json:"unreadableAssets"`
	TotalEntries      int               `json:"totalEntries"`
	IncludedEntries   int               `json:"includedEntries"`
	ReportedEntries   int               `json:"reportedEntries"`
	ExcludedEntries   int               `json:"excludedEntries"`
	UnreadableEntries int               `json:"unreadableEntries"`
	CandidateByType   map[AssetType]int `json:"candidateByType"`
	ProjectMigration  ProjectMigration  `json:"projectMigration"`
	DeviceMigration   DeviceMigration   `json:"deviceMigration"`
}

type Asset struct {
	LogicalPath string      `json:"logicalPath"`
	Source      Source      `json:"source"`
	Scope       Scope       `json:"scope"`
	Type        AssetType   `json:"type"`
	Portability Portability `json:"portability"`
	Sensitivity Sensitivity `json:"sensitivity"`
	Status      AssetStatus `json:"status"`
	Files       []string    `json:"files"`
	ConfigState ConfigState `json:"configState,omitempty"`
	Features    []Feature   `json:"features,omitempty"`
}

type Entry struct {
	LogicalPath     string          `json:"logicalPath"`
	AssetPath       string          `json:"assetPath,omitempty"`
	Source          Source          `json:"source"`
	Scope           Scope           `json:"scope"`
	Type            AssetType       `json:"type"`
	FileType        FileType        `json:"fileType"`
	Size            int64           `json:"size"`
	Mode            string          `json:"mode"`
	SHA256          string          `json:"sha256,omitempty"`
	Portability     Portability     `json:"portability"`
	Sensitivity     Sensitivity     `json:"sensitivity"`
	Status          EntryStatus     `json:"status"`
	ExclusionReason ExclusionReason `json:"exclusionReason,omitempty"`
	SymlinkState    SymlinkState    `json:"symlinkState,omitempty"`
	ConfigState     ConfigState     `json:"configState,omitempty"`
	Features        []Feature       `json:"features,omitempty"`
}

type Finding struct {
	Code     FindingCode `json:"code"`
	Severity Severity    `json:"severity"`
	Message  string      `json:"message"`
	Paths    []string    `json:"paths"`
}
