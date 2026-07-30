package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/apply"
)

type doctorMsg struct {
	report apply.DoctorReport
	err    error
}

type rollbackMsg struct {
	report apply.RollbackReport
	err    error
}

func doctorCommand(options apply.DoctorOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := apply.Doctor(options)
		return doctorMsg{report: report, err: err}
	}
}

func rollbackCoreCommand(options apply.RollbackOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := apply.Rollback(options)
		return rollbackMsg{report: report, err: err}
	}
}

func (m Model) doctorOptions() apply.DoctorOptions {
	return apply.DoctorOptions{Home: m.options.Home, Project: m.options.Project}
}

func (m Model) startDoctor() (tea.Model, tea.Cmd) {
	if !m.maintenance {
		m.notice = m.text(msgHealthUnavailable)
		m.noticeIsWarn = true
		return m, nil
	}
	m.screen = screenHealth
	m.doctorStatus = statusLoading
	m.doctorErr = nil
	m.rollbackResult = nil
	m.rollbackErr = nil
	m.healthCursor = 0
	m.notice = ""
	m.noticeIsWarn = false
	return m, doctorCommand(m.doctorOptions())
}

func (m Model) updateHealthKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.healthRows()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Doctor):
		return m.startDoctor()
	case key.Matches(message, m.keys.Version):
		return m.startVersion()
	case key.Matches(message, m.keys.Rollback):
		return m.startRollbackConfirmation()
	case key.Matches(message, m.keys.Up):
		if m.healthCursor > 0 {
			m.healthCursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.healthCursor+1 < len(rows) {
			m.healthCursor++
		}
	case key.Matches(message, m.keys.First):
		m.healthCursor = 0
	case key.Matches(message, m.keys.Last):
		if len(rows) > 0 {
			m.healthCursor = len(rows) - 1
		}
	case key.Matches(message, m.keys.Collapse):
		m.screen = screenInventory
		m.notice = ""
		m.noticeIsWarn = false
	}
	return m, nil
}

func (m Model) startRollbackConfirmation() (tea.Model, tea.Cmd) {
	switch {
	case m.doctorStatus != statusReady || m.doctorErr != nil:
		m.notice = m.text(msgHealthRollbackNotReady)
		m.noticeIsWarn = true
		return m, nil
	case !m.doctorReport.Ok:
		m.notice = m.text(msgHealthRollbackBlocked)
		m.noticeIsWarn = true
		return m, nil
	case m.doctorReport.Deployment == nil ||
		m.doctorReport.Deployment.BackupID == "":
		m.notice = m.text(msgHealthRollbackNoCurrent)
		m.noticeIsWarn = true
		return m, nil
	}
	m.rollbackConfirming = true
	m.rollbackInput.SetValue("")
	m.rollbackInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updateRollbackConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(message, m.keys.Collapse) {
		m.rollbackConfirming = false
		m.rollbackInput.SetValue("")
		m.rollbackInput.Blur()
		m.notice = m.text(msgHealthRollbackCancelled)
		m.noticeIsWarn = false
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		if m.rollbackInput.Value() != "rollback" {
			m.notice = m.text(msgHealthRollbackMismatch)
			m.noticeIsWarn = true
			m.rollbackInput.SetValue("")
			return m, nil
		}
		backupID := m.doctorReport.Deployment.BackupID
		m.rollbackConfirming = false
		m.rollbackInput.Blur()
		m.rollbacking = true
		m.notice = ""
		m.noticeIsWarn = false
		return m, rollbackCoreCommand(apply.RollbackOptions{
			Home: m.options.Home, Project: m.options.Project, BackupID: backupID,
		})
	}
	var command tea.Cmd
	m.rollbackInput, command = m.rollbackInput.Update(message)
	return m, command
}

func (m *Model) clampHealthCursor() {
	rows := m.healthRows()
	if len(rows) == 0 {
		m.healthCursor = 0
		return
	}
	if m.healthCursor >= len(rows) {
		m.healthCursor = len(rows) - 1
	}
	if m.healthCursor < 0 {
		m.healthCursor = 0
	}
}
