package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/migration"
)

type migrationMode int

const (
	migrationModeStatus migrationMode = iota
	migrationModeVersions
	migrationModePreflight
)

type migrationAction int

const (
	migrationActionNone migrationAction = iota
	migrationActionPublish
	migrationActionVersions
)

type migrationFlow struct {
	status          status
	report          migration.Report
	err             error
	channel         string
	channelInput    textinput.Model
	choosingChannel bool

	afterChannel   migrationAction
	afterMigration migrationAction
	mode           migrationMode

	versionsStatus status
	versionsReport channel.ListReport
	versionsErr    error
	versionsCursor int

	preflightStatus  status
	preflightReport  migration.PreflightReport
	preflightErr     error
	preflightProfile string
	preflightCursor  int
	pulledReport     channel.PullReport

	publishPackage    string
	publishConfirming bool
	publishInput      textinput.Model
	publishing        bool

	selectedRelease channel.Release
	choosingPullOut bool
	pullOutInput    textinput.Model
	pulling         bool
}

func newMigrationFlow() migrationFlow {
	channelInput := textinput.New()
	channelInput.Prompt = "> "
	channelInput.Placeholder = "/mnt/usb/aiah"
	channelInput.CharLimit = 512
	channelInput.Width = 64

	publishInput := textinput.New()
	publishInput.Prompt = "> "
	publishInput.Placeholder = "publish"
	publishInput.CharLimit = 7
	publishInput.Width = 14

	pullOutInput := textinput.New()
	pullOutInput.Prompt = "> "
	pullOutInput.Placeholder = "/tmp/aiah-incoming"
	pullOutInput.CharLimit = 512
	pullOutInput.Width = 64

	return migrationFlow{
		channelInput: channelInput,
		mode:         migrationModeStatus,
		publishInput: publishInput,
		pullOutInput: pullOutInput,
	}
}

type versionsMsg struct {
	report channel.ListReport
	err    error
}

type publishMsg struct {
	report channel.PublishReport
	err    error
}

type pullMsg struct {
	report channel.PullReport
	err    error
}

func versionsCommand(options channel.ListOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := channel.List(options)
		return versionsMsg{report: report, err: err}
	}
}

func publishCommand(options channel.PublishOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := channel.Publish(options)
		return publishMsg{report: report, err: err}
	}
}

func pullCommand(options channel.PullOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := channel.Pull(options)
		return pullMsg{report: report, err: err}
	}
}

func (m Model) handleVersionsMessage(message versionsMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.versionsStatus = statusReady
	m.migrationFlow.versionsReport = message.report
	m.migrationFlow.versionsErr = message.err
	if message.err != nil {
		m.migrationFlow.versionsStatus = statusFailed
		m.migrationFlow.versionsCursor = 0
		return m, nil
	}
	if len(message.report.Releases) > 0 {
		m.migrationFlow.versionsCursor = len(message.report.Releases) - 1
	}
	return m, nil
}

func (m Model) handlePublishMessage(message publishMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.publishing = false
	if message.err != nil || !message.report.Ok {
		m.notice = m.text(msgMigrationPublishFailed, m.operationError(message.err))
		m.noticeIsWarn = true
		return m, nil
	}
	action := m.text(msgMigrationPublished)
	if message.report.Unchanged {
		action = m.text(msgMigrationPublishedUnchanged)
	}
	m.notice = m.text(
		msgMigrationPublishedResult,
		action,
		message.report.Name,
		message.report.Version,
		message.report.Profile,
	)
	m.noticeIsWarn = false
	m.migrationFlow.status = statusLoading
	return m, migrationCommand(m.migrationOptions())
}

func (m Model) handlePullMessage(message pullMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.pulling = false
	if message.err != nil || !message.report.Ok {
		m.notice = m.text(msgMigrationPullFailed, m.operationError(message.err))
		m.noticeIsWarn = true
		return m, nil
	}
	m.migrationFlow.pulledReport = message.report
	m.migrationFlow.preflightProfile = message.report.Profile
	m.migrationFlow.preflightStatus = statusLoading
	m.migrationFlow.preflightErr = nil
	m.migrationFlow.preflightCursor = 0
	m.migrationFlow.mode = migrationModePreflight
	m.screen = screenMigration
	m.notice = m.text(
		msgMigrationPulledChecking,
		message.report.Name,
		message.report.Version,
		message.report.Profile,
	)
	m.noticeIsWarn = false
	return m, packagePreflightCommand(m.packagePreflightOptions())
}

func (m Model) operationError(err error) string {
	if err == nil {
		return m.text(msgMigrationOperationNoSuccess)
	}
	return err.Error()
}

func (m Model) startMigrationPublish() (tea.Model, tea.Cmd) {
	if m.migrationFlow.status != statusReady {
		m.notice = m.text(msgMigrationStatusUnavailableChannel)
		m.noticeIsWarn = true
		return m, nil
	}
	if strings.TrimSpace(m.migrationFlow.channel) == "" {
		return m.startChannelInputFor(migrationActionPublish)
	}
	m.migrationFlow.publishPackage = ""
	m.notice = m.text(msgMigrationPublishSelectProfile)
	m.noticeIsWarn = false
	return m.startProfileInputFor(profileForPublish)
}

func (m Model) startPublishConfirmation(packagePath string) (tea.Model, tea.Cmd) {
	m.screen = screenMigration
	m.migrationFlow.mode = migrationModeStatus
	m.migrationFlow.publishPackage = packagePath
	m.migrationFlow.publishConfirming = true
	m.migrationFlow.publishInput.SetValue("")
	m.migrationFlow.publishInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updatePublishConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(message, m.keys.Collapse) {
		m.migrationFlow.publishConfirming = false
		m.migrationFlow.publishPackage = ""
		m.migrationFlow.publishInput.SetValue("")
		m.migrationFlow.publishInput.Blur()
		m.notice = m.text(msgMigrationPublishCancelled)
		m.noticeIsWarn = false
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		if m.migrationFlow.publishInput.Value() != "publish" {
			m.notice = m.text(msgMigrationPublishMismatch)
			m.noticeIsWarn = true
			m.migrationFlow.publishInput.SetValue("")
			return m, nil
		}
		m.migrationFlow.publishConfirming = false
		m.migrationFlow.publishInput.Blur()
		m.migrationFlow.publishing = true
		m.notice = ""
		m.noticeIsWarn = false
		return m, publishCommand(channel.PublishOptions{
			Package: m.migrationFlow.publishPackage,
			Channel: m.migrationFlow.channel,
		})
	}
	var command tea.Cmd
	m.migrationFlow.publishInput, command = m.migrationFlow.publishInput.Update(message)
	return m, command
}

func (m Model) startVersions() (tea.Model, tea.Cmd) {
	if m.migrationFlow.status != statusReady {
		m.notice = m.text(msgMigrationStatusUnavailableChannel)
		m.noticeIsWarn = true
		return m, nil
	}
	if strings.TrimSpace(m.migrationFlow.channel) == "" {
		return m.startChannelInputFor(migrationActionVersions)
	}
	if strings.TrimSpace(m.migrationFlow.report.Library.Name) == "" {
		m.notice = m.text(msgMigrationLibraryNameUnknown)
		m.noticeIsWarn = true
		return m, nil
	}
	m.screen = screenMigration
	m.migrationFlow.mode = migrationModeVersions
	m.migrationFlow.versionsStatus = statusLoading
	m.migrationFlow.versionsErr = nil
	m.migrationFlow.versionsCursor = 0
	m.notice = ""
	m.noticeIsWarn = false
	return m, versionsCommand(channel.ListOptions{
		Channel: m.migrationFlow.channel,
		Name:    m.migrationFlow.report.Library.Name,
	})
}

func (m Model) startPullOutput() (tea.Model, tea.Cmd) {
	if m.migrationFlow.versionsStatus != statusReady ||
		m.migrationFlow.versionsCursor < 0 ||
		m.migrationFlow.versionsCursor >= len(m.migrationFlow.versionsReport.Releases) {
		m.notice = m.text(msgMigrationPullSelectRelease)
		m.noticeIsWarn = true
		return m, nil
	}
	m.migrationFlow.selectedRelease =
		m.migrationFlow.versionsReport.Releases[m.migrationFlow.versionsCursor]
	m.migrationFlow.choosingPullOut = true
	m.migrationFlow.pullOutInput.SetValue("")
	m.migrationFlow.pullOutInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updatePullOutput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(message, m.keys.Collapse) {
		m.migrationFlow.choosingPullOut = false
		m.migrationFlow.pullOutInput.SetValue("")
		m.migrationFlow.pullOutInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		candidate := expandHomePath(
			strings.TrimSpace(m.migrationFlow.pullOutInput.Value()),
			m.options.Home,
		)
		if candidate == "" {
			m.notice = m.text(msgMigrationPullOutRequired)
			m.noticeIsWarn = true
			return m, nil
		}
		m.migrationFlow.choosingPullOut = false
		m.migrationFlow.pullOutInput.Blur()
		m.migrationFlow.pulling = true
		m.notice = ""
		m.noticeIsWarn = false
		return m, pullCommand(channel.PullOptions{
			Channel: m.migrationFlow.channel,
			Name:    m.migrationFlow.selectedRelease.Name,
			Version: m.migrationFlow.selectedRelease.Version,
			Profile: m.migrationFlow.selectedRelease.Profile,
			Out:     candidate,
		})
	}
	var command tea.Cmd
	m.migrationFlow.pullOutInput, command = m.migrationFlow.pullOutInput.Update(message)
	return m, command
}

func (m Model) updateMigrationKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.migrationFlow.mode == migrationModePreflight {
		return m.updateMigrationPreflightKey(message)
	}
	if m.migrationFlow.mode == migrationModeVersions {
		switch {
		case key.Matches(message, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(message, m.keys.Help):
			m.showHelp = true
		case key.Matches(message, m.keys.Up):
			if m.migrationFlow.versionsCursor > 0 {
				m.migrationFlow.versionsCursor--
			}
		case key.Matches(message, m.keys.Down):
			if m.migrationFlow.versionsCursor+1 < len(m.migrationFlow.versionsReport.Releases) {
				m.migrationFlow.versionsCursor++
			}
		case key.Matches(message, m.keys.First):
			m.migrationFlow.versionsCursor = 0
		case key.Matches(message, m.keys.Last):
			if len(m.migrationFlow.versionsReport.Releases) > 0 {
				m.migrationFlow.versionsCursor = len(m.migrationFlow.versionsReport.Releases) - 1
			}
		case key.Matches(message, m.keys.Expand):
			return m.startPullOutput()
		case key.Matches(message, m.keys.Reload):
			return m.startVersions()
		case key.Matches(message, m.keys.CheckUpdate):
			return m.startChannelInputFor(migrationActionVersions)
		case key.Matches(message, m.keys.Home):
			m.screen = screenHome
		case key.Matches(message, m.keys.Collapse):
			m.migrationFlow.mode = migrationModeStatus
			m.notice = ""
			m.noticeIsWarn = false
		}
		return m, nil
	}
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.CheckUpdate):
		return m.startChannelInput()
	case key.Matches(message, m.keys.Preflight):
		return m.startMigrationPreflight()
	case key.Matches(message, m.keys.Publish):
		return m.startMigrationPublish()
	case key.Matches(message, m.keys.Version):
		return m.startVersions()
	case key.Matches(message, m.keys.Reload):
		return m.startMigration()
	case key.Matches(message, m.keys.Home), key.Matches(message, m.keys.Collapse):
		m.screen = screenHome
	}
	return m, nil
}

func (m Model) updateMigrationPreflightKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.migrationFlow.preflightStatus == statusLoading {
		switch {
		case key.Matches(message, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(message, m.keys.Home):
			m.screen = screenHome
		case key.Matches(message, m.keys.Collapse):
			m.leaveMigrationPreflight()
		}
		return m, nil
	}
	rows := m.preflightRows()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Up):
		if m.migrationFlow.preflightCursor > 0 {
			m.migrationFlow.preflightCursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.migrationFlow.preflightCursor+1 < len(rows) {
			m.migrationFlow.preflightCursor++
		}
	case key.Matches(message, m.keys.First):
		m.migrationFlow.preflightCursor = 0
	case key.Matches(message, m.keys.Last):
		if len(rows) > 0 {
			m.migrationFlow.preflightCursor = len(rows) - 1
		}
	case key.Matches(message, m.keys.Expand):
		if m.hasPackagePreflight() {
			return m.continuePulledPackage()
		}
	case key.Matches(message, m.keys.Reload):
		return m.reloadMigrationPreflight()
	case key.Matches(message, m.keys.Publish):
		if !m.hasPackagePreflight() {
			return m.startMigrationPublish()
		}
	case key.Matches(message, m.keys.Version):
		if !m.hasPackagePreflight() {
			return m.startVersions()
		}
	case key.Matches(message, m.keys.Home):
		m.screen = screenHome
	case key.Matches(message, m.keys.Collapse):
		m.leaveMigrationPreflight()
		m.notice = ""
		m.noticeIsWarn = false
	}
	return m, nil
}

func (m *Model) leaveMigrationPreflight() {
	if m.hasPackagePreflight() {
		m.migrationFlow.mode = migrationModeVersions
		return
	}
	m.migrationFlow.mode = migrationModeStatus
}

func (m Model) continuePulledPackage() (tea.Model, tea.Cmd) {
	if m.migrationFlow.preflightStatus != statusReady ||
		!m.migrationFlow.preflightReport.Ok ||
		!m.hasPackagePreflight() {
		m.notice = m.text(msgMigrationPreflightBlocked)
		m.noticeIsWarn = true
		return m, nil
	}
	m.deployOptions.Package = m.migrationFlow.pulledReport.Package
	m.deployOptions.ExpectedSHA256 = m.migrationFlow.pulledReport.SHA256
	m.deployOptions.DryRun = false
	m.packageFromBuild = false
	m.screen = screenDeployment
	m.diffStatus = statusLoading
	m.deployErr = nil
	m.applyResult = nil
	m.diffCursor = 0
	m.notice = m.text(msgMigrationDiffReady)
	m.noticeIsWarn = false
	return m, diffCommand(m.deployOptions)
}
