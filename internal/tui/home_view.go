package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type homeAction int

const (
	homeActionNone homeAction = iota
	homeActionOrganize
	homeActionApply
	homeActionHealth
	homeActionMigration
	homeActionVersion
)

type homeItem struct {
	action      homeAction
	title       string
	description string
}

func (m Model) homeItems() []homeItem {
	return []homeItem{
		{
			action:      homeActionOrganize,
			title:       m.text(msgHomeOrganizeTitle),
			description: m.text(msgHomeOrganizeDesc),
		},
		{
			action:      homeActionApply,
			title:       m.text(msgHomeApplyTitle),
			description: m.text(msgHomeApplyDesc),
		},
		{
			action:      homeActionHealth,
			title:       m.text(msgHomeHealthTitle),
			description: m.text(msgHomeHealthDesc),
		},
		{
			action:      homeActionMigration,
			title:       m.text(msgHomeMigrationTitle),
			description: m.text(msgHomeMigrationDesc),
		},
		{
			action:      homeActionVersion,
			title:       m.text(msgHomeVersionTitle),
			description: m.text(msgHomeVersionDesc),
		},
	}
}

func (m Model) homeView(style styles) string {
	header := joinEdges(
		style.header.Render(m.text(msgHomeAppTitle)),
		"aiah",
		max(20, m.width),
	)
	lines := []string{
		header,
		style.muted.Render(m.text(msgHomeSummary)),
		"",
		padRight(m.text(msgHomeLibraryLabel), 15) + " " + m.homeWorkspaceStatus(),
		padRight(m.text(msgHomeAssetStatusLabel), 15) + " " + m.homeInventoryStatus(),
		padRight(m.text(msgHomeInstallLabel), 15) + " " + m.homeDeploymentStatus(),
		"",
		style.header.Render(m.text(msgHomeTaskPrompt)),
	}

	for index, item := range m.homeItems() {
		prefix := "  "
		if index == m.homeCursor {
			prefix = "> "
		}
		const titleWidth = 28
		title := padRight(truncate(item.title, titleWidth), titleWidth)
		line := truncate(prefix+title+" "+item.description, max(20, m.width))
		if index == m.homeCursor {
			line = style.selected.Render(line)
		}
		lines = append(lines, line)
	}

	lines = append(lines,
		"",
		style.muted.Render(m.text(msgHomeFooter)),
	)
	return strings.Join(lines, "\n")
}

func (m Model) homeWorkspaceStatus() string {
	if m.workspace == "" {
		return m.text(msgHomeWorkspaceUnset)
	}
	return m.workspace
}

func (m Model) homeInventoryStatus() string {
	switch m.status {
	case statusLoading:
		return m.text(msgHomeScanLoading)
	case statusFailed:
		return m.text(msgHomeScanFailed)
	default:
		if m.workspace != "" {
			if m.catalogErr != nil {
				return m.text(
					msgHomeCatalogUnavailable,
					len(m.report.Findings),
				)
			}
			summary := m.catalog.Summary
			return m.text(
				msgHomeCatalogSummary,
				summary.Unmanaged,
				summary.Managed,
				summary.SourceChanged,
				summary.LibraryOnly,
				summary.Blocked,
				len(m.report.Findings),
			)
		}
		return m.text(
			msgHomeDiscovered,
			m.report.Summary.CandidateAssets,
			len(m.report.Findings),
		)
	}
}

func (m Model) homeDeploymentStatus() string {
	if m.doctorStatus == statusReady {
		if m.doctorReport.Deployment == nil {
			return m.text(msgHomeNoManagedInstall)
		}
		deployment := m.doctorReport.Deployment
		packageVersion := strings.TrimSpace(strings.Join([]string{
			deployment.Package,
			deployment.Version,
		}, " "))
		parts := make([]string, 0, 2)
		if packageVersion != "" {
			parts = append(parts, packageVersion)
		}
		if len(deployment.Targets) > 0 {
			parts = append(parts, strings.Join(deployment.Targets, ","))
		}
		identity := strings.Join(parts, " · ")
		if identity == "" {
			identity = m.text(msgHomeManagedInstall)
		}
		if m.doctorReport.Ok {
			return identity + " · " + m.text(msgHomeInstallHealthy)
		}
		return m.text(msgHomeInstallRisk, identity, len(m.doctorReport.Findings))
	}
	if m.doctorErr != nil {
		return m.text(msgHomeDoctorFailed)
	}
	return m.text(msgHomeDoctorChecking)
}

func (m Model) updateHomeKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.homeItems()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Up):
		if m.homeCursor > 0 {
			m.homeCursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.homeCursor+1 < len(items) {
			m.homeCursor++
		}
	case key.Matches(message, m.keys.First):
		m.homeCursor = 0
	case key.Matches(message, m.keys.Last):
		m.homeCursor = len(items) - 1
	case key.Matches(message, m.keys.Expand):
		return m.startHomeAction(items[m.homeCursor].action)
	}
	return m, nil
}

func (m Model) startHomeAction(action homeAction) (tea.Model, tea.Cmd) {
	switch action {
	case homeActionOrganize:
		if m.workspace == "" {
			return m.startWorkspaceInputFor(homeActionOrganize)
		}
		m.screen = screenInventory
		m.notice = m.text(msgHomeOrganizeNotice)
		m.noticeIsWarn = false
		return m, nil
	case homeActionApply:
		if m.workspace == "" {
			return m.startWorkspaceInputFor(homeActionApply)
		}
		return m.startProfileInput()
	case homeActionHealth:
		return m.startDoctor()
	case homeActionMigration:
		if m.workspace == "" {
			return m.startWorkspaceInputFor(homeActionMigration)
		}
		return m.startMigration()
	case homeActionVersion:
		return m.startVersion()
	default:
		return m, nil
	}
}

func (m Model) homeHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render(m.text(msgHomeHelpTitle)),
		"",
		m.text(msgHomeHelpIntroFirst),
		m.text(msgHomeHelpIntroSecond),
		"",
		m.text(msgHomeHelpOrganize),
		m.text(msgHomeHelpApply),
		m.text(msgHomeHelpHealth),
		m.text(msgHomeHelpMigration),
		m.text(msgHomeHelpVersion),
		"",
		m.text(msgHomeHelpLibrary),
		m.text(msgHomeHelpFooter),
	}, "\n")
}
