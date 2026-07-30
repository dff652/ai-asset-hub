package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type workspaceMsg struct {
	root    string
	created bool
	err     error
}

type buildMsg struct {
	report      build.Report
	packagePath string
	purpose     profilePurpose
	err         error
}

type profilePurpose int

const (
	profileForDeployment profilePurpose = iota
	profileForPublish
	profileForPreflight
)

func prepareWorkspaceCommand(candidate, home, project string) tea.Cmd {
	return func() tea.Msg {
		root, created, err := workspace.PrepareRoot(candidate, home, project)
		return workspaceMsg{root: root, created: created, err: err}
	}
}

func buildCommand(options build.Options, purpose profilePurpose) tea.Cmd {
	return func() tea.Msg {
		report, err := build.Build(options)
		message := buildMsg{report: report, purpose: purpose, err: err}
		if err == nil && report.Ok && report.Package != nil {
			message.packagePath = filepath.Join(options.OutDir, report.Package.Archive)
		}
		return message
	}
}

func (m Model) startWorkspaceInput() (tea.Model, tea.Cmd) {
	return m.startWorkspaceInputFor(homeActionNone)
}

func (m Model) startWorkspaceInputFor(next homeAction) (tea.Model, tea.Cmd) {
	m.choosingWorkspace = true
	m.afterWorkspace = next
	m.workspaceInput.SetValue("")
	m.notice = ""
	m.noticeIsWarn = false
	return m, m.workspaceInput.Focus()
}

func (m Model) updateWorkspaceInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.choosingWorkspace = false
		m.workspaceInput.SetValue("")
		m.workspaceInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		candidate := strings.TrimSpace(m.workspaceInput.Value())
		if candidate == "" {
			m.notice = "必须输入资产库路径"
			m.noticeIsWarn = true
			return m, nil
		}
		m.choosingWorkspace = false
		m.preparingWorkspace = true
		m.workspaceInput.Blur()
		m.notice = "正在打开资产库…"
		m.noticeIsWarn = false
		return m, prepareWorkspaceCommand(candidate, m.options.Home, m.options.Project)
	}
	var command tea.Cmd
	m.workspaceInput, command = m.workspaceInput.Update(message)
	return m, command
}

func (m Model) startProfileInput() (tea.Model, tea.Cmd) {
	return m.startProfileInputFor(profileForDeployment)
}

func (m Model) startProfileInputFor(purpose profilePurpose) (tea.Model, tea.Cmd) {
	if m.composing {
		m.notice = "正在加入资产库，请完成后再继续"
		m.noticeIsWarn = true
		return m, nil
	}
	if m.workspace == "" {
		m.notice = "先按 w 明确选择资产库"
		m.noticeIsWarn = true
		return m, nil
	}
	manifest := filepath.Join(m.workspace, "manifest.yaml")
	document, _, err := workspace.LoadManifest(manifest)
	if err != nil {
		m.notice = "无法读取 manifest.yaml：" + err.Error()
		m.noticeIsWarn = true
		return m, nil
	}
	profile := "personal"
	m.availableProfiles = make([]string, 0, len(document.Profiles))
	for name := range document.Profiles {
		m.availableProfiles = append(m.availableProfiles, name)
	}
	sort.Strings(m.availableProfiles)
	if _, ok := document.Profiles[profile]; !ok && len(m.availableProfiles) > 0 {
		profile = m.availableProfiles[0]
	}
	m.choosingProfile = true
	m.profilePurpose = purpose
	m.profileInput.SetValue(profile)
	m.notice = ""
	m.noticeIsWarn = false
	return m, m.profileInput.Focus()
}

func (m Model) updateProfileInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.choosingProfile = false
		m.profilePurpose = profileForDeployment
		m.profileInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		profile := strings.TrimSpace(m.profileInput.Value())
		if profile == "" {
			m.notice = "必须输入资产组合名称"
			m.noticeIsWarn = true
			return m, nil
		}
		m.choosingProfile = false
		m.profileInput.Blur()
		purpose := m.profilePurpose
		m.profilePurpose = profileForDeployment
		if purpose == profileForPreflight {
			m.screen = screenMigration
			m.migrationFlow.mode = migrationModePreflight
			m.migrationFlow.preflightStatus = statusLoading
			m.migrationFlow.preflightErr = nil
			m.migrationFlow.preflightProfile = profile
			m.migrationFlow.preflightCursor = 0
			m.notice = ""
			m.noticeIsWarn = false
			return m, preflightCommand(m.preflightOptions(profile))
		}
		m.invalidateBuiltPackage()
		m.building = true
		if purpose == profileForPublish {
			m.notice = "正在检查资产并准备待发布安装包…"
		} else {
			m.notice = "正在检查资产并准备安装包…"
		}
		m.noticeIsWarn = false
		return m, buildCommand(build.Options{
			Manifest: filepath.Join(m.workspace, "manifest.yaml"),
			Root:     m.workspace,
			Profile:  profile,
			OutDir:   filepath.Join(m.workspace, "dist"),
		}, purpose)
	}
	var command tea.Cmd
	m.profileInput, command = m.profileInput.Update(message)
	return m, command
}

func buildFailureNotice(report build.Report) string {
	if len(report.Findings) > 0 {
		finding := report.Findings[0]
		return fmt.Sprintf("资产检查未通过：%s — %s", finding.Code, finding.Message)
	}
	return "资产检查未通过；未生成安装包"
}
