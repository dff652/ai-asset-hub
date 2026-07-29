package tui

import (
	"fmt"
	"strings"

	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/version"
)

func (m Model) versionView(style styles) string {
	header := joinEdges(
		style.header.Render("aiah · version"),
		version.Version,
		max(20, m.width),
	)
	lines := []string{
		"",
		style.header.Render("aiah binary"),
		"version     " + displayValue(version.Version),
		"commit      " + displayValue(shortBuildCommit(version.Commit)),
		"built       " + displayValue(version.Date),
		"",
		style.header.Render("current asset deployment"),
	}
	lines = append(lines, m.deploymentVersionLines()...)
	lines = append(lines,
		"",
		style.header.Render("release check"),
	)
	lines = append(lines, m.releaseCheckLines(style)...)
	lines = append(lines,
		"",
		style.muted.Render("c 检查更新（仅按键后联网） · h doctor · Esc inventory · ? 帮助 · q 退出"),
	)
	return header + "\n" + strings.Join(lines, "\n")
}

func (m Model) deploymentVersionLines() []string {
	switch {
	case m.doctorStatus == statusLoading:
		return []string{"status      checking local deployment…"}
	case m.doctorStatus == statusFailed:
		return []string{"status      unavailable"}
	case m.doctorReport.Deployment == nil:
		return []string{"status      no current deployment"}
	default:
		deployment := m.doctorReport.Deployment
		return []string{
			"package     " + displayValue(deployment.Package),
			"version     " + displayValue(deployment.Version),
			"profile     " + displayValue(deployment.Profile),
			"backupId    " + displayValue(deployment.BackupID),
		}
	}
}

func (m Model) releaseCheckLines(style styles) []string {
	switch {
	case m.updateChecking:
		return []string{"status      checking GitHub Releases…"}
	case m.updateChecked && m.updateErr != nil:
		return []string{style.error.Render("status      failed: " + m.updateErr.Error())}
	case !m.updateChecked:
		return []string{
			"status      not checked",
			"hint        press c to check; opening TUI never checks automatically",
		}
	}

	report := m.updateReport
	lines := []string{
		"status      " + updateStatusLabel(report.Status),
		"latest      " + displayValue(report.LatestVersion),
		"release     " + displayValue(report.ReleaseURL),
	}
	if report.UpdateAvailable {
		lines = append(lines,
			style.warning.Render("upgrade"),
		)
		lines = append(lines, displayUpgradeCommand(report.UpgradeCommand)...)
	}
	return lines
}

func displayUpgradeCommand(command string) []string {
	const (
		prefix  = "curl -fsSL "
		urlBase = "https://raw.githubusercontent.com/dff652/"
		suffix  = " | sh"
	)
	script := strings.TrimPrefix(command, prefix+urlBase)
	script = strings.TrimSuffix(script, suffix)
	if script == command || script == "" ||
		prefix+urlBase+script+suffix != command {
		return []string{command}
	}
	return []string{
		"curl -fsSL \\",
		"  '" + urlBase + "'\\",
		"  '" + script + "' | sh",
	}
}

func updateStatusLabel(status string) string {
	switch status {
	case updater.StatusCurrent:
		return "current"
	case updater.StatusUpdateAvailable:
		return "update available"
	case updater.StatusAhead:
		return "ahead of latest release"
	case updater.StatusDevelopment:
		return "development build; comparison unavailable"
	default:
		return fmt.Sprintf("unknown (%s)", status)
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
