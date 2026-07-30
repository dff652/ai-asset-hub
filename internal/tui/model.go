package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/preferences"
	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type status int

const (
	statusLoading status = iota
	statusReady
	statusFailed
)

type scanMsg struct {
	generation int
	report     inventory.Report
	err        error
}

type screen int

const (
	screenHome screen = iota
	screenInventory
	screenDeployment
	screenHealth
	screenVersion
	screenMigration
	screenSettings
)

type Model struct {
	options            inventory.Options
	report             inventory.Report
	filterInput        textinput.Model
	filtering          bool
	workspaceInput     textinput.Model
	choosingWorkspace  bool
	preparingWorkspace bool
	profileInput       textinput.Model
	choosingProfile    bool
	availableProfiles  []string
	building           bool
	profilePurpose     profilePurpose
	findingsOnly       bool
	cursor             int
	expanded           map[string]bool
	status             status
	err                error
	width              int
	height             int
	showHelp           bool
	generation         int
	keys               keyMap
	plain              bool
	language           language
	homeEnabled        bool
	homeCursor         int
	afterWorkspace     homeAction

	// workspace is empty until the user names one by flag or in the path
	// prompt. The UI is read-only until then (ADR-0006 §2).
	workspace     string
	selected      map[string]bool
	composing     bool
	notice        string
	noticeIsWarn  bool
	lastFindings  []workspace.ComposeFinding
	catalog       workspace.CatalogReport
	catalogErr    error
	managing      bool
	manageAction  string
	manageInput   textinput.Model
	confirmManage bool

	screen           screen
	deployOptions    apply.Options
	packageFromBuild bool
	diffStatus       status
	diffReport       apply.Report
	deployErr        error
	diffCursor       int
	diffExpanded     map[string]bool
	confirming       bool
	confirmInput     textinput.Model
	applying         bool
	applyResult      *apply.Report

	maintenance        bool
	doctorStatus       status
	doctorReport       apply.DoctorReport
	doctorErr          error
	healthCursor       int
	rollbackConfirming bool
	rollbackInput      textinput.Model
	rollbacking        bool
	rollbackResult     *apply.RollbackReport
	rollbackErr        error

	updateChecking bool
	updateChecked  bool
	updateReport   updater.Report
	updateErr      error

	migrationFlow migrationFlow

	preferenceStore    preferences.StoreOptions
	preferencePath     string
	preferenceWarnings []preferences.WarningCode
	currentPreferences preferences.Document
	localeEnvironment  preferences.LocaleEnvironment
	languageOverride   preferences.Language
	autoLanguage       language
	settingsDraft      preferences.Document
	settingsCursor     int
	settingsDirty      bool
	settingsSaving     bool
	settingsNotice     string
	settingsErr        error
}

func NewModel(options inventory.Options) Model {
	input := textinput.New()
	input.Prompt = "/ "
	input.CharLimit = 120
	input.Width = 36
	confirm := textinput.New()
	confirm.Prompt = "> "
	confirm.Placeholder = "apply"
	confirm.CharLimit = 5
	confirm.Width = 12
	rollbackInput := textinput.New()
	rollbackInput.Prompt = "> "
	rollbackInput.Placeholder = "rollback"
	rollbackInput.CharLimit = 8
	rollbackInput.Width = 14
	workspaceInput := textinput.New()
	workspaceInput.Prompt = "> "
	workspaceInput.Placeholder = "~/ai-assets"
	workspaceInput.CharLimit = 512
	workspaceInput.Width = 64
	profileInput := textinput.New()
	profileInput.Prompt = "> "
	profileInput.Placeholder = "personal"
	profileInput.CharLimit = 120
	profileInput.Width = 36
	manageInput := textinput.New()
	manageInput.Prompt = "> "
	manageInput.CharLimit = 6
	manageInput.Width = 12
	model := Model{
		options:        options,
		filterInput:    input,
		workspaceInput: workspaceInput,
		profileInput:   profileInput,
		manageInput:    manageInput,
		migrationFlow:  newMigrationFlow(),
		expanded:       make(map[string]bool),
		selected:       make(map[string]bool),
		diffExpanded: map[string]bool{
			"action:create":    true,
			"action:update":    true,
			"action:unchanged": true,
			"action:skipped":   true,
			"findings":         true,
		},
		confirmInput:       confirm,
		rollbackInput:      rollbackInput,
		screen:             screenInventory,
		status:             statusLoading,
		width:              100,
		height:             30,
		generation:         1,
		keys:               defaultKeys(),
		language:           languageZhCN,
		currentPreferences: preferences.Defaults(),
		autoLanguage:       languageZhCN,
	}
	model.syncLocalizedInputs()
	return model
}

// WithWorkspace enables Phase B composition against the given workspace root.
// Without it the model can only read.
func (m Model) WithWorkspace(root string) Model {
	m.workspace = root
	return m
}

// WithHome makes the task-oriented home screen the first page. NewModel keeps
// inventory as its default so focused model tests and deployment-only callers
// do not silently change behavior.
func (m Model) WithHome(enabled bool) Model {
	m.homeEnabled = enabled
	if enabled && m.deployOptions.Package == "" {
		m.screen = screenHome
	}
	return m
}

// WithMaintenance enables doctor and rollback in the regular `aiah`
// workflow. Bootstrap deliberately keeps its existing deployment-only result
// contract and does not enable these actions.
func (m Model) WithMaintenance(enabled bool) Model {
	m.maintenance = enabled
	return m
}

// WithDeployment enables Phase C for one explicit package. It starts on the
// read-only diff screen; apply still requires typing "apply" in the TUI.
func (m Model) WithDeployment(options apply.Options) Model {
	options.DryRun = false
	m.deployOptions = options
	if options.Package == "" {
		return m
	}
	m.screen = screenDeployment
	m.diffStatus = statusLoading
	return m
}

func (m *Model) invalidateBuiltPackage() {
	if !m.packageFromBuild {
		return
	}
	m.deployOptions.Package = ""
	m.packageFromBuild = false
	m.diffReport = apply.Report{}
	m.deployErr = nil
	m.applyResult = nil
	m.diffCursor = 0
}

func (m Model) Init() tea.Cmd {
	commands := []tea.Cmd{scanCommand(m.options, m.generation)}
	if m.deployOptions.Package != "" {
		commands = append(commands, diffCommand(m.deployOptions))
	}
	if m.homeEnabled && m.maintenance {
		commands = append(commands, doctorCommand(m.doctorOptions()))
	}
	if len(commands) == 1 {
		return commands[0]
	}
	return tea.Batch(commands...)
}

func scanCommand(options inventory.Options, generation int) tea.Cmd {
	return func() tea.Msg {
		report, err := inventory.Scan(options)
		return scanMsg{generation: generation, report: report, err: err}
	}
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.filterInput.Width = max(10, min(48, message.Width-6))
		m.workspaceInput.Width = max(10, min(64, message.Width-6))
		m.migrationFlow.channelInput.Width = max(10, min(64, message.Width-6))
		m.migrationFlow.pullOutInput.Width = max(10, min(64, message.Width-6))
		return m, nil
	case scanMsg:
		if message.generation != m.generation {
			return m, nil
		}
		if message.err != nil {
			m.status = statusFailed
			m.err = message.err
			m.cursor = 0
			return m, nil
		}
		m.report = message.report
		m.status = statusReady
		m.err = nil
		m.ensureGroupsExpanded()
		m.refreshCatalog()
		m.pruneSelection()
		m.clampCursor()
		return m, nil
	case composeMsg:
		m.composing = false
		m.notice, m.noticeIsWarn = m.composeNotice(message)
		m.lastFindings = message.result.Findings
		if message.err == nil && message.result.Ok {
			if len(message.result.Registered) > 0 {
				m.invalidateBuiltPackage()
				for key := range m.selected {
					delete(m.selected, key)
				}
				m.refreshCatalog()
				return m.startProfileInput()
			}
			// Registered assets are now the workspace's; keeping them ticked
			// would invite a second write that can only report duplicates.
			for _, asset := range m.selectedAssets() {
				delete(m.selected, asset.LogicalPath)
			}
		}
		return m, nil
	case manageMsg:
		m.managing = false
		m.confirmManage = false
		m.manageInput.Blur()
		m.notice, m.noticeIsWarn = m.manageNotice(message)
		if message.err == nil && message.ok {
			m.invalidateBuiltPackage()
			for key := range m.selected {
				delete(m.selected, key)
			}
			m.refreshCatalog()
			return m.startProfileInput()
		}
		return m, nil
	case workspaceMsg:
		m.preparingWorkspace = false
		if message.err != nil {
			m.notice = m.text(msgWorkspaceUnavailable, message.err)
			m.noticeIsWarn = true
			return m, nil
		}
		m.workspace = message.root
		m.refreshCatalog()
		m.workspaceInput.SetValue("")
		if message.created {
			m.notice = m.text(msgWorkspaceCreated, message.root)
		} else {
			m.notice = m.text(msgWorkspaceOpened, message.root)
		}
		m.noticeIsWarn = false
		next := m.afterWorkspace
		m.afterWorkspace = homeActionNone
		switch next {
		case homeActionOrganize:
			m.screen = screenInventory
		case homeActionApply:
			return m.startProfileInput()
		case homeActionMigration:
			return m.startMigration()
		}
		return m, nil
	case buildMsg:
		m.building = false
		if message.err != nil {
			m.invalidateBuiltPackage()
			m.notice = m.text(msgProfileBuildCommandFailed, message.err)
			m.noticeIsWarn = true
			return m, nil
		}
		if !message.report.Ok || message.report.Package == nil {
			m.invalidateBuiltPackage()
			m.notice = m.buildFailureNotice(message.report)
			m.noticeIsWarn = true
			return m, nil
		}
		if message.purpose == profileForPublish {
			return m.startPublishConfirmation(message.packagePath)
		}
		m.deployOptions.Package = message.packagePath
		m.deployOptions.DryRun = false
		m.packageFromBuild = true
		m.screen = screenDeployment
		m.diffStatus = statusLoading
		m.deployErr = nil
		m.applyResult = nil
		m.diffCursor = 0
		m.notice = m.text(msgProfileBuildReady)
		m.noticeIsWarn = false
		return m, diffCommand(m.deployOptions)
	case diffMsg:
		if message.err != nil {
			m.diffStatus = statusFailed
			m.deployErr = message.err
			m.diffCursor = 0
			return m, nil
		}
		m.diffReport = message.report
		m.diffStatus = statusReady
		m.deployErr = nil
		m.applyResult = nil
		m.clampDiffCursor()
		return m, nil
	case applyMsg:
		m.applying = false
		m.confirming = false
		m.confirmInput.Blur()
		m.deployErr = message.err
		m.applyResult = &message.report
		if message.err == nil && message.report.Ok {
			m.generation++
			m.status = statusLoading
			if m.maintenance {
				m.doctorStatus = statusLoading
				return m, tea.Batch(
					scanCommand(m.options, m.generation),
					doctorCommand(m.doctorOptions()),
				)
			}
			return m, scanCommand(m.options, m.generation)
		}
		return m, nil
	case doctorMsg:
		m.doctorErr = message.err
		if message.err != nil {
			m.doctorStatus = statusFailed
			m.healthCursor = 0
			return m, nil
		}
		m.doctorReport = message.report
		m.doctorStatus = statusReady
		m.clampHealthCursor()
		return m, nil
	case rollbackMsg:
		m.rollbacking = false
		m.rollbackConfirming = false
		m.rollbackInput.Blur()
		m.rollbackErr = message.err
		m.rollbackResult = &message.report
		if message.err == nil && message.report.Ok {
			m.doctorStatus = statusLoading
			m.generation++
			m.status = statusLoading
			return m, tea.Batch(
				doctorCommand(m.doctorOptions()),
				scanCommand(m.options, m.generation),
			)
		}
		return m, nil
	case updateCheckMsg:
		m.updateChecking = false
		m.updateChecked = true
		m.updateReport = message.report
		m.updateErr = message.err
		return m, nil
	case preferencesSaveMsg:
		return m.handlePreferencesSave(message)
	case migrationMsg:
		return m.handleMigrationMessage(message)
	case preflightMsg:
		return m.handlePreflightMessage(message)
	case versionsMsg:
		return m.handleVersionsMessage(message)
	case publishMsg:
		return m.handlePublishMessage(message)
	case pullMsg:
		return m.handlePullMessage(message)
	case tea.KeyMsg:
		return m.updateKey(message)
	}
	return m, nil
}

func (m *Model) refreshCatalog() {
	m.catalog = workspace.CatalogReport{}
	m.catalogErr = nil
	if m.workspace == "" || m.status != statusReady {
		return
	}
	report, err := workspace.Catalog(workspace.CatalogOptions{
		WorkspaceRoot: m.workspace,
		Home:          m.options.Home,
		Project:       m.options.Project,
		Assets:        m.report.Assets,
	})
	m.catalog = report
	m.catalogErr = err
}

func (m Model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applying || m.rollbacking || m.managing ||
		m.migrationFlow.publishing || m.migrationFlow.pulling || m.settingsSaving {
		m.notice = m.text(msgCommonWriteInProgress)
		m.noticeIsWarn = true
		return m, nil
	}
	if key.Matches(message, m.keys.ForceQuit) {
		return m, tea.Quit
	}
	if m.preparingWorkspace || m.building {
		return m, nil
	}
	if m.choosingWorkspace {
		return m.updateWorkspaceInput(message)
	}
	if m.migrationFlow.choosingChannel {
		return m.updateChannelInput(message)
	}
	if m.migrationFlow.choosingPullOut {
		return m.updatePullOutput(message)
	}
	if m.choosingProfile {
		return m.updateProfileInput(message)
	}
	if m.migrationFlow.publishConfirming {
		return m.updatePublishConfirmation(message)
	}
	if m.confirmManage {
		return m.updateManageConfirmation(message)
	}
	if m.confirming {
		return m.updateConfirmation(message)
	}
	if m.rollbackConfirming {
		return m.updateRollbackConfirmation(message)
	}
	if m.showHelp {
		if key.Matches(message, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(message, m.keys.Help) || key.Matches(message, m.keys.Collapse) {
			m.showHelp = false
		}
		return m, nil
	}
	if m.filtering {
		if key.Matches(message, m.keys.Collapse) {
			m.filtering = false
			m.filterInput.Blur()
			m.clampCursor()
			return m, nil
		}
		var command tea.Cmd
		previous := m.filterInput.Value()
		m.filterInput, command = m.filterInput.Update(message)
		if m.filterInput.Value() != previous {
			m.cursor = 0
			m.clampCursor()
		}
		return m, command
	}
	if m.homeEnabled && m.screen != screenHome && key.Matches(message, m.keys.Home) {
		m.screen = screenHome
		m.showHelp = false
		m.notice = ""
		m.noticeIsWarn = false
		return m, nil
	}
	if m.screen == screenHome {
		return m.updateHomeKey(message)
	}
	if m.screen == screenDeployment {
		return m.updateDeploymentKey(message)
	}
	if m.screen == screenHealth {
		return m.updateHealthKey(message)
	}
	if m.screen == screenVersion {
		return m.updateVersionKey(message)
	}
	if m.screen == screenMigration {
		return m.updateMigrationKey(message)
	}
	if m.screen == screenSettings {
		return m.updateSettingsKey(message)
	}

	rows := m.visibleRows()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Filter):
		m.filtering = true
		return m, m.filterInput.Focus()
	case key.Matches(message, m.keys.FindingsOnly):
		m.findingsOnly = !m.findingsOnly
		m.cursor = 0
		m.clampCursor()
	case key.Matches(message, m.keys.Reload):
		m.generation++
		m.status = statusLoading
		m.err = nil
		return m, scanCommand(m.options, m.generation)
	case key.Matches(message, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.cursor+1 < len(rows) {
			m.cursor++
		}
	case key.Matches(message, m.keys.First):
		m.cursor = 0
	case key.Matches(message, m.keys.Last):
		if len(rows) > 0 {
			m.cursor = len(rows) - 1
		}
	case key.Matches(message, m.keys.Select):
		m.toggleSelection(rows)
	case key.Matches(message, m.keys.Write):
		if m.workspace == "" {
			return m.startWorkspaceInput()
		}
		return m.startCompose()
	case key.Matches(message, m.keys.UpdateAsset):
		return m.startAssetUpdate()
	case key.Matches(message, m.keys.RemoveAsset):
		return m.startAssetRemove()
	case key.Matches(message, m.keys.Build):
		return m.startProfileInput()
	case key.Matches(message, m.keys.Diff):
		return m.startDiff()
	case key.Matches(message, m.keys.Doctor):
		return m.startDoctor()
	case key.Matches(message, m.keys.Version):
		return m.startVersion()
	case key.Matches(message, m.keys.Expand):
		m.setCurrentExpanded(rows, true)
	case key.Matches(message, m.keys.Collapse):
		m.setCurrentExpanded(rows, false)
	}
	return m, nil
}
