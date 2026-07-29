package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/apply"
)

type diffMsg struct {
	report apply.Report
	err    error
}

type applyMsg struct {
	report apply.Report
	err    error
}

func diffCommand(options apply.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := apply.Diff(options)
		return diffMsg{report: report, err: err}
	}
}

func applyCommand(options apply.Options) tea.Cmd {
	options.DryRun = false
	return func() tea.Msg {
		report, err := apply.Apply(options)
		return applyMsg{report: report, err: err}
	}
}

func (m Model) startDiff() (tea.Model, tea.Cmd) {
	if m.deployOptions.Package == "" {
		m.notice = "未指定部署包；用 aiah ui --package PATH 启动才能审阅 diff"
		m.noticeIsWarn = true
		return m, nil
	}
	if m.diffStatus == statusLoading {
		return m, nil
	}
	m.screen = screenDeployment
	m.diffStatus = statusLoading
	m.deployErr = nil
	m.applyResult = nil
	m.diffCursor = 0
	m.notice = ""
	m.noticeIsWarn = false
	return m, diffCommand(m.deployOptions)
}

func (m Model) updateDeploymentKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.deploymentRows()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Diff):
		return m.startDiff()
	case key.Matches(message, m.keys.Doctor):
		return m.startDoctor()
	case key.Matches(message, m.keys.Version):
		return m.startVersion()
	case key.Matches(message, m.keys.Apply):
		return m.startApplyConfirmation()
	case key.Matches(message, m.keys.Up):
		if m.diffCursor > 0 {
			m.diffCursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.diffCursor+1 < len(rows) {
			m.diffCursor++
		}
	case key.Matches(message, m.keys.First):
		m.diffCursor = 0
	case key.Matches(message, m.keys.Last):
		if len(rows) > 0 {
			m.diffCursor = len(rows) - 1
		}
	case key.Matches(message, m.keys.Expand):
		m.setCurrentDiffExpanded(rows, true)
	case key.Matches(message, m.keys.Collapse):
		if message.String() == "esc" {
			m.screen = screenInventory
			m.notice = ""
			return m, nil
		}
		m.setCurrentDiffExpanded(rows, false)
	}
	return m, nil
}

func (m Model) startApplyConfirmation() (tea.Model, tea.Cmd) {
	if m.diffStatus != statusReady || !m.diffReport.Ok {
		m.notice = "diff 未通过，不能执行 apply；请先查看原始 findings"
		m.noticeIsWarn = true
		return m, nil
	}
	if m.applyResult != nil {
		m.notice = "本次执行已经完成；按 d 重新计算 diff"
		m.noticeIsWarn = true
		return m, nil
	}
	m.confirming = true
	m.confirmInput.SetValue("")
	m.confirmInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updateConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(message, m.keys.Collapse) {
		m.confirming = false
		m.confirmInput.SetValue("")
		m.confirmInput.Blur()
		m.notice = "已取消 apply"
		m.noticeIsWarn = false
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		if m.confirmInput.Value() != "apply" {
			m.notice = "确认文本不匹配；必须完整输入 apply"
			m.noticeIsWarn = true
			m.confirmInput.SetValue("")
			return m, nil
		}
		m.confirming = false
		m.confirmInput.Blur()
		m.applying = true
		m.notice = ""
		m.noticeIsWarn = false
		return m, applyCommand(m.deployOptions)
	}
	var command tea.Cmd
	m.confirmInput, command = m.confirmInput.Update(message)
	return m, command
}

func (m *Model) clampDiffCursor() {
	rows := m.deploymentRows()
	if len(rows) == 0 {
		m.diffCursor = 0
		return
	}
	if m.diffCursor >= len(rows) {
		m.diffCursor = len(rows) - 1
	}
	if m.diffCursor < 0 {
		m.diffCursor = 0
	}
}

func (m *Model) setCurrentDiffExpanded(rows []deploymentRow, expanded bool) {
	if m.diffCursor < 0 || m.diffCursor >= len(rows) || rows[m.diffCursor].kind != deploymentGroupRow {
		return
	}
	m.diffExpanded[rows[m.diffCursor].key] = expanded
	m.clampDiffCursor()
}

func rollbackCommand(options apply.Options, backupID string) string {
	return apply.RollbackCommand(apply.RollbackOptions{
		Home: options.Home, Project: options.Project, BackupID: backupID,
	})
}
