package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/migration"
)

type migrationMsg struct {
	report migration.Report
	err    error
}

func migrationCommand(options migration.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := migration.Inspect(options)
		return migrationMsg{report: report, err: err}
	}
}

func (m Model) migrationOptions() migration.Options {
	return migration.Options{
		WorkspaceRoot: m.workspace,
		Channel:       m.migrationChannel,
		Home:          m.options.Home,
		Project:       m.options.Project,
	}
}

func (m Model) startMigration() (tea.Model, tea.Cmd) {
	if m.workspace == "" {
		return m.startWorkspaceInputFor(homeActionMigration)
	}
	m.screen = screenMigration
	m.migrationStatus = statusLoading
	m.migrationErr = nil
	m.notice = ""
	m.noticeIsWarn = false
	return m, migrationCommand(m.migrationOptions())
}

func (m Model) startChannelInput() (tea.Model, tea.Cmd) {
	m.choosingChannel = true
	m.channelInput.SetValue(m.migrationChannel)
	m.notice = ""
	m.noticeIsWarn = false
	return m, m.channelInput.Focus()
}

func (m Model) updateChannelInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.choosingChannel = false
		m.channelInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		candidate := strings.TrimSpace(m.channelInput.Value())
		if candidate == "" {
			m.notice = "必须输入已有的分发通道目录；本页不会创建目录"
			m.noticeIsWarn = true
			return m, nil
		}
		if candidate == "~" {
			candidate = m.options.Home
		} else if strings.HasPrefix(candidate, "~/") {
			candidate = filepath.Join(m.options.Home, strings.TrimPrefix(candidate, "~/"))
		}
		m.migrationChannel = candidate
		m.choosingChannel = false
		m.channelInput.Blur()
		return m.startMigration()
	}
	var command tea.Cmd
	m.channelInput, command = m.channelInput.Update(message)
	return m, command
}

func (m Model) updateMigrationKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.CheckUpdate):
		return m.startChannelInput()
	case key.Matches(message, m.keys.Reload):
		return m.startMigration()
	case key.Matches(message, m.keys.Collapse):
		m.screen = screenHome
	}
	return m, nil
}
