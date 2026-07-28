package tui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type status int

const (
	statusLoading status = iota
	statusReady
	statusFailed
)

type rowKind int

const (
	rowSource rowKind = iota
	rowType
	rowAsset
	rowFindingGroup
	rowFinding
)

type treeRow struct {
	kind      rowKind
	key       string
	label     string
	source    inventory.Source
	assetType inventory.AssetType
	asset     *inventory.Asset
	finding   *inventory.Finding
	depth     int
	expanded  bool
	findings  int
}

type scanMsg struct {
	generation int
	report     inventory.Report
	err        error
}

type screen int

const (
	screenInventory screen = iota
	screenDeployment
)

type Model struct {
	options            inventory.Options
	report             inventory.Report
	filterInput        textinput.Model
	filtering          bool
	workspaceInput     textinput.Model
	choosingWorkspace  bool
	preparingWorkspace bool
	profileInput       textinput.Model
	choosingProfile    bool
	availableProfiles  []string
	building           bool
	findingsOnly       bool
	cursor             int
	expanded           map[string]bool
	status             status
	err                error
	width              int
	height             int
	showHelp           bool
	generation         int
	keys               keyMap
	plain              bool

	// workspace is empty until the user names one by flag or in the path
	// prompt. The UI is read-only until then (ADR-0006 §2).
	workspace    string
	selected     map[string]bool
	composing    bool
	notice       string
	noticeIsWarn bool
	lastFindings []workspace.ComposeFinding

	screen        screen
	deployOptions apply.Options
	diffStatus    status
	diffReport    apply.Report
	deployErr     error
	diffCursor    int
	diffExpanded  map[string]bool
	confirming    bool
	confirmInput  textinput.Model
	applying      bool
	applyResult   *apply.Report
}

func NewModel(options inventory.Options) Model {
	input := textinput.New()
	input.Prompt = "/ "
	input.Placeholder = "filter path, type, or finding"
	input.CharLimit = 120
	input.Width = 36
	confirm := textinput.New()
	confirm.Prompt = "> "
	confirm.Placeholder = "apply"
	confirm.CharLimit = 5
	confirm.Width = 12
	workspaceInput := textinput.New()
	workspaceInput.Prompt = "> "
	workspaceInput.Placeholder = "~/ai-assets"
	workspaceInput.CharLimit = 512
	workspaceInput.Width = 64
	profileInput := textinput.New()
	profileInput.Prompt = "> "
	profileInput.Placeholder = "personal"
	profileInput.CharLimit = 120
	profileInput.Width = 36
	return Model{
		options:        options,
		filterInput:    input,
		workspaceInput: workspaceInput,
		profileInput:   profileInput,
		expanded:       make(map[string]bool),
		selected:       make(map[string]bool),
		diffExpanded: map[string]bool{
			"action:create":    true,
			"action:update":    true,
			"action:unchanged": true,
			"action:skipped":   true,
			"findings":         true,
		},
		confirmInput: confirm,
		status:       statusLoading,
		width:        100,
		height:       30,
		generation:   1,
		keys:         defaultKeys(),
	}
}

// WithWorkspace enables Phase B composition against the given workspace root.
// Without it the model can only read.
func (m Model) WithWorkspace(root string) Model {
	m.workspace = root
	return m
}

// WithDeployment enables Phase C for one explicit package. It starts on the
// read-only diff screen; apply still requires typing "apply" in the TUI.
func (m Model) WithDeployment(options apply.Options) Model {
	options.DryRun = false
	m.deployOptions = options
	if options.Package == "" {
		return m
	}
	m.screen = screenDeployment
	m.diffStatus = statusLoading
	return m
}

func (m Model) Init() tea.Cmd {
	if m.deployOptions.Package != "" {
		return tea.Batch(
			scanCommand(m.options, m.generation),
			diffCommand(m.deployOptions),
		)
	}
	return scanCommand(m.options, m.generation)
}

func scanCommand(options inventory.Options, generation int) tea.Cmd {
	return func() tea.Msg {
		report, err := inventory.Scan(options)
		return scanMsg{generation: generation, report: report, err: err}
	}
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.filterInput.Width = max(10, min(48, message.Width-6))
		m.workspaceInput.Width = max(10, min(64, message.Width-6))
		return m, nil
	case scanMsg:
		if message.generation != m.generation {
			return m, nil
		}
		if message.err != nil {
			m.status = statusFailed
			m.err = message.err
			m.cursor = 0
			return m, nil
		}
		m.report = message.report
		m.status = statusReady
		m.err = nil
		m.ensureGroupsExpanded()
		m.pruneSelection()
		m.clampCursor()
		return m, nil
	case composeMsg:
		m.composing = false
		m.notice, m.noticeIsWarn = composeNotice(message)
		m.lastFindings = message.result.Findings
		if message.err == nil && message.result.Ok {
			// Registered assets are now the workspace's; keeping them ticked
			// would invite a second write that can only report duplicates.
			for _, asset := range m.selectedAssets() {
				delete(m.selected, asset.LogicalPath)
			}
		}
		return m, nil
	case workspaceMsg:
		m.preparingWorkspace = false
		if message.err != nil {
			m.notice = "工作区不可用：" + message.err.Error()
			m.noticeIsWarn = true
			return m, nil
		}
		m.workspace = message.root
		m.workspaceInput.SetValue("")
		m.notice = "已打开工作区：" + message.root
		if message.created {
			m.notice = "已创建工作区：" + message.root
		}
		m.noticeIsWarn = false
		return m, nil
	case buildMsg:
		m.building = false
		if message.err != nil {
			m.notice = "构建失败：" + message.err.Error()
			m.noticeIsWarn = true
			return m, nil
		}
		if !message.report.Ok || message.report.Package == nil {
			m.notice = buildFailureNotice(message.report)
			m.noticeIsWarn = true
			return m, nil
		}
		m.deployOptions.Package = message.packagePath
		m.deployOptions.DryRun = false
		m.screen = screenDeployment
		m.diffStatus = statusLoading
		m.deployErr = nil
		m.applyResult = nil
		m.diffCursor = 0
		m.notice = "构建完成，已进入部署 diff"
		m.noticeIsWarn = false
		return m, diffCommand(m.deployOptions)
	case diffMsg:
		if message.err != nil {
			m.diffStatus = statusFailed
			m.deployErr = message.err
			m.diffCursor = 0
			return m, nil
		}
		m.diffReport = message.report
		m.diffStatus = statusReady
		m.deployErr = nil
		m.applyResult = nil
		m.clampDiffCursor()
		return m, nil
	case applyMsg:
		m.applying = false
		m.confirming = false
		m.confirmInput.Blur()
		m.deployErr = message.err
		m.applyResult = &message.report
		if message.err == nil && message.report.Ok {
			m.generation++
			m.status = statusLoading
			return m, scanCommand(m.options, m.generation)
		}
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(message)
	}
	return m, nil
}

// pruneSelection drops ticks whose asset disappeared between scans, so a
// reload cannot leave the selection pointing at something no longer reported.
func (m *Model) pruneSelection() {
	if len(m.selected) == 0 {
		return
	}
	present := make(map[string]bool, len(m.report.Assets))
	for _, asset := range m.report.Assets {
		present[asset.LogicalPath] = true
	}
	for path := range m.selected {
		if !present[path] {
			delete(m.selected, path)
		}
	}
}

func (m Model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applying {
		m.notice = "正在执行 apply，请等待事务完成"
		m.noticeIsWarn = true
		return m, nil
	}
	if key.Matches(message, m.keys.ForceQuit) {
		return m, tea.Quit
	}
	if m.preparingWorkspace || m.building {
		return m, nil
	}
	if m.choosingWorkspace {
		return m.updateWorkspaceInput(message)
	}
	if m.choosingProfile {
		return m.updateProfileInput(message)
	}
	if m.confirming {
		return m.updateConfirmation(message)
	}
	if m.showHelp {
		if key.Matches(message, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(message, m.keys.Help) || key.Matches(message, m.keys.Collapse) {
			m.showHelp = false
		}
		return m, nil
	}
	if m.filtering {
		if key.Matches(message, m.keys.Collapse) {
			m.filtering = false
			m.filterInput.Blur()
			m.clampCursor()
			return m, nil
		}
		var command tea.Cmd
		previous := m.filterInput.Value()
		m.filterInput, command = m.filterInput.Update(message)
		if m.filterInput.Value() != previous {
			m.cursor = 0
			m.clampCursor()
		}
		return m, command
	}
	if m.screen == screenDeployment {
		return m.updateDeploymentKey(message)
	}

	rows := m.visibleRows()
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.Filter):
		m.filtering = true
		return m, m.filterInput.Focus()
	case key.Matches(message, m.keys.FindingsOnly):
		m.findingsOnly = !m.findingsOnly
		m.cursor = 0
		m.clampCursor()
	case key.Matches(message, m.keys.Reload):
		m.generation++
		m.status = statusLoading
		m.err = nil
		return m, scanCommand(m.options, m.generation)
	case key.Matches(message, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(message, m.keys.Down):
		if m.cursor+1 < len(rows) {
			m.cursor++
		}
	case key.Matches(message, m.keys.First):
		m.cursor = 0
	case key.Matches(message, m.keys.Last):
		if len(rows) > 0 {
			m.cursor = len(rows) - 1
		}
	case key.Matches(message, m.keys.Select):
		m.toggleSelection(rows)
	case key.Matches(message, m.keys.Write):
		if m.workspace == "" {
			return m.startWorkspaceInput()
		}
		return m.startCompose()
	case key.Matches(message, m.keys.Build):
		return m.startProfileInput()
	case key.Matches(message, m.keys.Diff):
		return m.startDiff()
	case key.Matches(message, m.keys.Expand):
		m.setCurrentExpanded(rows, true)
	case key.Matches(message, m.keys.Collapse):
		m.setCurrentExpanded(rows, false)
	}
	return m, nil
}

func (m *Model) toggleSelection(rows []treeRow) {
	if m.cursor < 0 || m.cursor >= len(rows) {
		return
	}
	row := rows[m.cursor]
	if !m.selectableAsset(row) {
		m.notice = "只有候选资产可以勾选"
		m.noticeIsWarn = true
		return
	}
	m.notice = ""
	m.noticeIsWarn = false
	if m.selected[row.asset.LogicalPath] {
		delete(m.selected, row.asset.LogicalPath)
		return
	}
	m.selected[row.asset.LogicalPath] = true
}

// startCompose refuses rather than guesses: no workspace means the UI stays
// read-only, and an empty selection is not an invitation to write nothing.
func (m Model) startCompose() (tea.Model, tea.Cmd) {
	if m.composing {
		return m, nil
	}
	if m.workspace == "" {
		m.notice = "未指定工作区；用 aiah ui --workspace PATH 启动才能写出"
		m.noticeIsWarn = true
		return m, nil
	}
	assets := m.selectedAssets()
	if len(assets) == 0 {
		m.notice = "先用空格勾选要登记的资产"
		m.noticeIsWarn = true
		return m, nil
	}
	m.composing = true
	m.notice = "正在写出工作区…"
	m.noticeIsWarn = false
	return m, composeCommand(workspace.ComposeOptions{
		WorkspaceRoot: m.workspace,
		Home:          m.options.Home,
		Project:       m.options.Project,
		Assets:        assets,
	})
}

func (m *Model) setCurrentExpanded(rows []treeRow, expanded bool) {
	if m.cursor < 0 || m.cursor >= len(rows) {
		return
	}
	row := rows[m.cursor]
	if row.kind == rowAsset || row.kind == rowFinding {
		return
	}
	m.expanded[row.key] = expanded
	m.clampCursor()
}

func (m *Model) ensureGroupsExpanded() {
	for _, asset := range m.report.Assets {
		sourceKey := sourceGroupKey(asset.Source)
		typeKey := typeGroupKey(asset.Source, asset.Type)
		if _, ok := m.expanded[sourceKey]; !ok {
			m.expanded[sourceKey] = true
		}
		if _, ok := m.expanded[typeKey]; !ok {
			m.expanded[typeKey] = true
		}
	}
	for _, finding := range m.report.Findings {
		if !m.findingAttached(finding) {
			if _, ok := m.expanded[unattachedFindingsGroupKey]; !ok {
				m.expanded[unattachedFindingsGroupKey] = true
			}
			break
		}
	}
}

func (m *Model) clampCursor() {
	rowCount := len(m.visibleRows())
	if rowCount == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= rowCount {
		m.cursor = rowCount - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) visibleRows() []treeRow {
	type typeGroup struct {
		name   inventory.AssetType
		assets []inventory.Asset
	}
	type sourceGroup struct {
		types map[inventory.AssetType][]inventory.Asset
	}

	groups := make(map[inventory.Source]*sourceGroup)
	for index := range m.report.Assets {
		asset := m.report.Assets[index]
		if !m.assetMatches(asset) {
			continue
		}
		group := groups[asset.Source]
		if group == nil {
			group = &sourceGroup{types: make(map[inventory.AssetType][]inventory.Asset)}
			groups[asset.Source] = group
		}
		group.types[asset.Type] = append(group.types[asset.Type], asset)
	}

	sources := make([]inventory.Source, 0, len(groups))
	for source := range groups {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i] < sources[j] })

	rows := make([]treeRow, 0)
	for _, source := range sources {
		group := groups[source]
		sourceKey := sourceGroupKey(source)
		sourceExpanded := m.expanded[sourceKey]
		sourceFindings := 0
		for _, assets := range group.types {
			for _, asset := range assets {
				sourceFindings += len(m.findingsFor(asset))
			}
		}
		rows = append(rows, treeRow{
			kind: rowSource, key: sourceKey, label: string(source), source: source,
			expanded: sourceExpanded, findings: sourceFindings,
		})
		if !sourceExpanded {
			continue
		}

		types := make([]typeGroup, 0, len(group.types))
		for name, assets := range group.types {
			sort.Slice(assets, func(i, j int) bool {
				return assets[i].LogicalPath < assets[j].LogicalPath
			})
			types = append(types, typeGroup{name: name, assets: assets})
		}
		sort.Slice(types, func(i, j int) bool { return types[i].name < types[j].name })
		for _, typeGroup := range types {
			typeKey := typeGroupKey(source, typeGroup.name)
			typeExpanded := m.expanded[typeKey]
			typeFindings := 0
			for _, asset := range typeGroup.assets {
				typeFindings += len(m.findingsFor(asset))
			}
			rows = append(rows, treeRow{
				kind: rowType, key: typeKey, label: string(typeGroup.name), source: source,
				assetType: typeGroup.name, depth: 1, expanded: typeExpanded, findings: typeFindings,
			})
			if !typeExpanded {
				continue
			}
			for index := range typeGroup.assets {
				asset := &typeGroup.assets[index]
				rows = append(rows, treeRow{
					kind: rowAsset, key: asset.LogicalPath, label: asset.LogicalPath,
					source: source, assetType: typeGroup.name, asset: asset, depth: 2,
					findings: len(m.findingsFor(*asset)),
				})
			}
		}
	}

	unattached := m.unattachedFindingIndexes()
	if len(unattached) > 0 {
		expanded := m.expanded[unattachedFindingsGroupKey]
		rows = append(rows, treeRow{
			kind: rowFindingGroup, key: unattachedFindingsGroupKey,
			label: "未关联 findings", expanded: expanded, findings: len(unattached),
		})
		if expanded {
			for _, index := range unattached {
				finding := &m.report.Findings[index]
				label := string(finding.Code)
				if len(finding.Paths) > 0 {
					label += " · " + finding.Paths[0]
				}
				rows = append(rows, treeRow{
					kind: rowFinding, key: findingRowKey(index, *finding), label: label,
					finding: finding, depth: 1, findings: 1,
				})
			}
		}
	}
	return rows
}

func (m Model) assetMatches(asset inventory.Asset) bool {
	findings := m.findingsFor(asset)
	if m.findingsOnly && len(findings) == 0 {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(asset.LogicalPath), query) ||
		strings.Contains(strings.ToLower(string(asset.Type)), query) {
		return true
	}
	for _, file := range asset.Files {
		if strings.Contains(strings.ToLower(file), query) {
			return true
		}
	}
	return false
}

func (m Model) findingsFor(asset inventory.Asset) []inventory.Finding {
	findings := make([]inventory.Finding, 0)
	for _, finding := range m.report.Findings {
		if findingAppliesToAsset(finding, asset) {
			findings = append(findings, finding)
		}
	}
	return findings
}

func (m Model) unattachedFindingIndexes() []int {
	indexes := make([]int, 0)
	for findingIndex := range m.report.Findings {
		finding := m.report.Findings[findingIndex]
		if !m.findingAttached(finding) && m.findingMatches(finding) {
			indexes = append(indexes, findingIndex)
		}
	}
	return indexes
}

func (m Model) findingAttached(finding inventory.Finding) bool {
	for _, asset := range m.report.Assets {
		if findingAppliesToAsset(finding, asset) {
			return true
		}
	}
	return false
}

func (m Model) findingMatches(finding inventory.Finding) bool {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(string(finding.Code)), query) ||
		strings.Contains(strings.ToLower(string(finding.Severity)), query) ||
		strings.Contains(strings.ToLower(finding.Message), query) {
		return true
	}
	for _, path := range finding.Paths {
		if strings.Contains(strings.ToLower(path), query) {
			return true
		}
	}
	return false
}

func findingAppliesToAsset(finding inventory.Finding, asset inventory.Asset) bool {
	for _, path := range finding.Paths {
		if path == asset.LogicalPath {
			return true
		}
		for _, file := range asset.Files {
			if path == file {
				return true
			}
		}
	}
	return false
}

func sourceGroupKey(source inventory.Source) string {
	return "source:" + string(source)
}

func typeGroupKey(source inventory.Source, assetType inventory.AssetType) string {
	return "type:" + string(source) + "/" + string(assetType)
}

const unattachedFindingsGroupKey = "findings:unattached"

func findingRowKey(index int, finding inventory.Finding) string {
	return unattachedFindingsGroupKey + "/" + strconv.Itoa(index) + "/" + string(finding.Code)
}
