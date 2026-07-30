package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/validate"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

const (
	manageUpdate = "update"
	manageRemove = "remove"
)

type manageMsg struct {
	action string
	ok     bool
	count  int
	err    error
}

func (m Model) startAssetUpdate() (tea.Model, tea.Cmd) {
	assets := m.selectedUpdateAssets()
	if len(assets) == 0 {
		m.notice = "先勾选状态为“源端有更新”的资产"
		m.noticeIsWarn = true
		return m, nil
	}
	m.manageAction = manageUpdate
	m.confirmManage = true
	m.manageInput.Placeholder = manageUpdate
	m.manageInput.SetValue("")
	m.notice = ""
	return m, m.manageInput.Focus()
}

func (m Model) startAssetRemove() (tea.Model, tea.Cmd) {
	ids := m.selectedRemoveIDs()
	if len(ids) == 0 {
		m.notice = "先勾选已纳管、待更新或仅在资产库的资产"
		m.noticeIsWarn = true
		return m, nil
	}
	m.manageAction = manageRemove
	m.confirmManage = true
	m.manageInput.Placeholder = manageRemove
	m.manageInput.SetValue("")
	m.notice = ""
	return m, m.manageInput.Focus()
}

func (m Model) updateManageConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.confirmManage = false
		m.manageAction = ""
		m.manageInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		if strings.TrimSpace(m.manageInput.Value()) != m.manageAction {
			m.notice = "确认词不匹配；请输入 " + m.manageAction
			m.noticeIsWarn = true
			return m, nil
		}
		m.confirmManage = false
		m.manageInput.Blur()
		m.managing = true
		m.noticeIsWarn = false
		if m.manageAction == manageUpdate {
			m.notice = "正在更新资产库…"
			return m, updateAssetsCommand(workspace.UpdateOptions{
				WorkspaceRoot: m.workspace,
				Home:          m.options.Home,
				Project:       m.options.Project,
				Assets:        m.selectedUpdateAssets(),
			})
		}
		m.notice = "正在从资产库移出…"
		return m, removeAssetsCommand(workspace.RemoveOptions{
			WorkspaceRoot: m.workspace,
			AssetIDs:      m.selectedRemoveIDs(),
		})
	}
	var command tea.Cmd
	m.manageInput, command = m.manageInput.Update(message)
	return m, command
}

func (m Model) selectedUpdateAssets() []inventory.Asset {
	selectedState := make(map[string]workspace.LibraryState, len(m.catalog.Items))
	for _, item := range m.catalog.Items {
		selectedState[item.LogicalPath] = item.State
	}
	var assets []inventory.Asset
	for _, asset := range m.report.Assets {
		if m.selected[asset.LogicalPath] &&
			selectedState[asset.LogicalPath] == workspace.LibrarySourceChanged {
			assets = append(assets, asset)
		}
	}
	return assets
}

func (m Model) selectedRemoveIDs() []string {
	var ids []string
	for _, item := range m.catalog.Items {
		if item.State != workspace.LibraryManaged &&
			item.State != workspace.LibrarySourceChanged &&
			item.State != workspace.LibraryOnly {
			continue
		}
		key := item.LogicalPath
		if item.State == workspace.LibraryOnly {
			key = "library:" + item.ID
		}
		if m.selected[key] {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func validateManagedManifest(manifestPath, root string) error {
	report, err := validate.Validate(validate.Options{Manifest: manifestPath, Root: root})
	if err != nil {
		return err
	}
	if !report.Ok {
		return fmt.Errorf("manifest validation failed: %s", firstErrorMessage(report))
	}
	return nil
}

func updateAssetsCommand(options workspace.UpdateOptions) tea.Cmd {
	return func() tea.Msg {
		result, err := workspace.UpdateAssets(options, validateManagedManifest)
		return manageMsg{action: manageUpdate, ok: result.Ok, count: len(result.Updated), err: err}
	}
}

func removeAssetsCommand(options workspace.RemoveOptions) tea.Cmd {
	return func() tea.Msg {
		result, err := workspace.RemoveAssets(options, validateManagedManifest)
		return manageMsg{action: manageRemove, ok: result.Ok, count: len(result.Removed), err: err}
	}
}

func manageNotice(message manageMsg) (string, bool) {
	if message.err != nil {
		return "资产库操作失败：" + message.err.Error() + "（已恢复操作前状态）", true
	}
	if message.action == manageUpdate {
		return fmt.Sprintf("已更新 %d 项资产，继续选择资产组合并预览应用", message.count), false
	}
	return fmt.Sprintf("已移出 %d 项资产，继续选择资产组合并预览应用", message.count), false
}

func (m Model) manageConfirmationView(style styles) string {
	action := "用源端内容替换资产库中的完整资产"
	warning := "只修改资产库，不写 AI 工具目录；验证失败会恢复原内容。"
	if m.manageAction == manageRemove {
		action = "从 manifest.yaml 和资产库中移出所选资产"
		warning = "源端文件不会删除；此操作不等于备份，请先用 Git/NAS 保护资产库。"
	}
	return strings.Join([]string{
		style.header.Render("aiah · 确认资产库操作"),
		"",
		action,
		warning,
		"",
		"输入 " + m.manageAction + " 后按 Enter：",
		m.manageInput.View(),
		"",
		"Esc 取消 · Ctrl+C 退出",
	}, "\n")
}
