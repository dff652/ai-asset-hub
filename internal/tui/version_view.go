package tui

import (
	"fmt"
	"strings"

	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/version"
)

func (m Model) versionView(style styles) string {
	header := joinEdges(
		style.header.Render("aiah · 关于与更新"),
		version.Version,
		max(20, m.width),
	)
	lines := []string{
		"",
		style.header.Render("当前程序"),
		"版本        " + displayValue(version.Version),
		"提交        " + displayValue(shortBuildCommit(version.Commit)),
		"构建时间    " + displayValue(version.Date),
		"",
		style.header.Render("当前资产安装"),
	}
	lines = append(lines, m.deploymentVersionLines()...)
	lines = append(lines,
		"",
		style.header.Render("版本更新"),
	)
	lines = append(lines, m.releaseCheckLines(style)...)
	lines = append(lines,
		"",
		style.muted.Render("c 检查更新（仅按键后联网） · h 安装检查 · m 首页 · ? 帮助 · q 退出"),
	)
	return header + "\n" + strings.Join(lines, "\n")
}

func (m Model) deploymentVersionLines() []string {
	switch {
	case m.doctorStatus == statusLoading:
		return []string{"状态        正在检查本机安装…"}
	case m.doctorStatus == statusFailed:
		return []string{"状态        无法读取"}
	case m.doctorReport.Deployment == nil:
		return []string{"状态        尚无当前安装"}
	default:
		deployment := m.doctorReport.Deployment
		return []string{
			"资产包      " + displayValue(deployment.Package),
			"版本        " + displayValue(deployment.Version),
			"资产组合    " + displayValue(deployment.Profile),
			"备份 ID     " + displayValue(deployment.BackupID),
		}
	}
}

func (m Model) releaseCheckLines(style styles) []string {
	switch {
	case m.updateChecking:
		return []string{"状态        正在检查 GitHub Releases…"}
	case m.updateChecked && m.updateErr != nil:
		return []string{style.error.Render("状态        检查失败: " + m.updateErr.Error())}
	case !m.updateChecked:
		return []string{
			"状态        尚未检查",
			"提示        按 c 手动检查；打开 TUI 不会自动联网",
		}
	}

	report := m.updateReport
	lines := []string{
		"状态        " + updateStatusLabel(report.Status),
		"最新版本    " + displayValue(report.LatestVersion),
		"发布页      " + displayValue(report.ReleaseURL),
	}
	if report.UpdateAvailable {
		lines = append(lines,
			style.warning.Render("升级命令"),
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
		return "已是最新"
	case updater.StatusUpdateAvailable:
		return "有可用更新"
	case updater.StatusAhead:
		return "当前版本高于最新 Release"
	case updater.StatusDevelopment:
		return "开发构建，无法比较"
	default:
		return fmt.Sprintf("未知状态 (%s)", status)
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
