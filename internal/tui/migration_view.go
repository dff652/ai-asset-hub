package tui

import (
	"fmt"
	"strings"
)

func (m Model) migrationView(style styles) string {
	header := joinEdges(
		style.header.Render("aiah · 迁移到其他设备"),
		"只读状态",
		max(20, m.width),
	)
	switch m.migrationStatus {
	case statusLoading:
		return header + "\n\n正在读取资产库、当前安装与分发通道…"
	case statusFailed:
		message := "迁移状态读取失败"
		if m.migrationErr != nil {
			message += "：" + m.migrationErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n按 c 重新选择通道 · r 重试 · m 返回首页"
	}

	report := m.migrationReport
	lines := []string{
		header,
		style.muted.Render("本页不会生成、发布、取回或应用文件，只帮助判断下一步。"),
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
		style.muted.Render("c 选择通道 · r 刷新 · m 返回首页 · ? 帮助 · q 退出"),
	)
	for index := range lines {
		lines[index] = truncate(lines[index], max(20, m.width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) channelInputView(style styles) string {
	return strings.Join([]string{
		style.header.Render("aiah · 选择分发通道"),
		"",
		"分发通道是一个已有的普通目录，可位于 Git checkout、NAS 挂载点或 U 盘。",
		"本页只读取 channel.json，不创建目录、不联网、不发布或取回文件。",
		"",
		m.channelInput.View(),
		"",
		"Enter 读取 · Esc 取消 · Ctrl+C 退出",
	}, "\n")
}

func (m Model) migrationHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render("aiah · 迁移到其他设备 · 帮助"),
		"",
		"本页比较资产库版本、当前受管安装和分发通道最近发布版本。",
		"“版本不同”只表示不能证明相同，不会猜测哪个版本更新。",
		"",
		"c             选择或更换已有分发通道目录",
		"r             重新读取全部状态",
		"m / Esc       返回任务首页",
		"?             关闭帮助",
		"q / Ctrl+C    退出",
		"",
		"E3.1 只读：publish、pull、bootstrap 和 apply 仍使用现有 CLI。",
		"通道只负责不可变版本分发，不是后台双向同步。",
	}, "\n")
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
