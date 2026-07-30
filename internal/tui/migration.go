package tui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/migration"
)

type migrationMsg struct {
	report migration.Report
	err    error
}

type preflightMsg struct {
	report migration.PreflightReport
	err    error
}

func migrationCommand(options migration.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := migration.Inspect(options)
		return migrationMsg{report: report, err: err}
	}
}

func preflightCommand(options migration.PreflightOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := migration.InspectPreflight(options)
		return preflightMsg{report: report, err: err}
	}
}

func (m Model) handleMigrationMessage(message migrationMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.status = statusReady
	m.migrationFlow.report = message.report
	m.migrationFlow.err = message.err
	if message.err != nil {
		m.migrationFlow.status = statusFailed
		m.migrationFlow.afterMigration = migrationActionNone
		return m, nil
	}
	next := m.migrationFlow.afterMigration
	m.migrationFlow.afterMigration = migrationActionNone
	switch next {
	case migrationActionPublish:
		return m.startMigrationPublish()
	case migrationActionVersions:
		return m.startVersions()
	default:
		return m, nil
	}
}

func (m Model) handlePreflightMessage(message preflightMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.preflightStatus = statusReady
	m.migrationFlow.preflightReport = message.report
	m.migrationFlow.preflightErr = message.err
	m.migrationFlow.preflightCursor = 0
	if message.err != nil {
		m.migrationFlow.preflightStatus = statusFailed
	}
	return m, nil
}

func (m Model) migrationOptions() migration.Options {
	return migration.Options{
		WorkspaceRoot: m.workspace,
		Channel:       m.migrationFlow.channel,
		Home:          m.options.Home,
		Project:       m.options.Project,
	}
}

func (m Model) preflightOptions(profile string) migration.PreflightOptions {
	return migration.PreflightOptions{
		WorkspaceRoot: m.workspace,
		Profile:       profile,
		Home:          m.options.Home,
		Project:       m.options.Project,
	}
}

func (m Model) startMigration() (tea.Model, tea.Cmd) {
	if m.workspace == "" {
		return m.startWorkspaceInputFor(homeActionMigration)
	}
	m.screen = screenMigration
	m.migrationFlow.mode = migrationModeStatus
	m.migrationFlow.status = statusLoading
	m.migrationFlow.err = nil
	m.notice = ""
	m.noticeIsWarn = false
	return m, migrationCommand(m.migrationOptions())
}

func (m Model) startMigrationPreflight() (tea.Model, tea.Cmd) {
	if m.migrationFlow.status != statusReady {
		m.notice = "迁移状态尚未可用；先按 r 刷新"
		m.noticeIsWarn = true
		return m, nil
	}
	m.notice = "选择要迁移的资产组合"
	m.noticeIsWarn = false
	return m.startProfileInputFor(profileForPreflight)
}

func (m Model) reloadMigrationPreflight() (tea.Model, tea.Cmd) {
	profile := strings.TrimSpace(m.migrationFlow.preflightProfile)
	if profile == "" {
		return m.startMigrationPreflight()
	}
	m.migrationFlow.preflightStatus = statusLoading
	m.migrationFlow.preflightErr = nil
	m.migrationFlow.preflightCursor = 0
	m.notice = ""
	m.noticeIsWarn = false
	return m, preflightCommand(m.preflightOptions(profile))
}

func (m Model) startChannelInput() (tea.Model, tea.Cmd) {
	return m.startChannelInputFor(migrationActionNone)
}

func (m Model) startChannelInputFor(next migrationAction) (tea.Model, tea.Cmd) {
	m.migrationFlow.choosingChannel = true
	m.migrationFlow.afterChannel = next
	m.migrationFlow.channelInput.SetValue(m.migrationFlow.channel)
	m.notice = ""
	m.noticeIsWarn = false
	return m, m.migrationFlow.channelInput.Focus()
}

func (m Model) updateChannelInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.migrationFlow.choosingChannel = false
		m.migrationFlow.afterChannel = migrationActionNone
		m.migrationFlow.channelInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		candidate := expandHomePath(
			strings.TrimSpace(m.migrationFlow.channelInput.Value()),
			m.options.Home,
		)
		if candidate == "" {
			m.notice = "必须输入已有的分发通道目录；本页不会创建目录"
			m.noticeIsWarn = true
			return m, nil
		}
		m.migrationFlow.channel = candidate
		m.migrationFlow.choosingChannel = false
		m.migrationFlow.channelInput.Blur()
		m.migrationFlow.afterMigration = m.migrationFlow.afterChannel
		m.migrationFlow.afterChannel = migrationActionNone
		m.screen = screenMigration
		m.migrationFlow.mode = migrationModeStatus
		m.migrationFlow.status = statusLoading
		m.migrationFlow.err = nil
		m.notice = ""
		m.noticeIsWarn = false
		return m, migrationCommand(m.migrationOptions())
	}
	var command tea.Cmd
	m.migrationFlow.channelInput, command = m.migrationFlow.channelInput.Update(message)
	return m, command
}

func expandHomePath(candidate, home string) string {
	if candidate == "~" {
		return home
	}
	if strings.HasPrefix(candidate, "~/") {
		return filepath.Join(home, strings.TrimPrefix(candidate, "~/"))
	}
	return candidate
}
