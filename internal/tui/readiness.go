package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/readiness"
)

type readinessEvidenceKind int

const (
	readinessEvidenceNone readinessEvidenceKind = iota
	readinessEvidenceBackup
	readinessEvidenceRestore
)

type readinessMsg struct {
	report readiness.Report
	err    error
}

type readinessFlow struct {
	status              status
	report              readiness.Report
	err                 error
	profile             string
	backupEvidencePath  string
	restoreExercisePath string
	choosingEvidence    readinessEvidenceKind
	evidenceInput       textinput.Model
}

func newReadinessFlow() readinessFlow {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 256
	input.Width = 48
	return readinessFlow{
		status:        statusReady,
		evidenceInput: input,
	}
}

func readinessCommand(options readiness.Options) tea.Cmd {
	return func() tea.Msg {
		report, err := readiness.Inspect(options)
		return readinessMsg{report: report, err: err}
	}
}

func (m Model) readinessOptions() readiness.Options {
	return readiness.Options{
		WorkspaceRoot:       m.workspace,
		Profile:             m.readinessFlow.profile,
		Home:                m.options.Home,
		Project:             m.options.Project,
		BackupEvidencePath:  m.readinessFlow.backupEvidencePath,
		RestoreExercisePath: m.readinessFlow.restoreExercisePath,
	}
}

func (m Model) startReadiness() (tea.Model, tea.Cmd) {
	if m.workspace == "" {
		return m.startWorkspaceInputFor(homeActionReadiness)
	}
	return m.startProfileInputFor(profileForReadiness)
}

func (m Model) beginReadiness(profile string) (tea.Model, tea.Cmd) {
	m.screen = screenReadiness
	m.readinessFlow.profile = strings.TrimSpace(profile)
	m.readinessFlow.status = statusLoading
	m.readinessFlow.err = nil
	m.readinessFlow.choosingEvidence = readinessEvidenceNone
	m.readinessFlow.evidenceInput.Blur()
	m.notice = ""
	m.noticeIsWarn = false
	m.showHelp = false
	return m, readinessCommand(m.readinessOptions())
}

func (m Model) handleReadinessMessage(message readinessMsg) (tea.Model, tea.Cmd) {
	m.readinessFlow.err = message.err
	if message.err != nil {
		m.readinessFlow.status = statusFailed
		return m, nil
	}
	m.readinessFlow.report = message.report
	m.readinessFlow.status = statusReady
	return m, nil
}

func (m Model) updateReadinessKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Reload):
		if m.readinessFlow.profile == "" {
			return m, nil
		}
		m.readinessFlow.status = statusLoading
		m.readinessFlow.err = nil
		m.notice = ""
		m.noticeIsWarn = false
		return m, readinessCommand(m.readinessOptions())
	case key.Matches(message, m.keys.Build):
		return m.startReadinessEvidenceInput(readinessEvidenceBackup)
	case key.Matches(message, m.keys.Rollback):
		return m.startReadinessEvidenceInput(readinessEvidenceRestore)
	case key.Matches(message, m.keys.Collapse):
		m.screen = screenHome
		m.notice = ""
		m.noticeIsWarn = false
		return m, nil
	}
	return m, nil
}

func (m Model) startReadinessEvidenceInput(kind readinessEvidenceKind) (tea.Model, tea.Cmd) {
	if m.readinessFlow.status == statusLoading {
		return m, nil
	}
	current := m.readinessFlow.backupEvidencePath
	if kind == readinessEvidenceRestore {
		current = m.readinessFlow.restoreExercisePath
	}
	m.readinessFlow.choosingEvidence = kind
	m.readinessFlow.evidenceInput.SetValue(current)
	m.readinessFlow.evidenceInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updateReadinessEvidenceInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.readinessFlow.choosingEvidence = readinessEvidenceNone
		m.readinessFlow.evidenceInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		value := strings.TrimSpace(m.readinessFlow.evidenceInput.Value())
		switch m.readinessFlow.choosingEvidence {
		case readinessEvidenceBackup:
			m.readinessFlow.backupEvidencePath = value
		case readinessEvidenceRestore:
			m.readinessFlow.restoreExercisePath = value
		}
		m.readinessFlow.choosingEvidence = readinessEvidenceNone
		m.readinessFlow.evidenceInput.Blur()
		if m.readinessFlow.profile == "" {
			return m, nil
		}
		m.readinessFlow.status = statusLoading
		m.readinessFlow.err = nil
		m.notice = ""
		m.noticeIsWarn = false
		return m, readinessCommand(m.readinessOptions())
	}
	var command tea.Cmd
	m.readinessFlow.evidenceInput, command = m.readinessFlow.evidenceInput.Update(message)
	return m, command
}
