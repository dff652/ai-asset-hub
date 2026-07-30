package tui

import (
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
			m.notice = m.text(msgWorkspacePathRequired)
			m.noticeIsWarn = true
			return m, nil
		}
		m.choosingWorkspace = false
		m.preparingWorkspace = true
		m.workspaceInput.Blur()
		m.notice = m.text(msgWorkspaceOpening)
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
		m.notice = m.text(msgProfileComposeBusy)
		m.noticeIsWarn = true
		return m, nil
	}
	if m.workspace == "" {
		m.notice = m.text(msgWorkspaceSelectFirst)
		m.noticeIsWarn = true
		return m, nil
	}
	manifest := filepath.Join(m.workspace, "manifest.yaml")
	document, _, err := workspace.LoadManifest(manifest)
	if err != nil {
		m.notice = m.text(msgProfileManifestReadFailed, err)
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
			m.notice = m.text(msgProfileRequired)
			m.noticeIsWarn = true
			return m, nil
		}
		m.choosingProfile = false
		m.profileInput.Blur()
		purpose := m.profilePurpose
		m.profilePurpose = profileForDeployment
		if purpose == profileForPreflight {
			return m, m.beginWorkspacePreflight(profile)
		}
		m.invalidateBuiltPackage()
		m.building = true
		if purpose == profileForPublish {
			m.notice = m.text(msgProfileBuildPublishing)
		} else {
			m.notice = m.text(msgProfileBuildPreparing)
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

func (m Model) buildFailureNotice(report build.Report) string {
	if len(report.Findings) > 0 {
		finding := report.Findings[0]
		return m.text(msgProfileBuildFailedFinding, finding.Code, finding.Message)
	}
	return m.text(msgProfileBuildFailedNoPackage)
}
