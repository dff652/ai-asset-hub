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
	state := "healthy"
	if !m.doctorReport.Ok || m.doctorErr != nil {
		state = fmt.Sprintf("findings %d", len(m.doctorReport.Findings))
	}
	header := joinEdges(
		style.header.Render("aiah · doctor"),
		state,
		max(20, m.width),
	)

	switch {
	case m.rollbackConfirming:
		return header + "\n" + m.rollbackConfirmationView(style)
	case m.rollbacking:
		return header + "\n\n" + style.warning.Render("正在执行 rollback…请等待事务完成")
	case m.doctorStatus == statusLoading:
		lines := []string{"", "正在执行只读 doctor…（可按 q 退出）"}
		if m.rollbackResult != nil && m.rollbackResult.Ok {
			lines = append([]string{"", m.rollbackResultBanner(style)}, lines...)
		}
		return header + "\n" + strings.Join(lines, "\n")
	case m.doctorStatus == statusFailed:
		message := "doctor 失败"
		if m.doctorErr != nil {
			message += ": " + m.doctorErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) + "\n按 h 重试，Esc 返回 inventory"
	}

	summary := m.doctorSummary(style)
	body := m.healthBody(style)
	footerText := "↑↓/jk 导航 · h 重跑 doctor · v version · Esc inventory · ? 帮助 · q 退出"
	if m.doctorReport.Ok && m.doctorReport.Deployment != nil &&
		m.doctorReport.Deployment.BackupID != "" {
		footerText = "↑↓/jk 导航 · h doctor · x rollback · v version · Esc inventory · ? 帮助 · q 退出"
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
	deployment := "no"
	if summary.Deployment {
		deployment = "yes"
	}
	line := fmt.Sprintf(
		"deployment %s · backups %d · unchanged %d · modified %d · missing %d · findings %d",
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
		return style.error.Render("rollback 失败；以下为 Core 原始 findings")
	}
	return style.header.Render(fmt.Sprintf(
		"回滚完成 · backupId %s · restored %d · removed %d",
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
		style.error.Render("即将回滚当前部署"),
		"backupId  " + backupID,
		"",
		"rollback 会恢复被更新的文件，并删除本次 apply 创建的文件。",
		"完整输入 rollback 后按 Enter 执行：",
		m.rollbackInput.View(),
		"",
		"Esc 取消；单独按 x 不会写入。",
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) healthBody(style styles) string {
	rows := m.healthRows()
	if len(rows) == 0 {
		return "\n未发现当前部署、drift 或 finding"
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
			label:      "deployment · " + deployment.BackupID,
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
				label:   "rollback · " + finding.Code + " · " + string(finding.Severity),
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
			style.header.Render("current deployment"),
			"backupId    " + row.deployment.BackupID,
			"package     " + row.deployment.Package,
			"version     " + row.deployment.Version,
			"profile     " + row.deployment.Profile,
			"producedBy  " + row.deployment.ProducedBy,
			"appliedAt   " + row.deployment.AppliedAt,
		}
	}
	if row.drift != nil {
		return []string{
			style.header.Render(row.drift.Path),
			"status      " + row.drift.Status,
		}
	}
	if row.finding != nil {
		lines := []string{
			style.header.Render(row.finding.Code),
			"severity    " + string(row.finding.Severity),
			"message     " + row.finding.Message,
		}
		for _, path := range row.finding.Paths {
			lines = append(lines, "  "+path)
		}
		return lines
	}
	return []string{"—"}
}
