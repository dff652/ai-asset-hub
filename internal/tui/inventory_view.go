package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	style := newStyles(m.plain)
	if m.showHelp {
		return m.helpView(style)
	}
	if m.screen == screenDeployment {
		return m.deploymentView(style)
	}

	header := style.header.Render("aiah · inventory")
	counts := fmt.Sprintf(
		"候选 %d · 排除 %d · findings %d",
		m.report.Summary.CandidateAssets,
		m.report.Summary.ExcludedAssets,
		len(m.report.Findings),
	)
	header = joinEdges(header, counts, max(20, m.width))

	filterLine := ""
	if m.filtering || m.filterInput.Value() != "" {
		filterLine = m.filterInput.View()
	} else {
		filterLine = style.muted.Render("/ 过滤路径、类型或 finding")
	}
	if m.findingsOnly {
		filterLine += style.warning.Render("  · 仅看 findings")
	}

	var body string
	switch m.status {
	case statusLoading:
		body = "\n正在扫描…（可按 q 退出）"
	case statusFailed:
		message := "扫描失败"
		if m.err != nil {
			message += ": " + m.err.Error()
		}
		body = "\n" + style.error.Render(message) + "\n按 r 重试，q 退出"
	default:
		body = m.inventoryBody(style)
	}

	keysHint := "↑↓/jk 导航 · / 搜索 · f findings · r 重扫 · ? 帮助 · q 退出"
	if m.workspace != "" {
		keysHint = "↑↓/jk 导航 · 空格 勾选 · w 写出工作区 · / 搜索 · f findings · r 重扫 · ? 帮助 · q 退出"
	}
	if m.deployOptions.Package != "" {
		keysHint += " · d 部署 diff"
	}
	footer := style.muted.Render(keysHint)
	if selected := len(m.selected); selected > 0 {
		footer = style.header.Render(fmt.Sprintf("已勾选 %d", selected)) + "  " + footer
	}
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		footer = noticeStyle.Render(m.notice) + "\n" + footer
	}
	return header + "\n" + filterLine + "\n" + body + "\n" + footer
}

func (m Model) inventoryBody(style styles) string {
	rows := m.visibleRows()
	if len(rows) == 0 {
		return "\n没有匹配的资产或 finding"
	}

	bodyHeight := max(4, m.height-5)
	start, end := visibleRange(len(rows), m.cursor, bodyHeight)
	leftWidth := max(28, min(52, m.width*45/100))
	rightWidth := max(24, m.width-leftWidth-3)

	leftLines := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		leftLines = append(leftLines, m.renderTreeRow(rows[index], index == m.cursor, leftWidth, style))
	}
	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, "")
	}

	detailLines := m.detailLines(rows, rightWidth, bodyHeight, style)
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

func (m Model) renderTreeRow(row treeRow, selected bool, width int, style styles) string {
	prefix := strings.Repeat("  ", row.depth)
	switch row.kind {
	case rowSource, rowType, rowFindingGroup:
		if row.expanded {
			prefix += "▾ "
		} else {
			prefix += "▸ "
		}
	case rowAsset, rowFinding:
		if row.findings > 0 {
			prefix += "⚠ "
		} else {
			prefix += "  "
		}
		if m.selectableAsset(row) {
			if m.selected[row.asset.LogicalPath] {
				prefix += "[x] "
			} else {
				prefix += "[ ] "
			}
		}
	}
	label := prefix + row.label
	if row.findings > 0 &&
		(row.kind == rowSource || row.kind == rowType || row.kind == rowFindingGroup) {
		label += fmt.Sprintf(" (%d)", row.findings)
	}
	label = truncate(label, width)
	if selected {
		return style.selected.Render(label)
	}
	return label
}

func (m Model) detailLines(rows []treeRow, width, height int, style styles) []string {
	if m.cursor < 0 || m.cursor >= len(rows) {
		return []string{"—"}
	}
	row := rows[m.cursor]
	if row.finding != nil {
		finding := *row.finding
		lines := []string{
			style.header.Render(string(finding.Code)),
			fmt.Sprintf("severity    %s", finding.Severity),
			fmt.Sprintf("message     %s", finding.Message),
			fmt.Sprintf("paths       %d", len(finding.Paths)),
		}
		for _, path := range finding.Paths {
			lines = append(lines, "  "+path)
			if len(lines) >= height {
				break
			}
		}
		for index := range lines {
			lines[index] = truncate(lines[index], width)
		}
		return lines
	}
	if row.asset == nil {
		kind := "source"
		if row.kind == rowType {
			kind = "type"
		} else if row.kind == rowFindingGroup {
			kind = "findings"
		}
		return []string{
			style.header.Render(row.label),
			fmt.Sprintf("group       %s", kind),
			fmt.Sprintf("findings    %d", row.findings),
			"",
			"→/Enter 展开 · ←/Esc 收起",
		}
	}

	asset := *row.asset
	lines := []string{
		style.header.Render(asset.LogicalPath),
		fmt.Sprintf("type        %s", asset.Type),
		fmt.Sprintf("source      %s", asset.Source),
		fmt.Sprintf("scope       %s", asset.Scope),
		fmt.Sprintf("portability %s", asset.Portability),
		fmt.Sprintf("sensitivity %s", asset.Sensitivity),
		fmt.Sprintf("status      %s", asset.Status),
		fmt.Sprintf("files       %d", len(asset.Files)),
	}
	for _, file := range asset.Files {
		lines = append(lines, "  "+file)
		if len(lines) >= height-2 {
			break
		}
	}

	findings := m.findingsFor(asset)
	if len(findings) == 0 {
		lines = append(lines, "findings    —")
	} else {
		lines = append(lines, fmt.Sprintf("findings    %d", len(findings)))
		for _, finding := range findings {
			lines = append(lines, fmt.Sprintf("  %s · %s", finding.Severity, finding.Code))
			if len(lines) >= height {
				break
			}
		}
	}
	for index := range lines {
		lines[index] = truncate(lines[index], width)
	}
	return lines
}

func (m Model) helpView(style styles) string {
	if m.screen == screenDeployment {
		return m.deploymentHelpView(style)
	}
	lines := []string{
		style.header.Render("aiah · inventory · 帮助"),
		"",
		"↑/↓ 或 j/k   上下移动",
		"g / G         跳到开头 / 结尾",
		"→/Enter       展开分组",
		"←/Esc         收起分组或退出搜索",
		"/             增量过滤路径、类型或 finding",
		"f             只看有 finding 的项",
		"r             重新扫描",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
	}
	if m.workspace == "" {
		lines = append(lines, "", "当前 inventory 只读。加 --workspace PATH 启动可勾选资产并写出 manifest。")
	} else {
		lines = append(lines,
			"空格          勾选 / 取消勾选候选资产",
			"w             把勾选项复制进工作区并登记进 manifest",
			"",
			"工作区："+m.workspace,
			"只写工作区：已存在的文件不覆盖，界面永不写 .claude / .codex / .grok。",
			"校验不过则整单回滚，不留半成品。",
		)
		if findings := m.composeFindingLines(); len(findings) > 0 {
			lines = append(lines, "", "上次写出的跳过原因：")
			lines = append(lines, findings...)
		}
	}
	if m.deployOptions.Package == "" {
		lines = append(lines, "未指定 --package；apply 仍只在 CLI 执行。")
	} else {
		lines = append(lines,
			"d             进入部署 diff 审阅",
			"部署写入只在完整输入 apply 二次确认后发生。",
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) deploymentHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render("aiah · deployment · 帮助"),
		"",
		"↑/↓ 或 j/k   上下移动",
		"g / G         跳到开头 / 结尾",
		"→/Enter       展开分组",
		"←             收起分组",
		"a             打开 apply 二次确认（不会直接写）",
		"d             重新计算只读 diff",
		"Esc           返回 inventory",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
		"",
		"执行前必须完整输入 apply。TUI 直接调用与 CLI 相同的 apply.Diff / apply.Apply。",
		"成功后显示 backupId 与完整 rollback 命令；失败 finding 保持 Core 原文。",
	}, "\n")
}

func visibleRange(total, cursor, height int) (int, int) {
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	var builder strings.Builder
	for _, character := range value {
		next := builder.String() + string(character)
		if lipgloss.Width(next)+1 > width {
			break
		}
		builder.WriteRune(character)
	}
	return builder.String() + "…"
}

func padRight(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func joinEdges(left, right string, width int) string {
	padding := width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		return truncate(left+" "+right, width)
	}
	return left + strings.Repeat(" ", padding) + right
}
