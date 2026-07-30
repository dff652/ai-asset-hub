package tui

import (
	"fmt"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type healthRow struct {
	label      string
	deployment *apply.DoctorDeployment
	drift      *apply.DriftEntry
	finding    *workspace.Finding
}

func (m Model) healthView(style styles) string {
	state := "正常"
	if !m.doctorReport.Ok || m.doctorErr != nil {
		state = fmt.Sprintf("风险与问题 %d", len(m.doctorReport.Findings))
	}
	header := joinEdges(
		style.header.Render("aiah · 安装检查"),
		state,
		max(20, m.width),
	)

	switch {
	case m.rollbackConfirming:
		return header + "\n" + m.rollbackConfirmationView(style)
	case m.rollbacking:
		return header + "\n\n" + style.warning.Render("正在撤销上次安装…请等待事务完成")
	case m.doctorStatus == statusLoading:
		lines := []string{"", "正在检查当前安装…（可按 q 退出）"}
		if m.rollbackResult != nil && m.rollbackResult.Ok {
			lines = append([]string{"", m.rollbackResultBanner(style)}, lines...)
		}
		return header + "\n" + strings.Join(lines, "\n")
	case m.doctorStatus == statusFailed:
		message := "安装检查失败"
		if m.doctorErr != nil {
			message += ": " + m.doctorErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) + "\n按 h 重试，m 返回首页"
	}

	summary := m.doctorSummary(style)
	body := m.healthBody(style)
	footerText := "↑↓/jk 导航 · h 重新检查 · v 版本 · m 首页 · ? 帮助 · q 退出"
	if m.doctorReport.Ok && m.doctorReport.Deployment != nil &&
		m.doctorReport.Deployment.BackupID != "" {
		footerText = "↑↓/jk 导航 · h 重新检查 · x 撤销上次安装 · v 版本 · m 首页 · ? 帮助 · q 退出"
	}
	footer := style.muted.Render(footerText)
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		footer = noticeStyle.Render(m.notice) + "\n" + footer
	}
	banner := ""
	if m.rollbackResult != nil {
		banner = m.rollbackResultBanner(style) + "\n"
	}
	return header + "\n" + banner + summary + "\n" + body + "\n" + footer
}

func (m Model) doctorSummary(style styles) string {
	summary := m.doctorReport.Summary
	deployment := "否"
	if summary.Deployment {
		deployment = "是"
	}
	line := fmt.Sprintf(
		"当前安装 %s · 备份 %d · 未变化 %d · 已修改 %d · 缺失 %d · 风险与问题 %d",
		deployment,
		summary.Backups,
		summary.Unchanged,
		summary.LocallyModified,
		summary.Missing,
		len(m.doctorReport.Findings),
	)
	if !m.doctorReport.Ok {
		return style.error.Render(line)
	}
	return line
}

func (m Model) rollbackResultBanner(style styles) string {
	report := m.rollbackResult
	if report == nil {
		return ""
	}
	if !report.Ok || m.rollbackErr != nil {
		return style.error.Render("撤销失败；以下为 Core 原始风险与问题")
	}
	return style.header.Render(fmt.Sprintf(
		"撤销完成 · 备份 ID %s · 恢复 %d · 移除 %d",
		report.BackupID,
		len(report.Restored),
		len(report.Removed),
	))
}

func (m Model) rollbackConfirmationView(style styles) string {
	deployment := m.doctorReport.Deployment
	backupID := "—"
	if deployment != nil {
		backupID = deployment.BackupID
	}
	lines := []string{
		"",
		style.error.Render("即将撤销当前安装"),
		"备份 ID (backupId)  " + backupID,
		"",
		"撤销会恢复被更新的文件，并删除本次应用创建的文件。",
		"为防止误操作，请完整输入 rollback 后按 Enter：",
		m.rollbackInput.View(),
		"",
		"Esc 取消；打开本页不会写入。",
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) healthBody(style styles) string {
	rows := m.healthRows()
	if len(rows) == 0 {
		return "\n未发现当前安装、文件漂移或风险与问题"
	}
	bodyHeight := max(4, m.height-6)
	start, end := visibleRange(len(rows), m.healthCursor, bodyHeight)
	leftWidth := max(28, min(52, m.width*45/100))
	rightWidth := max(24, m.width-leftWidth-3)
	leftLines := make([]string, 0, bodyHeight)
	for index := start; index < end; index++ {
		label := truncate(rows[index].label, leftWidth)
		if index == m.healthCursor {
			label = style.selected.Render(label)
		}
		leftLines = append(leftLines, label)
	}
	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, "")
	}
	details := m.healthDetailLines(rows, style)
	for len(details) < bodyHeight {
		details = append(details, "")
	}
	combined := make([]string, 0, bodyHeight)
	for index := 0; index < bodyHeight; index++ {
		left := padRight(truncate(leftLines[index], leftWidth), leftWidth)
		right := truncate(details[index], rightWidth)
		line := left + " " + style.border.Render("│")
		if right != "" {
			line += " " + right
		}
		combined = append(combined, line)
	}
	return strings.Join(combined, "\n")
}

func (m Model) healthRows() []healthRow {
	rows := make([]healthRow, 0, 1+len(m.doctorReport.Drift)+
		len(m.doctorReport.Findings))
	if deployment := m.doctorReport.Deployment; deployment != nil {
		rows = append(rows, healthRow{
			label:      "当前安装 · " + deployment.BackupID,
			deployment: deployment,
		})
	}
	for index := range m.doctorReport.Drift {
		drift := &m.doctorReport.Drift[index]
		rows = append(rows, healthRow{
			label: fmt.Sprintf("%s · %s", drift.Status, drift.Path),
			drift: drift,
		})
	}
	if m.rollbackResult != nil {
		for index := range m.rollbackResult.Findings {
			finding := &m.rollbackResult.Findings[index]
			rows = append(rows, healthRow{
				label:   "撤销 · " + finding.Code + " · " + string(finding.Severity),
				finding: finding,
			})
		}
	}
	for index := range m.doctorReport.Findings {
		finding := &m.doctorReport.Findings[index]
		rows = append(rows, healthRow{
			label:   finding.Code + " · " + string(finding.Severity),
			finding: finding,
		})
	}
	return rows
}

func (m Model) healthDetailLines(rows []healthRow, style styles) []string {
	if m.healthCursor < 0 || m.healthCursor >= len(rows) {
		return []string{"—"}
	}
	row := rows[m.healthCursor]
	if row.deployment != nil {
		return []string{
			style.header.Render("当前安装"),
			"备份 ID     " + row.deployment.BackupID,
			"资产包      " + row.deployment.Package,
			"版本        " + row.deployment.Version,
			"资产组合    " + row.deployment.Profile,
			"生成工具    " + row.deployment.ProducedBy,
			"应用时间    " + row.deployment.AppliedAt,
		}
	}
	if row.drift != nil {
		return []string{
			style.header.Render(row.drift.Path),
			"状态        " + row.drift.Status,
		}
	}
	if row.finding != nil {
		lines := []string{
			style.header.Render(row.finding.Code),
			"级别        " + string(row.finding.Severity),
			"说明        " + row.finding.Message,
		}
		for _, path := range row.finding.Paths {
			lines = append(lines, "  "+path)
		}
		return lines
	}
	return []string{"—"}
}
