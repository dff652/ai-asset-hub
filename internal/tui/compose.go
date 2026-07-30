package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/validate"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

// composeMsg carries the result of adding selected assets to the library.
type composeMsg struct {
	result workspace.ComposeResult
	err    error
}

// selectableAsset reports whether a row can be ticked. Only candidate assets
// can become manifest entries; findings and grouping rows cannot.
//
// Without a workspace nothing is selectable, so the read-only UI shows no
// checkboxes at all rather than offering a control that cannot be honoured.
func (m Model) selectableAsset(row treeRow) bool {
	if m.workspace == "" || row.library == nil {
		return false
	}
	switch row.library.State {
	case workspace.LibraryUnmanaged, workspace.LibraryManaged,
		workspace.LibrarySourceChanged, workspace.LibraryOnly:
		return row.kind == rowAsset || row.kind == rowLibraryAsset
	default:
		return false
	}
}

// selectedAssets returns the ticked assets in report order, so a compose run is
// deterministic regardless of the order the user ticked them.
func (m Model) selectedAssets() []inventory.Asset {
	assets := make([]inventory.Asset, 0, len(m.selected))
	for _, asset := range m.report.Assets {
		if m.selected[asset.LogicalPath] {
			assets = append(assets, asset)
		}
	}
	return assets
}

func (m Model) selectedAddAssets() []inventory.Asset {
	state := make(map[string]workspace.LibraryState, len(m.catalog.Items))
	for _, item := range m.catalog.Items {
		state[item.LogicalPath] = item.State
	}
	assets := make([]inventory.Asset, 0, len(m.selected))
	for _, asset := range m.report.Assets {
		if m.selected[asset.LogicalPath] && state[asset.LogicalPath] == workspace.LibraryUnmanaged {
			assets = append(assets, asset)
		}
	}
	return assets
}

// startCompose refuses rather than guesses: no workspace means the UI stays
// read-only, and an empty selection is not an invitation to write nothing.
func (m Model) startCompose() (tea.Model, tea.Cmd) {
	if m.composing {
		return m, nil
	}
	if m.workspace == "" {
		m.notice = m.text(msgComposeWorkspaceRequired)
		m.noticeIsWarn = true
		return m, nil
	}
	assets := m.selectedAddAssets()
	if len(assets) == 0 {
		m.notice = m.text(msgComposeSelectUnmanaged)
		m.noticeIsWarn = true
		return m, nil
	}
	m.composing = true
	m.notice = m.text(msgComposeAdding)
	m.noticeIsWarn = false
	return m, composeCommand(workspace.ComposeOptions{
		WorkspaceRoot: m.workspace,
		Home:          m.options.Home,
		Project:       m.options.Project,
		Assets:        assets,
	}, m.text(msgValidationFailed), m.text(msgValidationDetails))
}

// composeCommand writes the selection into the workspace.
//
// The TUI holds no policy of its own: it hands the selection to
// workspace.Compose and passes validate.Validate straight through as the gate
// that decides whether the staged manifest is allowed to land.
func composeCommand(options workspace.ComposeOptions, validationPrefix, validationFallback string) tea.Cmd {
	return func() tea.Msg {
		result, err := workspace.Compose(options, func(manifestPath, root string) error {
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
		})
		return composeMsg{result: result, err: err}
	}
}

// firstErrorMessage surfaces the actual validation error rather than a count,
// so the user sees what to fix without switching to the CLI.
func firstErrorMessage(report validate.Report, fallback string) string {
	for _, finding := range report.Findings {
		if finding.Severity == workspace.SeverityError {
			return string(finding.Code) + ": " + finding.Message
		}
	}
	return fallback
}

// composeNotice turns a compose result into the one line shown in the footer.
func (m Model) composeNotice(message composeMsg) (string, bool) {
	if message.err != nil {
		return m.text(msgComposeFailed, message.err), true
	}
	result := message.result
	if len(result.Registered) == 0 {
		if len(result.Findings) > 0 {
			return m.text(
				msgComposeNoneFinding,
				result.Findings[0].Message,
				result.Findings[0].Code,
			), true
		}
		return m.text(msgComposeNoneSelected), true
	}
	notice := m.text(
		msgComposeSucceeded,
		result.ManifestPath, len(result.Registered), len(result.Created))
	if skipped := len(result.Skipped); skipped > 0 {
		notice += m.text(msgComposeSkipped, skipped)
	}
	return notice, len(result.Skipped) > 0
}

// composeFindingLines renders skip reasons for the help overlay, so a user who
// sees "skipped 3" can find out why without leaving the UI.
func (m Model) composeFindingLines() []string {
	if len(m.lastFindings) == 0 {
		return nil
	}
	lines := make([]string, 0, len(m.lastFindings))
	for _, finding := range m.lastFindings {
		lines = append(lines, fmt.Sprintf("  %s  %s — %s", finding.Code, finding.Path, finding.Message))
	}
	sort.Strings(lines)
	return lines
}
