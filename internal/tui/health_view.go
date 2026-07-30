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
	state := m.text(msgHealthStateHealthy)
	if !m.doctorReport.Ok || m.doctorErr != nil {
		state = m.text(msgHealthStateRisks, len(m.doctorReport.Findings))
	}
	header := joinEdges(
		style.header.Render(m.text(msgHealthTitle)),
		state,
		max(20, m.width),
	)

	switch {
	case m.rollbackConfirming:
		return header + "\n" + m.rollbackConfirmationView(style)
	case m.rollbacking:
		return header + "\n\n" + style.warning.Render(m.text(msgHealthRollbacking))
	case m.doctorStatus == statusLoading:
		lines := []string{"", m.text(msgHealthChecking)}
		if m.rollbackResult != nil && m.rollbackResult.Ok {
			lines = append([]string{"", m.rollbackResultBanner(style)}, lines...)
		}
		return header + "\n" + strings.Join(lines, "\n")
	case m.doctorStatus == statusFailed:
		message := m.text(msgHealthFailed)
		if m.doctorErr != nil {
			message += ": " + m.doctorErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) + "\n" + m.text(msgHealthRetry)
	}

	summary := m.doctorSummary(style)
	body := m.healthBody(style)
	footerText := m.text(msgHealthFooter)
	if m.doctorReport.Ok && m.doctorReport.Deployment != nil &&
		m.doctorReport.Deployment.BackupID != "" {
		footerText = m.text(msgHealthFooterRollback)
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
	deployment := m.text(msgHealthNo)
	if summary.Deployment {
		deployment = m.text(msgHealthYes)
	}
	line := m.text(
		msgHealthSummary,
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
		return style.error.Render(m.text(msgHealthRollbackFailed))
	}
	return style.header.Render(m.text(
		msgHealthRollbackSucceeded,
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
		style.error.Render(m.text(msgHealthConfirmWarning)),
		m.text(msgHealthConfirmBackup, backupID),
		"",
		m.text(msgHealthConfirmAction),
		m.text(msgHealthConfirmPrompt),
		m.rollbackInput.View(),
		"",
		m.text(msgHealthConfirmFooter),
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) healthBody(style styles) string {
	rows := m.healthRows()
	if len(rows) == 0 {
		return m.text(msgHealthEmpty)
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
			label:      m.text(msgHealthRowCurrent, deployment.BackupID),
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
				label: m.text(
					msgHealthRowRollbackFinding,
					finding.Code,
					finding.Severity,
				),
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
			style.header.Render(m.text(msgHealthDetailCurrent)),
			m.text(msgHealthDetailBackup, row.deployment.BackupID),
			m.text(msgHealthDetailPackage, row.deployment.Package),
			m.text(msgHealthDetailVersion, row.deployment.Version),
			m.text(msgHealthDetailProfile, row.deployment.Profile),
			m.text(msgHealthDetailProducedBy, row.deployment.ProducedBy),
			m.text(msgHealthDetailAppliedAt, row.deployment.AppliedAt),
		}
	}
	if row.drift != nil {
		return []string{
			style.header.Render(row.drift.Path),
			m.text(msgHealthDetailStatus, row.drift.Status),
		}
	}
	if row.finding != nil {
		lines := []string{
			style.header.Render(row.finding.Code),
			m.text(msgHealthDetailSeverity, row.finding.Severity),
			m.text(msgHealthDetailDescription, row.finding.Message),
		}
		for _, path := range row.finding.Paths {
			lines = append(lines, "  "+path)
		}
		return lines
	}
	return []string{"—"}
}
