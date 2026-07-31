// Package preferences owns the device-local, non-business UI preferences.
//
// It deliberately does not select a workspace, create an asset library or
// change TUI state. Loading is read-only and fail-open to safe defaults; saving
// happens only when an interactive caller explicitly requests it.
package preferences

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

const (
	schemaVersion      = 1
	maxPreferencesSize = 64 << 10
)

// Language is a persisted UI language preference.
type Language string

const (
	LanguageAuto    Language = "auto"
	LanguageZhCN    Language = "zh-CN"
	LanguageEnglish Language = "en"
)

// Density controls only the default expansion of technical detail.
type Density string

const (
	DensityStandard Density = "standard"
	DensityDetailed Density = "detailed"
)

// Document is the complete version-1 preference document. It intentionally
// contains no business configuration, credentials or runtime history.
type Document struct {
	SchemaVersion         int      `json:"schemaVersion"`
	Language              Language `json:"language"`
	Density               Density  `json:"density"`
	PreferredAssetLibrary string   `json:"preferredAssetLibrary,omitempty"`
}

// StoreOptions makes the config file and workspace safety boundaries
// injectable. ConfigPath may be empty to use DefaultPath.
type StoreOptions struct {
	ConfigPath string
	Home       string
	Project    string
}

// WarningCode is a stable reason why loading fell back or needs user attention.
// Presentation layers translate these codes instead of parsing error text.
type WarningCode string

const (
	WarningConfigPath       WarningCode = "config_path_unavailable"
	WarningUnsafeStore      WarningCode = "unsafe_preferences_store"
	WarningUnreadableStore  WarningCode = "unreadable_preferences_store"
	WarningInvalidDocument  WarningCode = "invalid_preferences_document"
	WarningPreferredLibrary WarningCode = "preferred_asset_library_unavailable"
)

// LoadReport always contains usable preferences. Valid is false only when the
// stored document was ignored and Defaults were substituted. A stale preferred
// library remains available for prefill and is reported as a warning.
type LoadReport struct {
	Preferences Document
	ConfigPath  string
	Found       bool
	Valid       bool
	Warnings    []WarningCode
}

// LocaleEnvironment is injected locale state, ordered like the POSIX locale
// variables. It does not read process environment by itself.
type LocaleEnvironment struct {
	LCAll      string
	LCMessages string
	Lang       string
}

// ResolveOptions combines an already-loaded preference document with
// process-local overrides and injected locale state.
type ResolveOptions struct {
	Current          Document
	Locale           LocaleEnvironment
	LanguageOverride Language
	DensityOverride  Density
}

// Effective contains process-local UI choices. LanguageAuto is resolved to one
// of the two compiled languages.
type Effective struct {
	Language              Language
	Density               Density
	PreferredAssetLibrary string
}

var (
	// ErrInvalidPreferences means a document or override is not part of v1.
	ErrInvalidPreferences = errors.New("invalid preferences")
	// ErrStoreBlocked means preferences could not be saved safely.
	ErrStoreBlocked = errors.New("preferences store blocked")
)

// Defaults returns the safe v1 defaults. It performs no filesystem access.
func Defaults() Document {
	return Document{
		SchemaVersion: schemaVersion,
		Language:      LanguageAuto,
		Density:       DensityStandard,
	}
}

// DefaultPath returns the operator-local preference path. It is independent of
// every inventory/apply --home option.
func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("%w: user config directory", ErrStoreBlocked)
	}
	absolute, err := filepath.Abs(filepath.Join(root, "aiah", "preferences.json"))
	if err != nil {
		return "", fmt.Errorf("%w: preference path", ErrStoreBlocked)
	}
	return filepath.Clean(absolute), nil
}

// Load reads preferences without creating, repairing or rewriting anything.
// Every store/document failure returns Defaults plus a warning so optional UI
// preferences cannot block inventory, doctor or an explicit workspace.
func Load(options StoreOptions) LoadReport {
	report := LoadReport{
		Preferences: Defaults(),
		Valid:       true,
		Warnings:    []WarningCode{},
	}
	configPath, err := resolveConfigPath(options.ConfigPath)
	if err != nil {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningConfigPath)
		return report
	}
	report.ConfigPath = configPath

	parentInfo, err := os.Lstat(filepath.Dir(configPath))
	if errors.Is(err, os.ErrNotExist) {
		return report
	}
	if err != nil || !privateDirectory(parentInfo) {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningUnsafeStore)
		return report
	}

	targetInfo, err := os.Lstat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return report
	}
	report.Found = true
	if err != nil || !privateRegularFile(targetInfo) ||
		targetInfo.Size() > maxPreferencesSize {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningUnsafeStore)
		return report
	}

	file, err := os.Open(configPath)
	if err != nil {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningUnreadableStore)
		return report
	}
	defer func() { _ = file.Close() }()

	openedInfo, statErr := file.Stat()
	currentParent, parentErr := os.Lstat(filepath.Dir(configPath))
	currentTarget, targetErr := os.Lstat(configPath)
	if statErr != nil || parentErr != nil || targetErr != nil ||
		!os.SameFile(parentInfo, currentParent) ||
		!os.SameFile(targetInfo, currentTarget) ||
		!os.SameFile(targetInfo, openedInfo) ||
		!privateDirectory(currentParent) ||
		!privateRegularFile(currentTarget) {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningUnsafeStore)
		return report
	}

	body, err := io.ReadAll(io.LimitReader(file, maxPreferencesSize+1))
	if err != nil || len(body) > maxPreferencesSize {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningUnreadableStore)
		return report
	}
	document, err := decodeDocument(body)
	if err != nil {
		report.Valid = false
		report.Warnings = append(report.Warnings, WarningInvalidDocument)
		return report
	}
	report.Preferences = document
	if document.PreferredAssetLibrary != "" {
		if _, err := validatePreferredLibrary(
			document.PreferredAssetLibrary,
			options.Home,
			options.Project,
		); err != nil {
			report.Warnings = append(report.Warnings, WarningPreferredLibrary)
		}
	}
	return report
}

// Save validates and atomically replaces the full preference document. It
// returns the normalized document that was persisted.
func Save(options StoreOptions, document Document) (Document, error) {
	return save(options, document, nil)
}

type saveHooks struct {
	beforeRename func(temporaryPath, targetPath string) error
}

func save(options StoreOptions, document Document, hooks *saveHooks) (Document, error) {
	normalized, err := normalizeDocument(document, options.Home, options.Project)
	if err != nil {
		return Document{}, err
	}
	configPath, err := resolveConfigPath(options.ConfigPath)
	if err != nil {
		return Document{}, err
	}
	if err := ensurePrivateDirectory(filepath.Dir(configPath)); err != nil {
		return Document{}, err
	}
	if err := validateSaveTarget(configPath); err != nil {
		return Document{}, err
	}

	body, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return Document{}, fmt.Errorf("%w: encode document", ErrStoreBlocked)
	}
	body = append(body, '\n')

	temporary, err := os.CreateTemp(filepath.Dir(configPath), ".preferences-*.tmp")
	if err != nil {
		return Document{}, fmt.Errorf("%w: stage document", ErrStoreBlocked)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return Document{}, fmt.Errorf("%w: protect staged document", ErrStoreBlocked)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return Document{}, fmt.Errorf("%w: stage document", ErrStoreBlocked)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return Document{}, fmt.Errorf("%w: stage document", ErrStoreBlocked)
	}
	if err := temporary.Close(); err != nil {
		return Document{}, fmt.Errorf("%w: stage document", ErrStoreBlocked)
	}
	if hooks != nil && hooks.beforeRename != nil {
		if err := hooks.beforeRename(temporaryPath, configPath); err != nil {
			return Document{}, fmt.Errorf("%w: install document", ErrStoreBlocked)
		}
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		return Document{}, fmt.Errorf("%w: install document", ErrStoreBlocked)
	}
	return normalized, nil
}

// Resolve applies CLI-overrides-first precedence without reading or writing
// process state. The current document and locale environment are injected.
func Resolve(options ResolveOptions) (Effective, error) {
	if err := validateDocumentFields(options.Current); err != nil {
		return Effective{}, err
	}
	selectedLanguage := options.Current.Language
	if options.LanguageOverride != "" {
		if !validLanguage(options.LanguageOverride) {
			return Effective{}, fmt.Errorf("%w: language override", ErrInvalidPreferences)
		}
		selectedLanguage = options.LanguageOverride
	}
	if selectedLanguage == LanguageAuto {
		selectedLanguage = languageFromLocale(options.Locale)
	}

	selectedDensity := options.Current.Density
	if options.DensityOverride != "" {
		if !validDensity(options.DensityOverride) {
			return Effective{}, fmt.Errorf("%w: density override", ErrInvalidPreferences)
		}
		selectedDensity = options.DensityOverride
	}
	return Effective{
		Language:              selectedLanguage,
		Density:               selectedDensity,
		PreferredAssetLibrary: options.Current.PreferredAssetLibrary,
	}, nil
}

func resolveConfigPath(candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return DefaultPath()
	}
	if candidate == "~" || strings.HasPrefix(candidate, "~/") {
		return "", fmt.Errorf("%w: preference path", ErrStoreBlocked)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: preference path", ErrStoreBlocked)
	}
	return filepath.Clean(absolute), nil
}

func decodeDocument(body []byte) (Document, error) {
	var document Document
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("%w: decode document", ErrInvalidPreferences)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Document{}, fmt.Errorf("%w: trailing document content", ErrInvalidPreferences)
	}
	if err := validateDocumentFields(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func normalizeDocument(document Document, home, project string) (Document, error) {
	if err := validateDocumentFields(document); err != nil {
		return Document{}, err
	}
	if document.PreferredAssetLibrary == "" {
		return document, nil
	}
	normalized, err := validatePreferredLibrary(
		document.PreferredAssetLibrary,
		home,
		project,
	)
	if err != nil {
		return Document{}, err
	}
	document.PreferredAssetLibrary = normalized
	return document, nil
}

func validateDocumentFields(document Document) error {
	if document.SchemaVersion != schemaVersion ||
		!validLanguage(document.Language) ||
		!validDensity(document.Density) {
		return fmt.Errorf("%w: unsupported document fields", ErrInvalidPreferences)
	}
	if document.PreferredAssetLibrary != "" {
		value := document.PreferredAssetLibrary
		if strings.TrimSpace(value) != value ||
			value == "~" ||
			strings.HasPrefix(value, "~/") ||
			!filepath.IsAbs(value) {
			return fmt.Errorf("%w: preferred asset library", ErrInvalidPreferences)
		}
	}
	return nil
}

func validLanguage(value Language) bool {
	return value == LanguageAuto || value == LanguageZhCN || value == LanguageEnglish
}

func validDensity(value Density) bool {
	return value == DensityStandard || value == DensityDetailed
}

func validatePreferredLibrary(candidate, home, project string) (string, error) {
	if strings.TrimSpace(home) == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%w: preferred asset library", ErrInvalidPreferences)
		}
	}
	resolved, err := workspace.ValidateExistingRoot(candidate, home, project)
	if err != nil {
		return "", fmt.Errorf("%w: preferred asset library", ErrInvalidPreferences)
	}
	return resolved, nil
}

func privateDirectory(info os.FileInfo) bool {
	return info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0o700
}

func privateRegularFile(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Mode().Perm() == 0o600
}

func ensurePrivateDirectory(directory string) error {
	info, err := os.Lstat(directory)
	switch {
	case err == nil:
		if !privateDirectory(info) {
			return fmt.Errorf("%w: unsafe config directory", ErrStoreBlocked)
		}
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%w: inspect config directory", ErrStoreBlocked)
	}

	missing := make([]string, 0, 2)
	current := filepath.Clean(directory)
	for {
		info, err = os.Lstat(current)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: unsafe config ancestor", ErrStoreBlocked)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: inspect config ancestor", ErrStoreBlocked)
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("%w: config directory", ErrStoreBlocked)
		}
		current = parent
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := os.Mkdir(missing[index], 0o700); err != nil {
			return fmt.Errorf("%w: create config directory", ErrStoreBlocked)
		}
		created, err := os.Lstat(missing[index])
		if err != nil || !privateDirectory(created) {
			return fmt.Errorf("%w: protect config directory", ErrStoreBlocked)
		}
	}
	return nil
}

func validateSaveTarget(configPath string) error {
	info, err := os.Lstat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !privateRegularFile(info) {
		return fmt.Errorf("%w: unsafe preference target", ErrStoreBlocked)
	}
	return nil
}

func languageFromLocale(locale LocaleEnvironment) Language {
	value := locale.LCAll
	if value == "" {
		value = locale.LCMessages
	}
	if value == "" {
		value = locale.Lang
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "zh" ||
		strings.HasPrefix(normalized, "zh_") ||
		strings.HasPrefix(normalized, "zh-") {
		return LanguageZhCN
	}
	return LanguageEnglish
}
