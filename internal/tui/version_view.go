package tui

import (
	"strings"

	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/version"
)

func (m Model) versionView(style styles) string {
	header := joinEdges(
		style.header.Render(m.text(msgVersionTitle)),
		version.Version,
		max(20, m.width),
	)
	lines := []string{
		"",
		style.header.Render(m.text(msgVersionCurrentProgram)),
		m.text(msgVersionProgramVersion, displayValue(version.Version)),
		m.text(msgVersionCommit, displayValue(shortBuildCommit(version.Commit))),
		m.text(msgVersionBuildDate, displayValue(version.Date)),
		"",
		style.header.Render(m.text(msgVersionCurrentInstall)),
	}
	lines = append(lines, m.deploymentVersionLines()...)
	lines = append(lines,
		"",
		style.header.Render(m.text(msgVersionUpdateTitle)),
	)
	lines = append(lines, m.releaseCheckLines(style)...)
	lines = append(lines,
		"",
		style.muted.Render(m.text(msgVersionFooter)),
	)
	return header + "\n" + strings.Join(lines, "\n")
}

func (m Model) deploymentVersionLines() []string {
	switch {
	case m.doctorStatus == statusLoading:
		return []string{m.text(msgVersionInstallChecking)}
	case m.doctorStatus == statusFailed:
		return []string{m.text(msgVersionInstallUnreadable)}
	case m.doctorReport.Deployment == nil:
		return []string{m.text(msgVersionInstallNone)}
	default:
		deployment := m.doctorReport.Deployment
		return []string{
			m.text(msgVersionPackage, displayValue(deployment.Package)),
			m.text(msgVersionAssetVersion, displayValue(deployment.Version)),
			m.text(msgVersionProfile, displayValue(deployment.Profile)),
			m.text(msgVersionBackup, displayValue(deployment.BackupID)),
		}
	}
}

func (m Model) releaseCheckLines(style styles) []string {
	switch {
	case m.updateChecking:
		return []string{m.text(msgVersionReleaseChecking)}
	case m.updateChecked && m.updateErr != nil:
		return []string{
			style.error.Render(m.text(msgVersionReleaseFailed, m.updateErr.Error())),
		}
	case !m.updateChecked:
		return []string{
			m.text(msgVersionReleaseUnchecked),
			m.text(msgVersionReleaseHint),
		}
	}

	report := m.updateReport
	lines := []string{
		m.text(msgVersionReleaseStatus, m.updateStatusLabel(report.Status)),
		m.text(msgVersionLatest, displayValue(report.LatestVersion)),
		m.text(msgVersionReleasePage, displayValue(report.ReleaseURL)),
	}
	if report.UpdateAvailable {
		lines = append(lines,
			style.warning.Render(m.text(msgVersionUpgradeCommand)),
		)
		lines = append(lines, displayUpgradeCommand(report.UpgradeCommand)...)
	}
	return lines
}

func displayUpgradeCommand(command string) []string {
	const (
		prefix     = "curl -fsSL "
		urlBase    = "https://raw.githubusercontent.com/dff652/"
		pipeMarker = " | AIAH_VERSION="
		suffix     = " sh"
	)
	commandBody := strings.TrimPrefix(command, prefix+urlBase)
	parts := strings.Split(commandBody, pipeMarker)
	if commandBody == command || len(parts) != 2 {
		return []string{command}
	}
	script := parts[0]
	release := strings.TrimSuffix(parts[1], suffix)
	if script == "" || release == parts[1] || release == "" ||
		prefix+urlBase+script+pipeMarker+release+suffix != command {
		return []string{command}
	}
	return []string{
		"curl -fsSL \\",
		"  '" + urlBase + "'\\",
		"  '" + script + "' |",
		"  AIAH_VERSION=" + release + " sh",
	}
}

func (m Model) updateStatusLabel(status string) string {
	switch status {
	case updater.StatusCurrent:
		return m.text(msgVersionStatusCurrent)
	case updater.StatusUpdateAvailable:
		return m.text(msgVersionStatusAvailable)
	case updater.StatusAhead:
		return m.text(msgVersionStatusAhead)
	case updater.StatusDevelopment:
		return m.text(msgVersionStatusDevelopment)
	default:
		return m.text(msgVersionStatusUnknown, status)
	}
}

func shortBuildCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func displayValue(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
