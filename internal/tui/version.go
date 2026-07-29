package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	updater "github.com/dff652/ai-asset-hub/internal/update"
)

type updateCheckMsg struct {
	report updater.Report
	err    error
}

func updateCheckCommand(options updater.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := updater.Check(options)
		return updateCheckMsg{report: report, err: err}
	}
}

func (m Model) startVersion() (tea.Model, tea.Cmd) {
	if !m.maintenance {
		m.notice = "当前交互只提供部署审阅；请用 aiah ui 查看版本信息"
		m.noticeIsWarn = true
		return m, nil
	}
	m.screen = screenVersion
	m.notice = ""
	m.noticeIsWarn = false
	if m.doctorStatus == statusReady {
		return m, nil
	}
	m.doctorStatus = statusLoading
	m.doctorErr = nil
	return m, doctorCommand(m.doctorOptions())
}

func (m Model) startUpdateCheck() (tea.Model, tea.Cmd) {
	if m.updateChecking {
		return m, nil
	}
	m.updateChecking = true
	m.updateChecked = false
	m.updateErr = nil
	return m, updateCheckCommand(updater.Options{})
}

func (m Model) updateVersionKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.CheckUpdate):
		return m.startUpdateCheck()
	case key.Matches(message, m.keys.Doctor):
		return m.startDoctor()
	case key.Matches(message, m.keys.Collapse):
		m.screen = screenInventory
		m.notice = ""
		m.noticeIsWarn = false
	}
	return m, nil
}
