package tui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type rowKind int

const (
	rowSource rowKind = iota
	rowType
	rowAsset
	rowFindingGroup
	rowFinding
	rowLibraryGroup
	rowLibraryAsset
)

type treeRow struct {
	kind      rowKind
	key       string
	label     string
	source    inventory.Source
	assetType inventory.AssetType
	asset     *inventory.Asset
	finding   *inventory.Finding
	library   *workspace.CatalogItem
	depth     int
	expanded  bool
	findings  int
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
	for _, item := range m.catalog.Items {
		if item.State == workspace.LibraryOnly {
			present["library:"+item.ID] = true
		}
	}
	for path := range m.selected {
		if !present[path] {
			delete(m.selected, path)
		}
	}
}

func (m *Model) toggleSelection(rows []treeRow) {
	if m.cursor < 0 || m.cursor >= len(rows) {
		return
	}
	row := rows[m.cursor]
	if !m.selectableAsset(row) {
		m.notice = m.text(msgInventoryNotSelectable)
		m.noticeIsWarn = true
		return
	}
	m.notice = ""
	m.noticeIsWarn = false
	if m.selected[row.key] {
		delete(m.selected, row.key)
		return
	}
	m.selected[row.key] = true
}

func (m *Model) setCurrentExpanded(rows []treeRow, expanded bool) {
	if m.cursor < 0 || m.cursor >= len(rows) {
		return
	}
	row := rows[m.cursor]
	if row.kind == rowAsset || row.kind == rowLibraryAsset || row.kind == rowFinding {
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
	catalogByPath := make(map[string]*workspace.CatalogItem, len(m.catalog.Items))
	libraryOnly := make([]workspace.CatalogItem, 0)
	for index := range m.catalog.Items {
		item := &m.catalog.Items[index]
		if item.LogicalPath != "" {
			catalogByPath[item.LogicalPath] = item
		}
		if item.State == workspace.LibraryOnly {
			libraryOnly = append(libraryOnly, *item)
		}
	}
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
					findings: len(m.findingsFor(*asset)), library: catalogByPath[asset.LogicalPath],
				})
			}
		}
	}

	if len(libraryOnly) > 0 {
		const groupKey = "library:only"
		expanded, exists := m.expanded[groupKey]
		if !exists {
			expanded = true
		}
		rows = append(rows, treeRow{
			kind: rowLibraryGroup, key: groupKey, label: m.text(msgInventoryLibraryOnlyGroup),
			expanded: expanded,
		})
		if expanded {
			sort.Slice(libraryOnly, func(i, j int) bool { return libraryOnly[i].ID < libraryOnly[j].ID })
			for index := range libraryOnly {
				item := &libraryOnly[index]
				rows = append(rows, treeRow{
					kind: rowLibraryAsset, key: "library:" + item.ID,
					label: item.ID, library: item, depth: 1,
				})
			}
		}
	}

	unattached := m.unattachedFindingIndexes()
	if len(unattached) > 0 {
		expanded := m.expanded[unattachedFindingsGroupKey]
		rows = append(rows, treeRow{
			kind: rowFindingGroup, key: unattachedFindingsGroupKey,
			label: m.text(msgInventoryUnattachedGroup), expanded: expanded, findings: len(unattached),
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
