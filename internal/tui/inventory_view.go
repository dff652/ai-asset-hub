package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func (m Model) View() string {
	style := newStyles(m.plain)
	if m.choosingWorkspace {
		return m.workspaceInputView(style)
	}
	if m.choosingProfile {
		return m.profileInputView(style)
	}
	if m.migrationFlow.choosingChannel {
		return m.channelInputView(style)
	}
	if m.migrationFlow.choosingPullOut {
		return m.pullOutputView(style)
	}
	if m.migrationFlow.publishConfirming {
		return m.publishConfirmationView(style)
	}
	if m.confirmManage {
		return m.manageConfirmationView(style)
	}
	if m.showHelp {
		return m.helpView(style)
	}
	if m.screen == screenHome {
		return m.homeView(style)
	}
	if m.screen == screenDeployment {
		return m.deploymentView(style)
	}
	if m.screen == screenHealth {
		return m.healthView(style)
	}
	if m.screen == screenVersion {
		return m.versionView(style)
	}
	if m.screen == screenMigration {
		return m.migrationView(style)
	}

	header := style.header.Render("aiah · 本机 AI 资产")
	counts := fmt.Sprintf("可加入 %d · 排除 %d · 风险与问题 %d",
		m.report.Summary.CandidateAssets, m.report.Summary.ExcludedAssets, len(m.report.Findings))
	if m.workspace != "" && m.catalogErr == nil {
		counts = fmt.Sprintf(
			"未纳管 %d · 已纳管 %d · 待更新 %d · 仅库内 %d",
			m.catalog.Summary.Unmanaged, m.catalog.Summary.Managed,
			m.catalog.Summary.SourceChanged, m.catalog.Summary.LibraryOnly,
		)
	}
	header = joinEdges(header, counts, max(20, m.width))

	workspaceLine := "资产库  未选择 · 跨工具资产的可编辑事实源 · 下一步：按 w 选择或创建"
	if m.workspace != "" {
		workspaceLine = "资产库  " + m.workspace +
			" · 跨工具统一管理（每项“目标工具”决定应用到 Claude/Codex/Grok）"
	}
	workspaceLine = style.muted.Render(truncate(workspaceLine, max(20, m.width)))

	filterLine := ""
	if m.filtering || m.filterInput.Value() != "" {
		filterLine = m.filterInput.View()
	} else {
		filterLine = style.muted.Render("/ 过滤路径、类型或风险与问题")
	}
	if m.findingsOnly {
		filterLine += style.warning.Render("  · 仅看风险与问题")
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

	keysHint := "↑↓/jk 导航 · w 选择资产库 · / 搜索 · f 风险 · r 重扫 · m 首页 · ? 帮助 · q 退出"
	if m.workspace != "" {
		keysHint = "空格选择 · w 纳入 · u 更新 · X 移出 · b 预览并应用 · m 首页 · ? 帮助"
	}
	if m.deployOptions.Package != "" {
		keysHint += " · d 变更预览"
	}
	if m.maintenance {
		keysHint += " · h 安装检查 · v 关于与更新"
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
	return header + "\n" + workspaceLine + "\n" + filterLine + "\n" + body + "\n" + footer
}

func (m Model) inventoryBody(style styles) string {
	rows := m.visibleRows()
	if len(rows) == 0 {
		return "\n没有匹配的资产或风险与问题"
	}

	bodyHeight := max(4, m.height-6)
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
	case rowSource, rowType, rowFindingGroup, rowLibraryGroup:
		if row.expanded {
			prefix += "▾ "
		} else {
			prefix += "▸ "
		}
	case rowAsset, rowLibraryAsset, rowFinding:
		if row.findings > 0 {
			prefix += "⚠ "
		} else {
			prefix += "  "
		}
		if m.selectableAsset(row) {
			if m.selected[row.key] {
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
			fmt.Sprintf("级别        %s", finding.Severity),
			fmt.Sprintf("说明        %s", finding.Message),
			fmt.Sprintf("路径        %d", len(finding.Paths)),
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
	if row.asset == nil && row.library == nil {
		kind := "来源工具"
		if row.kind == rowType {
			kind = "资产类型"
		} else if row.kind == rowFindingGroup {
			kind = "风险与问题"
		} else if row.kind == rowLibraryGroup {
			kind = "资产库状态"
		}
		return []string{
			style.header.Render(row.label),
			fmt.Sprintf("分组        %s", kind),
			fmt.Sprintf("风险与问题  %d", row.findings),
			"",
			"→/Enter 展开 · ←/Esc 收起",
		}
	}

	if row.library != nil && row.asset == nil {
		return []string{
			style.header.Render(row.library.ID),
			fmt.Sprintf("资产状态    %s", libraryStateLabel(row.library.State)),
			fmt.Sprintf("类型        %s", row.library.Type),
			fmt.Sprintf("资产库路径  %s", row.library.LibraryPath),
			fmt.Sprintf("目标工具    %s", strings.Join(row.library.Targets, "、")),
			"",
			"空格选择 · X 移出资产库",
		}
	}

	asset := *row.asset
	lines := []string{
		style.header.Render(asset.LogicalPath),
	}
	if row.library != nil {
		lines = append(lines, fmt.Sprintf("资产状态    %s", libraryStateLabel(row.libraryState())))
	}
	lines = append(lines,
		fmt.Sprintf("类型        %s", asset.Type),
		fmt.Sprintf("来源工具    %s", asset.Source),
		fmt.Sprintf("使用范围    %s", asset.Scope),
		fmt.Sprintf("可迁移性    %s", asset.Portability),
		fmt.Sprintf("敏感级别    %s", asset.Sensitivity),
		fmt.Sprintf("状态        %s", asset.Status),
		fmt.Sprintf("文件        %d", len(asset.Files)),
	)
	for _, file := range asset.Files {
		lines = append(lines, "  "+file)
		if len(lines) >= height-2 {
			break
		}
	}

	findings := m.findingsFor(asset)
	if len(findings) == 0 {
		lines = append(lines, "风险与问题  —")
	} else {
		lines = append(lines, fmt.Sprintf("风险与问题  %d", len(findings)))
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

func (row treeRow) libraryState() workspace.LibraryState {
	if row.library == nil {
		return workspace.LibraryBlocked
	}
	return row.library.State
}

func libraryStateLabel(state workspace.LibraryState) string {
	switch state {
	case workspace.LibraryUnmanaged:
		return "未纳管（可纳入）"
	case workspace.LibraryManaged:
		return "已纳管"
	case workspace.LibrarySourceChanged:
		return "源端有更新（可更新）"
	case workspace.LibraryOnly:
		return "仅在资产库"
	default:
		return "不可纳管"
	}
}

func (m Model) helpView(style styles) string {
	if m.screen == screenHome {
		return m.homeHelpView(style)
	}
	if m.screen == screenDeployment {
		return m.deploymentHelpView(style)
	}
	if m.screen == screenHealth {
		return m.healthHelpView(style)
	}
	if m.screen == screenVersion {
		return m.versionHelpView(style)
	}
	if m.screen == screenMigration {
		return m.migrationHelpView(style)
	}
	lines := []string{
		style.header.Render("aiah · 本机 AI 资产 · 帮助"),
		"",
		"↑/↓ 或 j/k   上下移动",
		"g / G         跳到开头 / 结尾",
		"→/Enter       展开分组",
		"←/Esc         收起分组或退出搜索",
		"/             增量过滤路径、类型或风险与问题",
		"f             只看有风险与问题的项",
		"r             重新扫描",
		"m             返回任务首页",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
	}
	if m.workspace == "" {
		lines = append(lines,
			"",
			"资产库是跨工具资产的可编辑事实源：assets/ 保存内容，manifest.yaml 保存清单。",
			"它不按 AI 工具拆目录；每项资产的“目标工具”决定应用到 Claude/Codex/Grok。",
			"当前页面只读。按 w 输入一个明确的资产库路径，或用 --workspace PATH 启动。",
			"路径确认前不会创建目录；aiah 不会猜测默认资产库。",
		)
	} else {
		lines = append(lines,
			"空格          勾选 / 取消勾选可管理资产",
			"w 纳入        把“未纳管”项目复制进资产库并登记清单",
			"u 更新        用源端完整替换“源端有更新”项目；需输入 update",
			"X 移出        从资产库和清单移出；不删源端；需输入 remove",
			"b 预览        选择资产组合，检查并准备安装包，然后展示变更预览",
			"a 应用        审阅变化后，再完整输入 apply 写目标工具目录",
			"",
			"资产库："+m.workspace,
			"资产库跨工具统一管理；manifest 中每项资产的 targets 是“目标工具”。",
			"纳入/更新/移出只写资产库，不写 .claude / .codex / .grok。",
			"成功后连续进入预览向导；校验不过则恢复操作前状态。",
			"安装恢复点只用于 rollback，不替代 Git/NAS 对资产库的备份。",
		)
		if findings := m.composeFindingLines(); len(findings) > 0 {
			lines = append(lines, "", "上次加入资产库时的跳过原因：")
			lines = append(lines, findings...)
		}
	}
	if m.deployOptions.Package != "" {
		lines = append(lines,
			"d             进入变更预览",
			"目标目录写入只在完整输入 apply 二次确认后发生。",
		)
	} else if m.workspace == "" {
		lines = append(lines, "未选择资产库或指定 --package 时，TUI 不提供应用入口。")
	}
	if m.maintenance {
		lines = append(lines,
			"h             运行只读安装检查，查看当前安装、备份与文件漂移",
			"v             查看 aiah、当前资产安装与 Release 版本",
			"安装检查通过且存在当前安装时，可在检查页按 x 撤销上次安装。",
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) workspaceInputView(style styles) string {
	lines := []string{
		style.header.Render("aiah · 选择资产库"),
		"",
		"资产库是跨工具资产的可编辑事实源，不是最终安装目录。",
		"assets/ 保存资产，manifest.yaml 保存清单、profile 与每项资产的 targets。",
		"targets（目标工具）决定资产应用到 Claude、Codex、Grok 或共享目录。",
		"",
		"输入要打开或创建的资产库路径：",
		m.workspaceInput.View(),
		"",
		"支持 ~/...；路径必须由你明确输入，不会使用隐藏默认值。",
		"确认后流程：选择资产 → 加入资产库 → 预览变化 → 确认应用。",
		"Enter 确认 · Esc 取消 · Ctrl+C 退出",
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) profileInputView(style styles) string {
	title := "aiah · 预览并应用资产库"
	next := "将先检查资产并在 dist/ 准备安装包，然后自动进入只读变更预览。"
	if m.profilePurpose == profileForPublish {
		title = "aiah · 发布资产库版本"
		next = "将先检查资产并在 dist/ 准备安装包，然后显示不可变发布确认。"
	} else if m.profilePurpose == profileForPreflight {
		title = "aiah · 换机前置检查"
		next = "只读检查本机排除项、secret 可用性和目标工具适配；不会生成安装包。"
	}
	lines := []string{
		style.header.Render(title),
		"",
		"资产库    " + m.workspace,
		"选择 manifest.yaml 中的资产组合（profile）：",
		m.profileInput.View(),
		"",
		next,
		"Enter 继续 · Esc 取消 · Ctrl+C 退出",
	}
	if len(m.availableProfiles) > 0 {
		lines = append(lines, "可用资产组合："+strings.Join(m.availableProfiles, "、"))
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) deploymentHelpView(style styles) string {
	lines := []string{
		style.header.Render("aiah · 变更预览 · 帮助"),
		"",
		"↑/↓ 或 j/k   上下移动",
		"g / G         跳到开头 / 结尾",
		"→/Enter       展开分组",
		"←             收起分组",
		"a             打开“确认应用”二次确认页（不会直接写）",
		"d             重新计算只读变更预览",
		"m             返回任务首页",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
		"",
		"执行前必须完整输入 apply。TUI 直接调用与 CLI 相同的 Diff / Apply Core。",
		"成功后显示备份 ID 与完整撤销命令；失败时保留 Core 原始问题代码。",
	}
	if m.maintenance {
		lines = append(lines,
			"h             运行安装检查",
			"v             查看 aiah、当前资产安装与 Release 版本",
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) healthHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render("aiah · 安装检查 · 帮助"),
		"",
		"↑/↓ 或 j/k   上下移动",
		"g / G         跳到开头 / 结尾",
		"h             重新运行安装检查",
		"x             撤销检查识别到的当前安装",
		"v             查看版本信息",
		"m             返回任务首页",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
		"",
		"只有安装检查通过且当前安装有备份 ID 时，才开放撤销。",
		"执行前必须完整输入 rollback；历史备份仍需通过 CLI 显式选择。",
	}, "\n")
}

func (m Model) versionHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render("aiah · 关于与更新 · 帮助"),
		"",
		"c             用户触发只读 Release 检查",
		"h             进入安装检查",
		"m             返回任务首页",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
		"",
		"打开版本页只读取本地程序和当前资产安装信息，不会联网。",
		"只有按 c 才查询 GitHub latest release；本页不替换当前二进制。",
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
