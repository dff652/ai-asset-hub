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
		m.notice = m.text(msgManageSelectUpdate)
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
		m.notice = m.text(msgManageSelectRemove)
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
			m.notice = m.text(msgManageConfirmMismatch, m.manageAction)
			m.noticeIsWarn = true
			return m, nil
		}
		m.confirmManage = false
		m.manageInput.Blur()
		m.managing = true
		m.noticeIsWarn = false
		if m.manageAction == manageUpdate {
			m.notice = m.text(msgManageUpdating)
			return m, updateAssetsCommand(workspace.UpdateOptions{
				WorkspaceRoot: m.workspace,
				Home:          m.options.Home,
				Project:       m.options.Project,
				Assets:        m.selectedUpdateAssets(),
			}, m.text(msgValidationFailed), m.text(msgValidationDetails))
		}
		m.notice = m.text(msgManageRemoving)
		return m, removeAssetsCommand(workspace.RemoveOptions{
			WorkspaceRoot: m.workspace,
			AssetIDs:      m.selectedRemoveIDs(),
		}, m.text(msgValidationFailed), m.text(msgValidationDetails))
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

func validateManagedManifest(manifestPath, root, validationPrefix, validationFallback string) error {
	report, err := validate.Validate(validate.Options{Manifest: manifestPath, Root: root})
	if err != nil {
		return err
	}
	if !report.Ok {
		return fmt.Errorf(
			"%s: %s",
			validationPrefix,
			firstErrorMessage(report, validationFallback),
		)
	}
	return nil
}

func updateAssetsCommand(
	options workspace.UpdateOptions,
	validationPrefix, validationFallback string,
) tea.Cmd {
	return func() tea.Msg {
		result, err := workspace.UpdateAssets(options, func(manifestPath, root string) error {
			return validateManagedManifest(
				manifestPath,
				root,
				validationPrefix,
				validationFallback,
			)
		})
		return manageMsg{action: manageUpdate, ok: result.Ok, count: len(result.Updated), err: err}
	}
}

func removeAssetsCommand(
	options workspace.RemoveOptions,
	validationPrefix, validationFallback string,
) tea.Cmd {
	return func() tea.Msg {
		result, err := workspace.RemoveAssets(options, func(manifestPath, root string) error {
			return validateManagedManifest(
				manifestPath,
				root,
				validationPrefix,
				validationFallback,
			)
		})
		return manageMsg{action: manageRemove, ok: result.Ok, count: len(result.Removed), err: err}
	}
}

func (m Model) manageNotice(message manageMsg) (string, bool) {
	if message.err != nil {
		return m.text(msgManageFailed, message.err), true
	}
	if message.action == manageUpdate {
		return m.text(msgManageUpdated, message.count), false
	}
	return m.text(msgManageRemoved, message.count), false
}

func (m Model) manageConfirmationView(style styles) string {
	action := m.text(msgManageUpdateAction)
	warning := m.text(msgManageUpdateWarning)
	if m.manageAction == manageRemove {
		action = m.text(msgManageRemoveAction)
		warning = m.text(msgManageRemoveWarning)
	}
	return strings.Join([]string{
		style.header.Render(m.text(msgManageConfirmationTitle)),
		"",
		action,
		warning,
		"",
		m.text(msgManageConfirmationPrompt, m.manageAction),
		m.manageInput.View(),
		"",
		m.text(msgManageConfirmationFooter),
	}, "\n")
}
