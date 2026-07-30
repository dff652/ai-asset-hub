package tui

import (
	"fmt"
	"strings"
)

func (m Model) migrationView(style styles) string {
	if m.migrationFlow.mode == migrationModeVersions {
		return m.versionsView(style)
	}
	if m.migrationFlow.mode == migrationModePreflight {
		return m.preflightView(style)
	}
	header := joinEdges(
		style.header.Render("aiah · 迁移到其他设备"),
		"版本对齐",
		max(20, m.width),
	)
	if m.migrationFlow.publishing {
		return header + "\n\n" + style.warning.Render("正在发布不可变版本…请等待操作完成")
	}
	if m.migrationFlow.pulling {
		return header + "\n\n" + style.warning.Render("正在取回并校验版本…请等待操作完成")
	}
	switch m.migrationFlow.status {
	case statusLoading:
		return header + "\n\n正在读取资产库、当前安装与分发通道…"
	case statusFailed:
		message := "迁移状态读取失败"
		if m.migrationFlow.err != nil {
			message += "：" + m.migrationFlow.err.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n按 c 重新选择通道 · r 重试 · m 返回首页"
	}

	report := m.migrationFlow.report
	lines := []string{
		header,
		style.muted.Render("状态读取保持只读；只有明确选择“发布”或“取回”才写通道/输出目录。"),
		"",
		style.header.Render("资产库"),
		"  路径      " + report.Library.Root,
		"  名称/版本 " + valueOrDash(report.Library.Name) + " / " + valueOrDash(report.Library.Version),
		fmt.Sprintf("  资产/组合 %d / %d（%s）",
			report.Library.AssetCount,
			len(report.Library.Profiles),
			valueOrDash(strings.Join(report.Library.Profiles, "、")),
		),
		"  状态      " + okLabel(report.Library.Ok),
		"",
		style.header.Render("当前安装"),
	}
	if !report.Installation.Present {
		lines = append(lines, "  尚无受管安装")
	} else {
		lines = append(lines,
			"  名称/版本 "+report.Installation.Package+" / "+report.Installation.Version,
			"  资产组合  "+valueOrDash(report.Installation.Profile),
			"  目标工具  "+valueOrDash(strings.Join(report.Installation.Targets, "、")),
			"  状态      "+okLabel(report.Installation.Ok),
		)
	}
	lines = append(lines, "", style.header.Render("分发通道"))
	if !report.Channel.Selected {
		lines = append(lines, "  未选择；按 c 输入已有的 Git/NAS/U 盘通道目录")
	} else {
		lines = append(lines,
			"  路径      "+report.Channel.Path,
			fmt.Sprintf("  相关版本  %d", report.Channel.ReleaseCount),
		)
		if report.Channel.Latest == nil {
			lines = append(lines, "  最近发布  —")
		} else {
			lines = append(lines,
				fmt.Sprintf("  最近发布  %s / %s",
					report.Channel.Latest.Version,
					report.Channel.Latest.Profile,
				),
				"  SHA256    "+report.Channel.Latest.SHA256,
			)
		}
	}
	lines = append(lines,
		"",
		style.header.Render("版本对齐"),
		"  当前安装  "+alignmentLabel(report.Alignment.Installation),
		"  分发通道  "+alignmentLabel(report.Alignment.Channel),
	)
	if len(report.Findings) > 0 {
		lines = append(lines, fmt.Sprintf("  风险与问题 %d（先处理后再迁移）", len(report.Findings)))
	}
	lines = append(lines,
		"",
		style.muted.Render("e 换机检查 · p 发布当前版本 · v 查看/取回版本 · c 选择通道 · r 刷新 · m 首页 · ? 帮助"),
	)
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		lines = append(lines, noticeStyle.Render(m.notice))
	}
	for index := range lines {
		lines[index] = truncate(lines[index], max(20, m.width))
	}
	return strings.Join(lines, "\n")
}

type preflightRow struct {
	label   string
	status  string
	details []string
}

func (m Model) preflightRows() []preflightRow {
	report := m.migrationFlow.preflightReport
	rows := make([]preflightRow, 0,
		len(report.Targets)+len(report.Secrets)+len(report.DevicePrivate)+len(report.Findings))
	for _, target := range report.Targets {
		status := "正常"
		switch {
		case !target.Supported:
			status = "不支持"
		case len(target.Dropped) > 0:
			status = "有阻止项"
		case len(target.Degraded) > 0:
			status = "有降级"
		}
		details := []string{
			"检查类型    目标工具适配",
			"目标工具    " + target.Target,
			"支持状态    " + status,
			fmt.Sprintf("将生成文件  %d", target.Emitted),
			fmt.Sprintf("阻止/降级   %d / %d", len(target.Dropped), len(target.Degraded)),
		}
		for _, item := range target.Dropped {
			details = append(details, "  阻止："+item)
		}
		for _, item := range target.Degraded {
			details = append(details, "  降级："+item)
		}
		rows = append(rows, preflightRow{
			label: "目标工具 · " + target.Target, status: status, details: details,
		})
	}
	for _, secret := range report.Secrets {
		status := "可用"
		if !secret.Available {
			status = "缺失"
		}
		rows = append(rows, preflightRow{
			label:  fmt.Sprintf("secret · %s:%s", secret.Provider, secret.Name),
			status: status,
			details: []string{
				"检查类型    secret 引用",
				fmt.Sprintf("引用        %s:%s", secret.Provider, secret.Name),
				"本机状态    " + status,
				"影响目标    " + valueOrDash(strings.Join(secret.Targets, "、")),
				"",
				"只检查可用性；不会把 secret 值写入报告或安装包。",
			},
		})
	}
	for _, item := range report.DevicePrivate {
		rows = append(rows, preflightRow{
			label:  "本机不迁移 · " + item.LogicalPath,
			status: "按设计排除",
			details: []string{
				"检查类型    本机私有项",
				"路径        " + item.LogicalPath,
				"来源工具    " + item.Source,
				"类型        " + item.Type,
				"状态        " + item.Status,
				"",
				"登录态、会话、缓存和设备状态不会进入迁移包。",
			},
		})
	}
	for _, finding := range report.Findings {
		details := []string{
			"检查类型    风险与问题",
			"代码        " + finding.Code,
			"级别        " + finding.Severity,
			"说明        " + finding.Message,
		}
		for _, path := range finding.Paths {
			details = append(details, "  "+path)
		}
		rows = append(rows, preflightRow{
			label: "问题 · " + finding.Code, status: finding.Severity, details: details,
		})
	}
	return rows
}

func (m Model) preflightView(style styles) string {
	header := joinEdges(
		style.header.Render("aiah · 换机前置检查"),
		valueOrDash(m.migrationFlow.preflightProfile),
		max(20, m.width),
	)
	switch m.migrationFlow.preflightStatus {
	case statusLoading:
		return header + "\n\n正在只读检查本机排除项、secret 和目标工具适配…"
	case statusFailed:
		message := "换机前置检查失败"
		if m.migrationFlow.preflightErr != nil {
			message += "：" + m.migrationFlow.preflightErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n按 r 重试 · Esc 返回版本对齐 · m 首页"
	}

	report := m.migrationFlow.preflightReport
	result := "可继续：未发现阻止项"
	resultStyle := style.header
	if !report.Ok {
		result = "需处理：存在阻止项"
		resultStyle = style.error
	} else if report.Summary.DegradedItems > 0 {
		result = "可继续：存在需确认的降级项"
		resultStyle = style.warning
	}
	summary := fmt.Sprintf(
		"目标 %d · 不支持 %d · 丢弃 %d · 降级 %d · secret 缺失 %d/%d · 本机不迁移 %d",
		report.Summary.TargetCount,
		report.Summary.UnsupportedTargets,
		report.Summary.DroppedItems,
		report.Summary.DegradedItems,
		report.Summary.MissingSecrets,
		report.Summary.SecretReferences,
		report.Summary.DevicePrivateItems,
	)
	rows := m.preflightRows()
	if len(rows) == 0 {
		return strings.Join([]string{
			header,
			style.muted.Render("检查阶段零写入；本页不会生成安装包、发布版本或应用资产。"),
			"",
			resultStyle.Render(result),
			summary,
			"",
			"没有需要展示的检查项。",
			"",
			style.muted.Render("r 重查 · p 发布 · v 查看/取回版本 · Esc 返回 · m 首页 · ? 帮助"),
		}, "\n")
	}

	cursor := m.migrationFlow.preflightCursor
	if cursor < 0 || cursor >= len(rows) {
		cursor = 0
	}
	bodyHeight := max(5, m.height-9)
	start, end := visibleRange(len(rows), cursor, bodyHeight)
	leftWidth := max(32, min(60, m.width*48/100))
	rightWidth := max(24, m.width-leftWidth-3)
	leftLines := make([]string, 0, bodyHeight)
	for index := start; index < end; index++ {
		prefix := "  "
		if index == cursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s [%s]", prefix, rows[index].label, rows[index].status)
		line = truncate(line, leftWidth)
		if index == cursor {
			line = style.selected.Render(line)
		}
		leftLines = append(leftLines, line)
	}
	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, "")
	}
	detailLines := append([]string(nil), rows[cursor].details...)
	for index := range detailLines {
		detailLines[index] = truncate(detailLines[index], rightWidth)
	}
	for len(detailLines) < bodyHeight {
		detailLines = append(detailLines, "")
	}
	if len(detailLines) > bodyHeight {
		detailLines = detailLines[:bodyHeight]
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

	lines := []string{
		header,
		style.muted.Render("检查阶段零写入；本页不会生成安装包、发布版本或应用资产。"),
		resultStyle.Render(result),
		truncate(summary, max(20, m.width)),
		strings.Join(combined, "\n"),
		style.muted.Render("↑↓/jk 查看全部项 · r 重查 · p 发布 · v 查看/取回版本 · Esc 返回 · m 首页 · ? 帮助"),
	}
	return strings.Join(lines, "\n")
}

func (m Model) channelInputView(style styles) string {
	lines := []string{
		style.header.Render("aiah · 选择分发通道"),
		"",
		"分发通道是一个已有的普通目录，可位于 Git checkout、NAS 挂载点或 U 盘。",
		"路径确认本身只读取 channel.json，不创建目录、不联网。",
		"",
		m.migrationFlow.channelInput.View(),
		"",
		"Enter 读取 · Esc 取消 · Ctrl+C 退出",
	}
	return strings.Join(appendInputNotice(lines, m.notice, m.noticeIsWarn, style), "\n")
}

func (m Model) publishConfirmationView(style styles) string {
	lines := []string{
		style.header.Render("aiah · 发布不可变版本"),
		"",
		"安装包    " + m.migrationFlow.publishPackage,
		"分发通道  " + m.migrationFlow.channel,
		"",
		style.warning.Render("发布会写入通道；同名/版本/组合一旦发布，不允许覆盖不同内容。"),
		"若通道目录为空，成功发布会在其中初始化索引和不可变包布局。",
		"请完整输入 publish 后按 Enter：",
		m.migrationFlow.publishInput.View(),
		"",
		"Esc 取消；取消时安装包仍保留在资产库 dist 目录。",
	}
	return strings.Join(appendInputNotice(lines, m.notice, m.noticeIsWarn, style), "\n")
}

func (m Model) versionsView(style styles) string {
	header := joinEdges(
		style.header.Render("aiah · 查看与取回版本"),
		filepathBaseOrDash(m.migrationFlow.channel),
		max(20, m.width),
	)
	switch m.migrationFlow.versionsStatus {
	case statusLoading:
		return header + "\n\n正在读取分发通道版本…"
	case statusFailed:
		message := "读取版本失败"
		if m.migrationFlow.versionsErr != nil {
			message += "：" + m.migrationFlow.versionsErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n按 r 重试 · Esc 返回版本对齐 · c 更换通道"
	}

	lines := []string{
		header,
		style.muted.Render("按发布顺序显示；不会比较版本号大小，也不会自动选择“最新版”。"),
		"",
	}
	if len(m.migrationFlow.versionsReport.Releases) == 0 {
		lines = append(lines,
			"该资产库尚未发布版本。",
			"",
			style.muted.Render("Esc 返回版本对齐，再按 p 发布 · r 刷新 · m 首页"),
		)
		return strings.Join(lines, "\n")
	}
	for index, release := range m.migrationFlow.versionsReport.Releases {
		prefix := "  "
		if index == m.migrationFlow.versionsCursor {
			prefix = "> "
		}
		line := fmt.Sprintf(
			"%s%-18s %-16s %s",
			prefix,
			release.Version,
			release.Profile,
			shortSHA(release.SHA256),
		)
		if index == m.migrationFlow.versionsCursor {
			line = style.selected.Render(line)
		}
		lines = append(lines, line)
	}
	selected := m.migrationFlow.versionsReport.Releases[m.migrationFlow.versionsCursor]
	lines = append(lines,
		"",
		"当前选择  "+selected.Name+" / "+selected.Version+" / "+selected.Profile,
		"SHA256    "+selected.SHA256,
		"",
		style.muted.Render("↑↓/jk 选择 · Enter 取回此版本 · r 刷新 · Esc 返回 · m 首页 · ? 帮助"),
	)
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		lines = append(lines, noticeStyle.Render(m.notice))
	}
	for index := range lines {
		lines[index] = truncate(lines[index], max(20, m.width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) pullOutputView(style styles) string {
	release := m.migrationFlow.selectedRelease
	lines := []string{
		style.header.Render("aiah · 取回版本"),
		"",
		"版本      " + release.Name + " / " + release.Version + " / " + release.Profile,
		"分发通道  " + m.migrationFlow.channel,
		"",
		"输入一个已有的输出目录；取回只写该目录，不写 .claude/.codex/.grok。",
		"同名产物残缺、内容不同或不是普通文件时会拒绝，不会覆盖。",
		"",
		m.migrationFlow.pullOutInput.View(),
		"",
		"Enter 取回并进入变更预览 · Esc 取消",
	}
	return strings.Join(appendInputNotice(lines, m.notice, m.noticeIsWarn, style), "\n")
}

func (m Model) migrationHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render("aiah · 迁移到其他设备 · 帮助"),
		"",
		"本页比较资产库版本、当前受管安装和分发通道最近发布版本。",
		"“版本不同”只表示不能证明相同，不会猜测哪个版本更新。",
		"",
		"e             选择资产组合并运行零写入换机前置检查",
		"p             选择资产组合、生成安装包，输入 publish 后发布",
		"v             查看该资产库的全部通道版本；明确选择后才能取回",
		"c             选择或更换已有分发通道目录",
		"r             重新读取全部状态",
		"m / Esc       返回任务首页",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
		"",
		"换机检查显示本机不迁移项、secret 可用性、不支持目标与 adapter 降级。",
		"取回后复用现有变更预览；仍须完整输入 apply 才写目标工具目录。",
		"发布与取回直接调用同一 Core，不执行 shell，也不接管 Git/rsync/U 盘传输。",
		"通道只负责不可变版本分发，不是后台双向同步。",
	}, "\n")
}

func filepathBaseOrDash(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "—"
	}
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func shortSHA(value string) string {
	if len(value) <= 12 {
		return valueOrDash(value)
	}
	return value[:12]
}

func appendInputNotice(lines []string, notice string, warning bool, style styles) []string {
	if notice == "" {
		return lines
	}
	noticeStyle := style.muted
	if warning {
		noticeStyle = style.warning
	}
	return append(lines, "", noticeStyle.Render(notice))
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func okLabel(ok bool) string {
	if ok {
		return "正常"
	}
	return "有风险与问题"
}

func alignmentLabel(value string) string {
	switch value {
	case "same-version":
		return "与资产库版本一致"
	case "different-version":
		return "与资产库版本不同（不判断新旧）"
	case "different-package":
		return "当前安装来自另一资产库"
	case "not-installed":
		return "尚未安装"
	case "not-published":
		return "资产库尚未发布到该通道"
	case "channel-not-selected":
		return "未选择通道"
	default:
		return valueOrDash(value)
	}
}
