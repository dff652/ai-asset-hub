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
	if m.screen == screenSettings {
		return m.settingsView(style)
	}

	header := style.header.Render(m.text(msgInventoryTitle))
	counts := m.text(msgInventoryCountsScanned,
		m.report.Summary.CandidateAssets, m.report.Summary.ExcludedAssets, len(m.report.Findings))
	if m.workspace != "" && m.catalogErr == nil {
		counts = m.text(
			msgInventoryCountsManaged,
			m.catalog.Summary.Unmanaged, m.catalog.Summary.Managed,
			m.catalog.Summary.SourceChanged, m.catalog.Summary.LibraryOnly,
		)
	}
	header = joinEdges(header, counts, max(20, m.width))

	workspaceLine := m.text(msgInventoryWorkspaceUnset)
	if m.workspace != "" {
		workspaceLine = m.text(msgInventoryWorkspaceSelected, m.workspace)
	}
	workspaceLine = style.muted.Render(workspaceLine)

	filterLine := ""
	if m.filtering || m.filterInput.Value() != "" {
		filterLine = m.filterInput.View()
	} else {
		filterLine = style.muted.Render(m.text(msgInventoryFilterHint))
	}
	if m.findingsOnly {
		filterLine += style.warning.Render(m.text(msgInventoryFindingsOnly))
	}

	var body string
	switch m.status {
	case statusLoading:
		body = m.text(msgInventoryScanning)
	case statusFailed:
		message := m.text(msgInventoryScanFailed)
		if m.err != nil {
			message += ": " + m.err.Error()
		}
		body = "\n" + style.error.Render(message) + "\n" + m.text(msgInventoryScanRetry)
	default:
		body = m.inventoryBody(style)
	}

	keysHint := m.text(msgInventoryFooterReadOnly)
	if m.workspace != "" {
		keysHint = m.text(msgInventoryFooterManaged)
	}
	if m.deployOptions.Package != "" {
		keysHint += m.text(msgInventoryFooterDiff)
	}
	if m.maintenance {
		keysHint += m.text(msgInventoryFooterMaintenance)
	}
	footer := style.muted.Render(keysHint)
	if selected := len(m.selected); selected > 0 {
		footer = style.header.Render(m.text(msgInventorySelected, selected)) + "  " + footer
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
		return m.text(msgInventoryEmpty)
	}
	if m.width < 80 {
		return m.narrowInventoryBody(rows, style)
	}

	bodyHeight := max(4, m.height-6)
	start, end := visibleRange(len(rows), m.cursor, bodyHeight)
	leftWidth := max(28, min(52, m.width*45/100))
	rightWidth := max(24, m.width-leftWidth-3)
	detailLines := m.detailLines(rows, 1<<20, bodyHeight, style)
	if !linesFitWidth(detailLines, rightWidth) {
		return m.narrowInventoryBody(rows, style)
	}

	leftLines := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		leftLines = append(leftLines, m.renderTreeRow(rows[index], index == m.cursor, leftWidth, style))
	}
	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, "")
	}

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

func (m Model) narrowInventoryBody(rows []treeRow, style styles) string {
	listHeight := max(3, min(6, m.height/3))
	start, end := visibleRange(len(rows), m.cursor, listHeight)
	lines := make([]string, 0, listHeight+10)
	for index := start; index < end; index++ {
		lines = append(lines, m.renderTreeRow(
			rows[index],
			index == m.cursor,
			max(20, m.width),
			style,
		))
	}
	lines = append(lines, style.border.Render(strings.Repeat("─", max(20, m.width))))
	detailHeight := max(5, m.height-listHeight-7)
	lines = append(lines, m.detailLines(rows, 1<<20, detailHeight, style)...)
	return strings.Join(lines, "\n")
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
			m.text(msgInventoryDetailSeverity, finding.Severity),
			m.text(msgInventoryDetailDescription, finding.Message),
			m.text(msgInventoryDetailPaths, len(finding.Paths)),
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
		kind := m.text(msgInventoryGroupSource)
		if row.kind == rowType {
			kind = m.text(msgInventoryGroupType)
		} else if row.kind == rowFindingGroup {
			kind = m.text(msgInventoryGroupFindings)
		} else if row.kind == rowLibraryGroup {
			kind = m.text(msgInventoryGroupLibraryStatus)
		}
		return []string{
			style.header.Render(row.label),
			m.text(msgInventoryDetailGroup, kind),
			m.text(msgInventoryDetailFindings, row.findings),
			"",
			m.text(msgInventoryDetailExpand),
		}
	}

	if row.library != nil && row.asset == nil {
		return []string{
			style.header.Render(row.library.ID),
			m.text(msgInventoryDetailAssetState, m.libraryStateLabel(row.library.State)),
			m.text(msgInventoryDetailType, row.library.Type),
			m.text(msgInventoryDetailLibraryPath, row.library.LibraryPath),
			m.text(
				msgInventoryDetailTargets,
				strings.Join(row.library.Targets, m.text(msgCommonListSeparator)),
			),
			"",
			m.text(msgInventoryDetailRemove),
		}
	}

	asset := *row.asset
	lines := []string{
		style.header.Render(asset.LogicalPath),
	}
	if row.library != nil {
		lines = append(
			lines,
			m.text(msgInventoryDetailAssetState, m.libraryStateLabel(row.libraryState())),
		)
	}
	lines = append(lines,
		m.text(msgInventoryDetailType, asset.Type),
		m.text(msgInventoryDetailSource, asset.Source),
		m.text(msgInventoryDetailScope, asset.Scope),
		m.text(msgInventoryDetailPortability, asset.Portability),
		m.text(msgInventoryDetailSensitivity, asset.Sensitivity),
		m.text(msgInventoryDetailStatus, asset.Status),
		m.text(msgInventoryDetailFiles, len(asset.Files)),
	)
	for _, file := range asset.Files {
		lines = append(lines, "  "+file)
		if len(lines) >= height-2 {
			break
		}
	}

	findings := m.findingsFor(asset)
	if len(findings) == 0 {
		lines = append(lines, m.text(msgInventoryDetailNoFindings))
	} else {
		lines = append(lines, m.text(msgInventoryDetailFindings, len(findings)))
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

func (m Model) libraryStateLabel(state workspace.LibraryState) string {
	switch state {
	case workspace.LibraryUnmanaged:
		return m.text(msgInventoryStateUnmanaged)
	case workspace.LibraryManaged:
		return m.text(msgInventoryStateManaged)
	case workspace.LibrarySourceChanged:
		return m.text(msgInventoryStateSourceChanged)
	case workspace.LibraryOnly:
		return m.text(msgInventoryStateLibraryOnly)
	default:
		return m.text(msgInventoryStateBlocked)
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
	if m.screen == screenSettings {
		return m.settingsHelpView(style)
	}
	lines := []string{
		style.header.Render(m.text(msgInventoryHelpTitle)),
		"",
		m.text(msgHelpMove),
		m.text(msgHelpFirstLast),
		m.text(msgHelpExpand),
		m.text(msgHelpCollapse),
		m.text(msgHelpFilter),
		m.text(msgHelpFindingsOnly),
		m.text(msgHelpRescan),
		m.text(msgHelpHome),
		m.text(msgHelpClose),
		m.text(msgHelpQuit),
	}
	if m.workspace == "" {
		lines = append(lines,
			"",
			m.text(msgInventoryHelpLibrary),
			m.text(msgInventoryHelpTargets),
			m.text(msgInventoryHelpReadOnly),
			m.text(msgInventoryHelpNoDefault),
		)
	} else {
		lines = append(lines,
			m.text(msgInventoryHelpSelect),
			m.text(msgInventoryHelpAdd),
			m.text(msgInventoryHelpUpdate),
			m.text(msgInventoryHelpRemove),
			m.text(msgInventoryHelpPreview),
			m.text(msgInventoryHelpApply),
			"",
			m.text(msgInventoryHelpWorkspace, m.workspace),
			m.text(msgInventoryHelpManagedTargets),
			m.text(msgInventoryHelpLibraryWrites),
			m.text(msgInventoryHelpWizard),
			m.text(msgInventoryHelpBackups),
		)
		if findings := m.composeFindingLines(); len(findings) > 0 {
			lines = append(lines, "", m.text(msgInventoryHelpSkipped))
			lines = append(lines, findings...)
		}
	}
	if m.deployOptions.Package != "" {
		lines = append(lines,
			m.text(msgInventoryHelpDiff),
			m.text(msgInventoryHelpApplySafety),
		)
	} else if m.workspace == "" {
		lines = append(lines, m.text(msgInventoryHelpNoApply))
	}
	if m.maintenance {
		lines = append(lines,
			m.text(msgInventoryHelpDoctor),
			m.text(msgInventoryHelpVersion),
			m.text(msgInventoryHelpRollback),
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) workspaceInputView(style styles) string {
	lines := []string{
		style.header.Render(m.text(msgWorkspaceInputTitle)),
		"",
		m.text(msgWorkspaceInputDefinition),
		m.text(msgWorkspaceInputContents),
		m.text(msgWorkspaceInputTargets),
		"",
		m.text(msgWorkspaceInputPrompt),
		m.workspaceInput.View(),
		"",
		m.text(msgWorkspaceInputExplicit),
		m.text(msgWorkspaceInputFlow),
		m.text(msgWorkspaceInputFooter),
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) profileInputView(style styles) string {
	title := m.text(msgProfileInputApplyTitle)
	next := m.text(msgProfileInputApplyNext)
	if m.profilePurpose == profileForPublish {
		title = m.text(msgProfileInputPublishTitle)
		next = m.text(msgProfileInputPublishNext)
	} else if m.profilePurpose == profileForPreflight {
		title = m.text(msgProfileInputPreflightTitle)
		next = m.text(msgProfileInputPreflightNext)
	}
	lines := []string{
		style.header.Render(title),
		"",
		m.text(msgProfileInputLibrary, m.workspace),
		m.text(msgProfileInputPrompt),
		m.profileInput.View(),
		"",
		next,
		m.text(msgProfileInputFooter),
	}
	if len(m.availableProfiles) > 0 {
		lines = append(
			lines,
			m.text(
				msgProfileInputAvailable,
				strings.Join(m.availableProfiles, m.text(msgCommonListSeparator)),
			),
		)
	}
	if m.notice != "" {
		lines = append(lines, "", style.warning.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) deploymentHelpView(style styles) string {
	lines := []string{
		style.header.Render(m.text(msgDeploymentHelpTitle)),
		"",
		m.text(msgHelpMove),
		m.text(msgHelpFirstLast),
		m.text(msgHelpExpand),
		m.text(msgHelpCollapseOnly),
		m.text(msgDeploymentHelpApply),
		m.text(msgDeploymentHelpRefresh),
		m.text(msgHelpHome),
		m.text(msgHelpClose),
		m.text(msgHelpQuit),
		"",
		m.text(msgDeploymentHelpSafety),
		m.text(msgDeploymentHelpResult),
	}
	if m.maintenance {
		lines = append(lines,
			m.text(msgDeploymentHelpDoctor),
			m.text(msgDeploymentHelpVersion),
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) healthHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render(m.text(msgHealthHelpTitle)),
		"",
		m.text(msgHelpMove),
		m.text(msgHelpFirstLast),
		m.text(msgHealthHelpRerun),
		m.text(msgHealthHelpRollback),
		m.text(msgHealthHelpVersion),
		m.text(msgHelpHome),
		m.text(msgHelpClose),
		m.text(msgHelpQuit),
		"",
		m.text(msgHealthHelpAvailability),
		m.text(msgHealthHelpTyped),
	}, "\n")
}

func (m Model) versionHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render(m.text(msgVersionHelpTitle)),
		"",
		m.text(msgVersionHelpCheck),
		m.text(msgVersionHelpHealth),
		m.text(msgHelpHome),
		m.text(msgHelpClose),
		m.text(msgHelpQuit),
		"",
		m.text(msgVersionHelpOffline),
		m.text(msgVersionHelpOnline),
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

func linesFitWidth(lines []string, width int) bool {
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			return false
		}
	}
	return true
}

func joinEdges(left, right string, width int) string {
	padding := width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		return truncate(left+" "+right, width)
	}
	return left + strings.Repeat(" ", padding) + right
}
