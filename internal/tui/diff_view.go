package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type deploymentRowKind int

const (
	deploymentGroupRow deploymentRowKind = iota
	deploymentChangeRow
	deploymentFindingRow
)

type deploymentRow struct {
	kind     deploymentRowKind
	key      string
	label    string
	expanded bool
	change   *apply.Change
	finding  *workspace.Finding
	depth    int
}

func (m Model) deploymentView(style styles) string {
	title := "aiah · deployment diff"
	if m.applyResult != nil {
		title = "aiah · deployment result"
	}
	header := joinEdges(
		style.header.Render(title),
		filepath.Base(m.deployOptions.Package),
		max(20, m.width),
	)

	switch {
	case m.confirming:
		return header + "\n" + m.confirmationView(style)
	case m.applying:
		return header + "\n\n" + style.warning.Render("正在执行 apply…请等待事务完成")
	case m.diffStatus == statusLoading:
		return header + "\n\n正在计算只读 diff…（可按 q 退出）"
	case m.diffStatus == statusFailed:
		message := "diff 失败"
		if m.deployErr != nil {
			message += ": " + m.deployErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) + "\n按 d 重试，Esc 返回 inventory"
	}

	var banner string
	report := m.diffReport
	if m.applyResult != nil {
		report = *m.applyResult
		banner = m.applyResultBanner(style, report)
	} else if !report.Ok {
		banner = style.error.Render("diff 未通过；以下为 Core 原始 findings")
	} else {
		banner = fmt.Sprintf(
			"只读计划 · create %d · update %d · unchanged %d · skipped %d",
			report.Summary.Create,
			report.Summary.Update,
			report.Summary.Unchanged,
			report.Summary.Skipped,
		)
	}

	body := m.deploymentBody(style, report)
	footer := style.muted.Render("↑↓/jk 导航 · ←→ 折叠 · a apply · d 重算 diff · Esc inventory · ? 帮助 · q 退出")
	if m.applyResult != nil || !report.Ok {
		footer = style.muted.Render("↑↓/jk 导航 · ←→ 折叠 · d 重算 diff · Esc inventory · ? 帮助 · q 退出")
	}
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		footer = noticeStyle.Render(m.notice) + "\n" + footer
	}
	return header + "\n" + banner + "\n" + body + "\n" + footer
}

func (m Model) confirmationView(style styles) string {
	report := m.diffReport
	lines := []string{
		"",
		style.error.Render("即将写入目标目录"),
		fmt.Sprintf(
			"create %d · update %d · unchanged %d · skipped %d",
			report.Summary.Create,
			report.Summary.Update,
			report.Summary.Unchanged,
			report.Summary.Skipped,
		),
		"",
		"这是第二次确认。完整输入 apply 后按 Enter 执行：",
		m.confirmInput.View(),
		"",
		"Esc 取消；单独按 a 不会写入。",
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) applyResultBanner(style styles, report apply.Report) string {
	if !report.Ok || m.deployErr != nil {
		return style.error.Render("apply 失败；以下为 Core 原始 findings")
	}
	if report.BackupID == "" {
		return style.header.Render("应用完成 · written 0 · backupId — · 无需回滚")
	}
	return strings.Join([]string{
		style.header.Render(fmt.Sprintf("应用完成 · written %d", report.Summary.Written)),
		style.header.Render("backupId  " + report.BackupID),
		style.warning.Render("rollback  " + rollbackCommand(m.deployOptions, report.BackupID)),
	}, "\n")
}

func (m Model) deploymentBody(style styles, report apply.Report) string {
	if !report.Ok || m.deployErr != nil {
		return m.failureBody(style, report)
	}

	rows := m.deploymentRowsFor(report)
	if len(rows) == 0 {
		return "\n没有文件变化"
	}
	bodyHeight := max(4, m.height-7)
	start, end := visibleRange(len(rows), m.diffCursor, bodyHeight)
	leftWidth := max(28, min(56, m.width*55/100))
	rightWidth := max(24, m.width-leftWidth-3)
	leftLines := make([]string, 0, bodyHeight)
	for index := start; index < end; index++ {
		leftLines = append(leftLines, m.renderDeploymentRow(
			rows[index],
			index == m.diffCursor,
			leftWidth,
			style,
		))
	}
	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, "")
	}
	detailLines := m.deploymentDetailLines(rows, style)
	for len(detailLines) < bodyHeight {
		detailLines = append(detailLines, "")
	}
	combined := make([]string, 0, bodyHeight)
	for index := 0; index < bodyHeight; index++ {
		left := padRight(truncate(leftLines[index], leftWidth), leftWidth)
		right := truncate(detailLines[index], rightWidth)
		line := left + " " + style.border.Render("│")
		if right != "" {
			line += " " + right
		}
		combined = append(combined, line)
	}
	return strings.Join(combined, "\n")
}

func (m Model) failureBody(style styles, report apply.Report) string {
	if m.deployErr != nil && len(report.Findings) == 0 {
		return "\n" + style.error.Render(m.deployErr.Error())
	}
	lines := make([]string, 0)
	for _, finding := range report.Findings {
		lines = append(lines,
			fmt.Sprintf("%s · %s", finding.Code, finding.Severity),
			finding.Message,
		)
		for _, path := range finding.Paths {
			lines = append(lines, "  "+path)
		}
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		return "\n—"
	}
	return strings.Join(lines, "\n")
}

func (m Model) deploymentRows() []deploymentRow {
	report := m.diffReport
	if m.applyResult != nil {
		report = *m.applyResult
	}
	return m.deploymentRowsFor(report)
}

func (m Model) deploymentRowsFor(report apply.Report) []deploymentRow {
	rows := make([]deploymentRow, 0, len(report.Changes)+5)
	for _, action := range []string{"create", "update", "unchanged", "skipped"} {
		key := "action:" + action
		count := 0
		for _, change := range report.Changes {
			if change.Action == action {
				count++
			}
		}
		if count == 0 {
			continue
		}
		expanded := m.diffExpanded[key]
		rows = append(rows, deploymentRow{
			kind: deploymentGroupRow, key: key,
			label: fmt.Sprintf("%s (%d)", action, count), expanded: expanded,
		})
		if !expanded {
			continue
		}
		for index := range report.Changes {
			if report.Changes[index].Action != action {
				continue
			}
			change := &report.Changes[index]
			rows = append(rows, deploymentRow{
				kind: deploymentChangeRow, key: action + ":" + change.Path,
				label: change.Path, change: change, depth: 1,
			})
		}
	}
	if len(report.Findings) > 0 {
		expanded := m.diffExpanded["findings"]
		rows = append(rows, deploymentRow{
			kind: deploymentGroupRow, key: "findings",
			label: fmt.Sprintf("findings (%d)", len(report.Findings)), expanded: expanded,
		})
		if expanded {
			for index := range report.Findings {
				finding := &report.Findings[index]
				rows = append(rows, deploymentRow{
					kind:    deploymentFindingRow,
					key:     fmt.Sprintf("finding:%d:%s", index, finding.Code),
					label:   fmt.Sprintf("%s · %s", finding.Code, finding.Severity),
					finding: finding, depth: 1,
				})
			}
		}
	}
	return rows
}

func (m Model) renderDeploymentRow(row deploymentRow, selected bool, width int, style styles) string {
	prefix := strings.Repeat("  ", row.depth)
	if row.kind == deploymentGroupRow {
		if row.expanded {
			prefix += "▾ "
		} else {
			prefix += "▸ "
		}
	} else {
		prefix += "  "
	}
	label := truncate(prefix+row.label, width)
	if selected {
		return style.selected.Render(label)
	}
	return label
}

func (m Model) deploymentDetailLines(rows []deploymentRow, style styles) []string {
	if m.diffCursor < 0 || m.diffCursor >= len(rows) {
		return []string{"—"}
	}
	row := rows[m.diffCursor]
	if row.change != nil {
		return []string{
			style.header.Render(row.change.Path),
			"action      " + row.change.Action,
			"sha256      " + row.change.SHA256,
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
	return []string{
		style.header.Render(row.label),
		"→/Enter 展开 · ← 收起",
	}
}
