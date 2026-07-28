package inventory

import (
	"path"
	"strings"
)

type pathPolicy struct {
	source          Source
	scope           Scope
	assetType       AssetType
	portability     Portability
	sensitivity     Sensitivity
	exclusionReason ExclusionReason
}

func policyFor(rootID RootID, relative string) pathPolicy {
	slashPath := strings.TrimPrefix(strings.ReplaceAll(relative, `\`, "/"), "./")
	lower := strings.ToLower(slashPath)
	base := path.Base(lower)
	segments := strings.Split(lower, "/")

	policy := pathPolicy{
		source:      sourceFor(rootID, slashPath),
		scope:       scopeFor(rootID),
		assetType:   assetTypeFor(lower, segments),
		portability: PortabilityAdapterRequired,
		sensitivity: SensitivityPrivate,
	}

	if reason, assetType := excludedPath(base, segments); reason != "" {
		policy.scope = ScopeDevicePrivate
		policy.assetType = assetType
		policy.portability = PortabilityExcluded
		policy.sensitivity = SensitivitySensitive
		if reason == ExcludeCredential {
			policy.sensitivity = SensitivitySecret
		}
		policy.exclusionReason = reason
		return policy
	}

	if policy.assetType == TypeUnknown {
		policy.portability = PortabilityUnknown
		policy.sensitivity = SensitivityUnknown
	}
	if policy.assetType == TypeConfig || policy.assetType == TypeMCPConfig {
		policy.sensitivity = SensitivitySensitive
	}
	if policy.assetType == TypeProjectSource {
		policy.source = SourceProject
	}
	return policy
}

func sourceFor(rootID RootID, relative string) Source {
	switch {
	case strings.HasPrefix(relative, ".claude/") || relative == ".claude" ||
		relative == "CLAUDE.md" || relative == ".claude.json":
		return SourceClaude
	case strings.HasPrefix(relative, ".codex/") || relative == ".codex" ||
		relative == "AGENTS.md":
		return SourceCodex
	case strings.HasPrefix(relative, ".grok/") || relative == ".grok":
		return SourceGrok
	case strings.HasPrefix(relative, ".agents/") || relative == ".agents":
		return SourceShared
	case rootID == RootProject:
		return SourceProject
	default:
		return SourceShared
	}
}

func scopeFor(rootID RootID) Scope {
	if rootID == RootProject {
		return ScopeProject
	}
	return ScopeGlobal
}

func assetTypeFor(lower string, segments []string) AssetType {
	base := path.Base(lower)
	switch {
	case strings.HasPrefix(lower, "scripts/claude-setup"):
		return TypeProjectSource
	case base == "claude.md" || base == "agents.md":
		return TypeRules
	}
	if assetType := firstAssetContainer(segments); assetType != TypeUnknown {
		return assetType
	}
	switch {
	case base == ".mcp.json":
		return TypeMCPConfig
	case base == ".claude.json" || base == "settings.json" ||
		base == "settings.local.json" || base == "config.toml":
		return TypeConfig
	default:
		return TypeUnknown
	}
}

func firstAssetContainer(segments []string) AssetType {
	for _, segment := range segments {
		switch segment {
		case "skills":
			return TypeSkill
		case "agents":
			return TypeAgent
		case "hooks":
			return TypeHook
		case "plugins":
			return TypePlugin
		case "rules":
			return TypeRules
		case "mcp":
			return TypeMCPConfig
		case "memory", "memories":
			return TypeMemory
		}
	}
	return TypeUnknown
}

func excludedPath(base string, segments []string) (ExclusionReason, AssetType) {
	switch {
	case base == ".credentials.json" || base == "credentials.json" ||
		base == "auth.json" || base == ".env" || strings.HasPrefix(base, ".env."):
		return ExcludeCredential, TypeCredential
	case strings.HasSuffix(base, ".sqlite") || strings.HasSuffix(base, ".sqlite3") ||
		strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".db-wal") ||
		strings.HasSuffix(base, ".db-shm"):
		return ExcludeDatabase, TypeDatabase
	case base == "history.jsonl" || containsAnySegment(segments,
		"session", "sessions", "archived_sessions", "projects", "file-history",
		"shell-snapshots", "session-env", "worktrees", "tasks", "todos"):
		return ExcludeNativeSession, TypeSession
	case containsCacheSegment(segments):
		return ExcludeCache, TypeCache
	case containsAnySegment(segments, "log", "logs", "telemetry"):
		return ExcludeDeviceState, TypeDeviceState
	case isDeviceStatePath(segments):
		return ExcludeDeviceState, TypeDeviceState
	// Known Grok home root runtime files (not under skills/rules/...).
	case underDotDir(segments, ".grok") && len(segments) == 2 && isGrokRootDeviceFile(base):
		return ExcludeDeviceState, TypeDeviceState
	case containsAnySegment(segments, "memory", "memories"):
		return ExcludeNativeMemory, TypeMemory
	default:
		return "", ""
	}
}

// containsCacheSegment reports whether any path segment names a cache
// container. Matching is per segment, never a basename substring: substring
// matching used to drop whole asset trees such as skills/cache-warmer.
func containsCacheSegment(segments []string) bool {
	for index, segment := range segments {
		if isCacheSegment(segment) && !isAssetNamePosition(segments, index) {
			return true
		}
	}
	return false
}

// isCacheSegment matches the cache directory and file names real harnesses
// produce: cache/, .cache/, paste-cache/, transcript-cache/, stats-cache.json,
// models_cache.json. A cache prefix (cache-warmer) is a name, not a cache.
func isCacheSegment(segment string) bool {
	if cacheNamed(segment) {
		return true
	}
	// A cache-named segment carrying an extension is only cache state when the
	// extension says so. Plugin sources (context-cache.ts) and docs (CACHE.md)
	// keep their cache-ish names while staying assets. A leading dot is never
	// read as an extension, so .cache/ stays a directory match above.
	if dot := strings.LastIndex(segment, "."); dot > 0 {
		return cacheNamed(segment[:dot]) && isStateFileExtension(segment[dot:])
	}
	return false
}

func isStateFileExtension(extension string) bool {
	switch extension {
	case ".json", ".jsonl", ".yaml", ".yml", ".toml", ".bin", ".dat", ".idx", ".lock":
		return true
	default:
		return false
	}
}

func cacheNamed(name string) bool {
	switch name {
	case "cache", "caches", ".cache", ".caches":
		return true
	}
	return strings.HasSuffix(name, "-cache") || strings.HasSuffix(name, "_cache") ||
		strings.HasSuffix(name, "-caches") || strings.HasSuffix(name, "_caches")
}

// isAssetNamePosition reports whether segments[index] is where the user names
// an asset, i.e. the direct child of an asset container. Only there does a
// cache-suffixed name lose to the asset reading: skills/go-cache is a skill,
// while plugins/<plugin>/transcript-cache is harness cache. Plugin names are
// deliberately not protected — they come from marketplaces, not the user.
func isAssetNamePosition(segments []string, index int) bool {
	if index == 0 {
		return false
	}
	// Derived from firstAssetContainer so the container vocabulary has one
	// definition. Plugins, mcp and memory are deliberately absent: those names
	// come from marketplaces and harnesses, not from the user, and that is
	// where harnesses write their caches.
	switch firstAssetContainer(segments[index-1 : index]) {
	case TypeSkill, TypeAgent, TypeRules, TypeHook:
		return true
	default:
		return false
	}
}

// Device state is named differently by each harness: Codex writes
// shell_snapshots where Claude writes shell-snapshots. The generic branches
// above only know Claude's spelling, so every target must declare its own
// names here — a missing entry silently turns device state into inventory.
// A real scan of one machine found ~/.codex/.tmp alone contributing 4658 of
// 5527 inventoried entries (docs/migrations/2026-07-25-dogfood-inventory.md).
//
// ADR-0002 phase A should move these onto the Target registry. Until then they
// live in one table keyed by harness root, not scattered across call sites.
var (
	// deviceStatePrefixes matches a fixed location below the harness root, so
	// common names stay usable elsewhere: plugins/data is device state while a
	// skill's own data/ directory is not.
	deviceStatePrefixes = map[string][]string{
		".claude": {
			"backups", "ide",
			"plugins/data", "plugins/marketplaces", "plugins/.last_inuse_sweep",
		},
		".codex": {
			".tmp", "tmp", "packages", "attachments", "ipc", "shell_snapshots",
			"vendor_imports", "session_index.jsonl", "installation_id", "version.json",
		},
		".grok": {"docs"},
	}
	// deviceStateAnywhere matches at any depth, for trees a harness recreates
	// wherever it likes.
	deviceStateAnywhere = map[string][]string{
		".codex": {".system"},
		".grok": {
			"downloads", "bundled", "marketplace-cache", "installed-plugins",
			"memtrace", "relocations", "debug", "completions", "vendor", "bin",
		},
	}
)

func isDeviceStatePath(segments []string) bool {
	if len(segments) < 2 {
		return false
	}
	root := segments[0]
	below := strings.Join(segments[1:], "/")
	for _, prefix := range deviceStatePrefixes[root] {
		if below == prefix || strings.HasPrefix(below, prefix+"/") {
			return true
		}
	}
	return containsAnySegment(segments[1:], deviceStateAnywhere[root]...)
}

func underDotDir(segments []string, dir string) bool {
	return len(segments) > 0 && segments[0] == dir
}

// isGrokRootDeviceFile matches runtime metadata living directly under ~/.grok
// (or project/.grok), not inside inventory asset containers.
func isGrokRootDeviceFile(base string) bool {
	switch base {
	case "active_sessions.json", "active_sessions.lock",
		"campaigns_state.json", "campaigns_state.lock",
		"slash-mru.json", "tip_cursor.json", "version.json",
		"models_cache.json", "trusted_folders.toml", "trusted_folders.toml.lock",
		"managed_config.lock", ".config-init.lock", ".metadata_version",
		"agent_id", "last-copy.txt", "changelog.json", "changelog.md", "readme.md":
		return true
	default:
		return false
	}
}

func containsSegment(segments []string, wanted string) bool {
	for _, segment := range segments {
		if segment == wanted {
			return true
		}
	}
	return false
}

func containsAnySegment(segments []string, wanted ...string) bool {
	for _, value := range wanted {
		if containsSegment(segments, value) {
			return true
		}
	}
	return false
}
