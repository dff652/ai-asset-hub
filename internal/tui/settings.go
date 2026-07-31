package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dff652/ai-asset-hub/internal/preferences"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type settingsItemKind int

const (
	settingsItemLanguage settingsItemKind = iota
	settingsItemDensity
	settingsItemPreferredLibrary
	settingsItemSave
	settingsItemReset
)

type settingsItem struct {
	kind        settingsItemKind
	section     messageID
	language    preferences.Language
	density     preferences.Density
	title       string
	description string
}

type preferencesSaveMsg struct {
	document preferences.Document
	err      error
}

func (m Model) withPreferences(
	store preferences.StoreOptions,
	report preferences.LoadReport,
	locale preferences.LocaleEnvironment,
	languageOverride preferences.Language,
	densityOverride preferences.Density,
	effective preferences.Effective,
) Model {
	m.preferenceStore = store
	m.preferencePath = report.ConfigPath
	m.preferenceWarnings = append([]preferences.WarningCode(nil), report.Warnings...)
	m.currentPreferences = report.Preferences
	m.localeEnvironment = locale
	m.languageOverride = languageOverride
	m.densityOverride = densityOverride
	m.autoLanguage = resolvedLanguage(preferences.Document{
		SchemaVersion: report.Preferences.SchemaVersion,
		Language:      preferences.LanguageAuto,
		Density:       report.Preferences.Density,
	}, locale, "")
	m.language = tuiLanguage(effective.Language)
	m.density = effective.Density
	m.resetDiffExpansionForDensity()
	m.syncLocalizedInputs()
	return m
}

func (m Model) startSettings() (tea.Model, tea.Cmd) {
	m.screen = screenSettings
	m.settingsDraft = m.currentPreferences
	m.settingsCursor = settingsLanguageIndex(m.settingsDraft.Language)
	m.settingsDirty = false
	m.settingsSaving = false
	m.settingsNotice = ""
	m.settingsErr = nil
	return m, nil
}

func (m Model) updateSettingsKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.settingsItems()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Collapse) || key.Matches(message, m.keys.Home):
		m.discardSettings()
	case key.Matches(message, m.keys.Up):
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.settingsCursor+1 < len(items) {
			m.settingsCursor++
		}
	case key.Matches(message, m.keys.First):
		m.settingsCursor = 0
	case key.Matches(message, m.keys.Last):
		m.settingsCursor = len(items) - 1
	case key.Matches(message, m.keys.Expand):
		item := items[m.settingsCursor]
		switch item.kind {
		case settingsItemLanguage:
			m.settingsDraft.Language = item.language
			m.settingsDirty = m.settingsDraft != m.currentPreferences
			m.settingsNotice = ""
			m.settingsErr = nil
			m.previewSettings()
		case settingsItemDensity:
			m.settingsDraft.Density = item.density
			m.settingsDirty = m.settingsDraft != m.currentPreferences
			m.settingsNotice = ""
			m.settingsErr = nil
			m.previewSettings()
		case settingsItemPreferredLibrary:
			m.editingPreferred = true
			m.preferredInput.SetValue(m.settingsDraft.PreferredAssetLibrary)
			m.settingsNotice = ""
			m.settingsErr = nil
			return m, m.preferredInput.Focus()
		case settingsItemSave:
			m.settingsSaving = true
			m.settingsNotice = m.text(msgSettingsSaving)
			m.settingsErr = nil
			return m, savePreferencesCommand(m.preferenceStore, m.settingsDraft)
		case settingsItemReset:
			m.settingsDraft = preferences.Defaults()
			m.settingsDirty = m.settingsDraft != m.currentPreferences
			m.settingsNotice = m.text(msgSettingsResetReady)
			m.settingsErr = nil
			m.previewSettings()
		}
	}
	return m, nil
}

func (m Model) updatePreferredInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.editingPreferred = false
		m.preferredInput.SetValue("")
		m.preferredInput.Blur()
		m.settingsNotice = ""
		m.settingsErr = nil
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		candidate := strings.TrimSpace(m.preferredInput.Value())
		if candidate == "" {
			m.settingsDraft.PreferredAssetLibrary = ""
		} else {
			normalized, err := workspace.ValidateExistingRoot(
				candidate,
				m.options.Home,
				m.options.Project,
			)
			if err != nil {
				m.settingsNotice = m.text(msgSettingsLibraryInvalid)
				m.settingsErr = err
				return m, nil
			}
			m.settingsDraft.PreferredAssetLibrary = normalized
		}
		m.editingPreferred = false
		m.preferredInput.SetValue("")
		m.preferredInput.Blur()
		m.settingsDirty = m.settingsDraft != m.currentPreferences
		m.settingsNotice = ""
		m.settingsErr = nil
		return m, nil
	}
	var command tea.Cmd
	m.preferredInput, command = m.preferredInput.Update(message)
	return m, command
}

func (m *Model) previewSettings() {
	effective, err := preferences.Resolve(preferences.ResolveOptions{
		Current: m.settingsDraft,
		Locale:  m.localeEnvironment,
	})
	if err != nil {
		return
	}
	m.language = tuiLanguage(effective.Language)
	m.density = effective.Density
	m.syncLocalizedInputs()
}

func (m *Model) discardSettings() {
	m.applyRuntimeLanguage()
	m.settingsDraft = m.currentPreferences
	m.settingsDirty = false
	m.settingsNotice = ""
	m.settingsErr = nil
	m.screen = screenHome
}

func savePreferencesCommand(
	store preferences.StoreOptions,
	document preferences.Document,
) tea.Cmd {
	return func() tea.Msg {
		normalized, err := preferences.Save(store, document)
		return preferencesSaveMsg{document: normalized, err: err}
	}
}

func (m Model) handlePreferencesSave(message preferencesSaveMsg) (tea.Model, tea.Cmd) {
	m.settingsSaving = false
	if message.err != nil {
		m.settingsDraft = m.currentPreferences
		m.settingsDirty = false
		m.settingsCursor = settingsLanguageIndex(m.settingsDraft.Language)
		m.settingsErr = message.err
		m.applyRuntimeLanguage()
		m.settingsNotice = m.text(msgSettingsSaveFailed)
		return m, nil
	}
	m.currentPreferences = message.document
	m.settingsDraft = message.document
	m.settingsDirty = false
	m.preferenceWarnings = nil
	m.settingsErr = nil
	m.applyRuntimeLanguage()
	m.settingsNotice = m.text(msgSettingsSaved)
	return m, nil
}

func (m *Model) applyRuntimePreferences() {
	effective, err := preferences.Resolve(preferences.ResolveOptions{
		Current:          m.currentPreferences,
		Locale:           m.localeEnvironment,
		LanguageOverride: m.languageOverride,
		DensityOverride:  m.densityOverride,
	})
	if err != nil {
		return
	}
	m.language = tuiLanguage(effective.Language)
	m.density = effective.Density
	m.syncLocalizedInputs()
}

func (m *Model) applyRuntimeLanguage() {
	m.applyRuntimePreferences()
}

func (m *Model) resetDiffExpansionForDensity() {
	detailed := m.density == preferences.DensityDetailed
	m.diffExpanded["action:create"] = true
	m.diffExpanded["action:update"] = true
	m.diffExpanded["action:unchanged"] = detailed
	m.diffExpanded["action:skipped"] = detailed
	m.diffExpanded["findings"] = true
	m.clampDiffCursor()
}

func resolvedLanguage(
	document preferences.Document,
	locale preferences.LocaleEnvironment,
	override preferences.Language,
) language {
	effective, err := preferences.Resolve(preferences.ResolveOptions{
		Current:          document,
		Locale:           locale,
		LanguageOverride: override,
	})
	if err != nil {
		return languageEnglish
	}
	return tuiLanguage(effective.Language)
}

func tuiLanguage(value preferences.Language) language {
	if value == preferences.LanguageZhCN {
		return languageZhCN
	}
	return languageEnglish
}

func settingsLanguageIndex(value preferences.Language) int {
	switch value {
	case preferences.LanguageZhCN:
		return 1
	case preferences.LanguageEnglish:
		return 2
	default:
		return 0
	}
}

func (m Model) settingsItems() []settingsItem {
	return []settingsItem{
		{
			kind:        settingsItemLanguage,
			section:     msgSettingsLanguageTitle,
			language:    preferences.LanguageAuto,
			title:       m.text(msgSettingsLanguageAuto, m.languageLabel(m.autoLanguage)),
			description: m.text(msgSettingsLanguageAutoDesc),
		},
		{
			kind:        settingsItemLanguage,
			language:    preferences.LanguageZhCN,
			title:       m.text(msgSettingsLanguageZhCN),
			description: m.text(msgSettingsLanguageZhCNDesc),
		},
		{
			kind:        settingsItemLanguage,
			language:    preferences.LanguageEnglish,
			title:       m.text(msgSettingsLanguageEnglish),
			description: m.text(msgSettingsLanguageEnglishDesc),
		},
		{
			kind:        settingsItemDensity,
			section:     msgSettingsDensityTitle,
			density:     preferences.DensityStandard,
			title:       m.text(msgSettingsDensityStandard),
			description: m.text(msgSettingsDensityStandardDesc),
		},
		{
			kind:        settingsItemDensity,
			density:     preferences.DensityDetailed,
			title:       m.text(msgSettingsDensityDetailed),
			description: m.text(msgSettingsDensityDetailedDesc),
		},
		{
			kind:        settingsItemPreferredLibrary,
			section:     msgSettingsLibraryTitle,
			title:       m.preferredLibraryLabel(),
			description: m.text(msgSettingsLibraryDesc),
		},
		{
			kind:        settingsItemSave,
			section:     msgSettingsActionsTitle,
			title:       m.text(msgSettingsSaveTitle),
			description: m.text(msgSettingsSaveDesc),
		},
		{
			kind:        settingsItemReset,
			title:       m.text(msgSettingsResetTitle),
			description: m.text(msgSettingsResetDesc),
		},
	}
}

func (m Model) preferredLibraryLabel() string {
	if m.settingsDraft.PreferredAssetLibrary == "" {
		return m.text(msgSettingsLibraryNone)
	}
	return m.text(msgSettingsLibrarySelected, m.settingsDraft.PreferredAssetLibrary)
}

func (m Model) languageLabel(value language) string {
	if value == languageZhCN {
		return m.text(msgSettingsEffectiveZhCN)
	}
	return m.text(msgSettingsEffectiveEnglish)
}

func (m Model) preferenceWarningLabel(code preferences.WarningCode) string {
	switch code {
	case preferences.WarningConfigPath:
		return m.text(msgSettingsWarningConfigPath)
	case preferences.WarningUnsafeStore:
		return m.text(msgSettingsWarningUnsafeStore)
	case preferences.WarningUnreadableStore:
		return m.text(msgSettingsWarningUnreadableStore)
	case preferences.WarningInvalidDocument:
		return m.text(msgSettingsWarningInvalidDocument)
	case preferences.WarningPreferredLibrary:
		return m.text(msgSettingsWarningPreferredLibrary)
	default:
		return m.text(msgSettingsWarningUnknown)
	}
}

func (m Model) settingsView(style styles) string {
	if m.editingPreferred {
		return m.preferredLibraryInputView(style)
	}
	state := m.text(msgSettingsStateSaved)
	if m.settingsDirty {
		state = m.text(msgSettingsStateUnsaved)
	}
	header := joinEdges(
		style.header.Render(m.text(msgSettingsTitle)),
		state,
		max(20, m.width),
	)
	lines := []string{
		header,
		style.muted.Render(m.text(msgSettingsSummary)),
	}
	for index, item := range m.settingsItems() {
		if item.section != "" {
			lines = append(lines, "", style.header.Render(m.text(item.section)))
		}
		prefix := "  "
		if index == m.settingsCursor {
			prefix = "> "
		}
		checked := "   "
		if item.kind == settingsItemLanguage &&
			item.language == m.settingsDraft.Language {
			checked = "[x]"
		} else if item.kind == settingsItemDensity &&
			item.density == m.settingsDraft.Density {
			checked = "[x]"
		}
		line := prefix + checked + " " +
			padRight(truncate(item.title, 30), 30) + " " + item.description
		line = truncate(line, max(20, m.width))
		if index == m.settingsCursor {
			line = style.selected.Render(line)
		}
		lines = append(lines, line)
	}
	if m.settingsDraft.PreferredAssetLibrary != "" {
		lines = append(
			lines,
			"",
			style.muted.Render(m.text(
				msgSettingsLibraryCurrent,
				m.settingsDraft.PreferredAssetLibrary,
			)),
		)
	}
	lines = append(lines, "", m.text(msgSettingsPath, m.preferencePath))
	if m.languageOverride != "" {
		lines = append(
			lines,
			style.muted.Render(
				m.text(msgSettingsOverride, string(m.languageOverride)),
			),
		)
	}
	if m.densityOverride != "" {
		lines = append(
			lines,
			style.muted.Render(
				m.text(msgSettingsDensityOverride, string(m.densityOverride)),
			),
		)
	}
	if len(m.preferenceWarnings) > 0 {
		lines = append(lines, "", style.warning.Render(m.text(msgSettingsWarningsTitle)))
		for _, warning := range m.preferenceWarnings {
			lines = append(lines, style.warning.Render("  "+m.preferenceWarningLabel(warning)))
		}
	}
	if m.settingsNotice != "" {
		noticeStyle := style.muted
		if m.settingsErr != nil {
			noticeStyle = style.error
		}
		lines = append(lines, "", noticeStyle.Render(m.settingsNotice))
	}
	lines = append(lines, "", style.muted.Render(m.text(msgSettingsFooter)))
	return strings.Join(lines, "\n")
}

func (m Model) preferredLibraryInputView(style styles) string {
	lines := []string{
		style.header.Render(m.text(msgSettingsLibraryEditTitle)),
		"",
		m.text(msgSettingsLibraryEditPrompt),
		m.preferredInput.View(),
		"",
		m.text(msgSettingsLibraryEditBoundary),
		m.text(msgSettingsLibraryEditFooter),
	}
	if m.settingsNotice != "" {
		lines = append(lines, "", style.error.Render(m.settingsNotice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) settingsHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render(m.text(msgSettingsHelpTitle)),
		"",
		m.text(msgSettingsHelpChoose),
		m.text(msgSettingsHelpPreview),
		m.text(msgSettingsHelpLibrary),
		m.text(msgSettingsHelpSave),
		m.text(msgSettingsHelpReset),
		m.text(msgSettingsHelpOverride),
		m.text(msgSettingsHelpCancel),
		"",
		m.text(msgSettingsHelpBoundary),
	}, "\n")
}
