package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type homeAction int

const (
	homeActionNone homeAction = iota
	homeActionOrganize
	homeActionApply
	homeActionHealth
	homeActionMigration
	homeActionVersion
)

type homeItem struct {
	action      homeAction
	title       string
	description string
}

func homeItems() []homeItem {
	return []homeItem{
		{
			action:      homeActionOrganize,
			title:       "整理本机资产",
			description: "发现并加入统一资产库",
		},
		{
			action:      homeActionApply,
			title:       "预览并应用资产库",
			description: "检查资产、准备安装包并预览变化",
		},
		{
			action:      homeActionHealth,
			title:       "安装检查与撤销",
			description: "检查漂移，必要时撤销上次安装",
		},
		{
			action:      homeActionMigration,
			title:       "迁移到其他设备",
			description: "只读比较资产库、当前安装与分发通道",
		},
		{
			action:      homeActionVersion,
			title:       "关于与更新",
			description: "查看版本并手动检查 Release",
		},
	}
}

func (m Model) homeView(style styles) string {
	header := joinEdges(
		style.header.Render("AI 编程资产管理器"),
		"aiah",
		max(20, m.width),
	)
	lines := []string{
		header,
		style.muted.Render("统一管理 Claude、Codex、Grok 的资产，并安全迁移到新工具或设备"),
		"",
		"资产库    " + m.homeWorkspaceStatus(),
		"资产状态  " + m.homeInventoryStatus(),
		"当前安装  " + m.homeDeploymentStatus(),
		"",
		style.header.Render("你想做什么？"),
	}

	for index, item := range homeItems() {
		prefix := "  "
		if index == m.homeCursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%-18s %s", prefix, item.title, item.description)
		if index == m.homeCursor {
			line = style.selected.Render(line)
		}
		lines = append(lines, line)
	}

	lines = append(lines,
		"",
		style.muted.Render("↑↓/jk 选择 · Enter 进入 · ? 帮助 · q 退出"),
	)
	return strings.Join(lines, "\n")
}

func (m Model) homeWorkspaceStatus() string {
	if m.workspace == "" {
		return "未选择（整理或应用时再明确选择）"
	}
	return m.workspace
}

func (m Model) homeInventoryStatus() string {
	switch m.status {
	case statusLoading:
		return "正在扫描…"
	case statusFailed:
		return "扫描失败"
	default:
		if m.workspace != "" {
			if m.catalogErr != nil {
				return fmt.Sprintf(
					"资产库状态不可用 · 本机风险 %d",
					len(m.report.Findings),
				)
			}
			summary := m.catalog.Summary
			return fmt.Sprintf(
				"未纳管 %d · 已纳管 %d · 待更新 %d\n"+
					"          仅库内 %d · 不可纳管 %d · 本机问题 %d",
				summary.Unmanaged,
				summary.Managed,
				summary.SourceChanged,
				summary.LibraryOnly,
				summary.Blocked,
				len(m.report.Findings),
			)
		}
		return fmt.Sprintf(
			"发现 %d 项 · %d 个风险与问题",
			m.report.Summary.CandidateAssets,
			len(m.report.Findings),
		)
	}
}

func (m Model) homeDeploymentStatus() string {
	if m.doctorStatus == statusReady {
		if m.doctorReport.Deployment == nil {
			return "尚无受管安装"
		}
		deployment := m.doctorReport.Deployment
		packageVersion := strings.TrimSpace(strings.Join([]string{
			deployment.Package,
			deployment.Version,
		}, " "))
		parts := make([]string, 0, 2)
		if packageVersion != "" {
			parts = append(parts, packageVersion)
		}
		if len(deployment.Targets) > 0 {
			parts = append(parts, strings.Join(deployment.Targets, ","))
		}
		identity := strings.Join(parts, " · ")
		if identity == "" {
			identity = "受管安装"
		}
		if m.doctorReport.Ok {
			return identity + " · 正常"
		}
		return fmt.Sprintf("%s · %d 个风险与问题", identity, len(m.doctorReport.Findings))
	}
	if m.doctorErr != nil {
		return "检查失败"
	}
	return "正在检查…"
}

func (m Model) updateHomeKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := homeItems()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Up):
		if m.homeCursor > 0 {
			m.homeCursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.homeCursor+1 < len(items) {
			m.homeCursor++
		}
	case key.Matches(message, m.keys.First):
		m.homeCursor = 0
	case key.Matches(message, m.keys.Last):
		m.homeCursor = len(items) - 1
	case key.Matches(message, m.keys.Expand):
		return m.startHomeAction(items[m.homeCursor].action)
	}
	return m, nil
}

func (m Model) startHomeAction(action homeAction) (tea.Model, tea.Cmd) {
	switch action {
	case homeActionOrganize:
		if m.workspace == "" {
			return m.startWorkspaceInputFor(homeActionOrganize)
		}
		m.screen = screenInventory
		m.notice = "选择要管理的资产：空格勾选，按 w 加入资产库"
		m.noticeIsWarn = false
		return m, nil
	case homeActionApply:
		if m.workspace == "" {
			return m.startWorkspaceInputFor(homeActionApply)
		}
		return m.startProfileInput()
	case homeActionHealth:
		return m.startDoctor()
	case homeActionMigration:
		if m.workspace == "" {
			return m.startWorkspaceInputFor(homeActionMigration)
		}
		return m.startMigration()
	case homeActionVersion:
		return m.startVersion()
	default:
		return m, nil
	}
}

func (m Model) homeHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render("AI 编程资产管理器 · 帮助"),
		"",
		"这个工具把散落在 Claude、Codex、Grok 中的 AI 编程资产整理进统一资产库，",
		"并安全迁移到新工具或设备；写入前展示变化，写入后支持安装检查和撤销。",
		"",
		"整理本机资产      选择 skills、rules、agents 等并加入资产库",
		"预览并应用资产库  检查资产、准备安装包、预览变化，再确认应用",
		"安装检查与撤销    检查当前安装、文件漂移和备份",
		"迁移到其他设备    比较资产库、当前安装与分发通道；本阶段只读",
		"关于与更新        查看本地版本；只有再次按 c 才联网检查 Release",
		"",
		"资产库是跨工具统一、可编辑、可进入 Git 的事实源，不是工具安装目录。",
		"↑↓/jk 选择 · Enter 进入 · ? 关闭帮助 · q 退出",
	}, "\n")
}
