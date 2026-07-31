package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestSelectionRequiresAWorkspace(t *testing.T) {
	// Without --workspace the UI is read-only, so it must not offer checkboxes
	// it cannot honour.
	model := composeModel(t, "")
	rows := model.visibleRows()
	assetRow := firstAssetRow(t, rows)
	if model.selectableAsset(assetRow) {
		t.Fatal("assets are selectable without a workspace")
	}
	if strings.Contains(model.View(), "[ ]") {
		t.Fatal("read-only view rendered checkboxes")
	}

	withWorkspace := composeModel(t, t.TempDir())
	if !withWorkspace.selectableAsset(firstAssetRow(t, withWorkspace.visibleRows())) {
		t.Fatal("assets are not selectable with a workspace")
	}
	if !strings.Contains(withWorkspace.View(), "[ ]") {
		t.Fatal("compose view did not render checkboxes")
	}
}

func TestWriteWithoutWorkspaceOpensExplicitPathPrompt(t *testing.T) {
	model := composeModel(t, "")
	updated, command := model.Update(keyPress("w"))
	next := updated.(Model)
	if command == nil || !next.choosingWorkspace || next.workspace != "" {
		t.Fatalf("workspace prompt state = choosing %v workspace %q command nil=%v",
			next.choosingWorkspace, next.workspace, command == nil)
	}
	if _, err := os.Stat(filepath.Join(next.options.Home, "ai-assets")); !os.IsNotExist(err) {
		t.Fatalf("opening the prompt wrote a workspace: %v", err)
	}
}

func TestWriteWithoutSelectionRefuses(t *testing.T) {
	model := composeModel(t, t.TempDir())
	updated, command := model.Update(keyPress("w"))
	if command != nil {
		t.Fatal("a write was attempted with nothing selected")
	}
	if next := updated.(Model); !next.noticeIsWarn || next.notice == "" {
		t.Fatalf("empty selection was not reported: %q", next.notice)
	}
}

func TestSpaceTogglesSelection(t *testing.T) {
	model := composeModel(t, t.TempDir())
	model.cursor = assetRowIndex(t, model.visibleRows())

	updated, _ := model.Update(keyPress(" "))
	selected := updated.(Model)
	if len(selected.selected) != 1 {
		t.Fatalf("space did not select: %#v", selected.selected)
	}
	if !strings.Contains(selected.View(), "[x]") {
		t.Fatal("selected asset is not marked in the view")
	}

	updated, _ = selected.Update(keyPress(" "))
	if cleared := updated.(Model); len(cleared.selected) != 0 {
		t.Fatalf("space did not deselect: %#v", cleared.selected)
	}
}

func TestSelectionSurvivesOnlyForStillReportedAssets(t *testing.T) {
	model := composeModel(t, t.TempDir())
	model.selected = map[string]bool{
		"home/.claude/skills/review": true,
		"home/.claude/skills/gone":   true,
	}
	model.pruneSelection()
	if model.selected["home/.claude/skills/gone"] {
		t.Fatal("a selection survived its asset disappearing from the report")
	}
	if !model.selected["home/.claude/skills/review"] {
		t.Fatal("a still-reported selection was dropped")
	}
}

func TestComposeNoticeReportsRollbackOnFailure(t *testing.T) {
	notice, warn := NewModel(inventory.Options{}).
		composeNotice(composeMsg{err: workspace.ErrComposeBlocked})
	if !warn || !strings.Contains(notice, "回滚") {
		t.Fatalf("notice = %q warn=%v; a failed write must say the workspace was rolled back", notice, warn)
	}
}

func TestEnglishAssetLibraryViewsAndNotices(t *testing.T) {
	model := readyTestModel().withLanguage(languageEnglish)

	model.choosingWorkspace = true
	if view := model.View(); !strings.Contains(view, "Choose an asset library") ||
		!strings.Contains(view, "Enter Confirm") {
		t.Fatalf("English workspace prompt is incomplete:\n%s", view)
	}

	model.choosingWorkspace = false
	model.confirmManage = true
	model.manageAction = manageRemove
	if view := model.View(); !strings.Contains(view, "Confirm asset library operation") ||
		!strings.Contains(view, "Type remove and press Enter") ||
		!strings.Contains(view, "Source files are not deleted") {
		t.Fatalf("English remove confirmation is incomplete:\n%s", view)
	}

	model.confirmManage = false
	model.workspace = ""
	updated, command := model.startCompose()
	next := updated.(Model)
	if command != nil || !next.noticeIsWarn ||
		!strings.Contains(next.notice, "No asset library selected") {
		t.Fatalf(
			"English compose guard = notice %q warn=%v command nil=%v",
			next.notice,
			next.noticeIsWarn,
			command == nil,
		)
	}

	notice, warn := model.composeNotice(composeMsg{err: workspace.ErrComposeBlocked})
	if !warn || !strings.Contains(notice, "Could not add assets") ||
		!strings.Contains(notice, "no partial result remains") {
		t.Fatalf("English compose failure = %q warn=%v", notice, warn)
	}
	notice, warn = model.manageNotice(manageMsg{action: manageUpdate, ok: true, count: 2})
	if warn || !strings.Contains(notice, "Updated 2 assets") {
		t.Fatalf("English update result = %q warn=%v", notice, warn)
	}

	model.choosingProfile = true
	model.workspace = "/tmp/assets"
	model.availableProfiles = []string{"personal", "work"}
	if view := model.View(); !strings.Contains(view, "Preview and apply library") ||
		!strings.Contains(view, "Available profiles: personal, work") {
		t.Fatalf("English profile prompt is incomplete:\n%s", view)
	}
}

func TestComposeSuccessClearsSelection(t *testing.T) {
	model := composeModel(t, t.TempDir())
	model.selected = map[string]bool{"home/.claude/skills/review": true}
	model.composing = true
	model.deployOptions.Package = "/tmp/old-generated.tar"
	model.packageFromBuild = true

	updated, _ := model.Update(composeMsg{result: workspace.ComposeResult{
		Ok: true, Registered: []string{"skill.review"}, ManifestPath: "/w/manifest.yaml",
	}})
	next := updated.(Model)
	if len(next.selected) != 0 {
		t.Fatalf("selection survived a successful write: %#v", next.selected)
	}
	if next.composing {
		t.Fatal("composing flag was not cleared")
	}
	if next.deployOptions.Package != "" || next.packageFromBuild {
		t.Fatalf("successful compose retained a stale generated package: %#v", next.deployOptions)
	}
}

func TestComposeKeepsAnExplicitDeploymentPackage(t *testing.T) {
	model := composeModel(t, t.TempDir())
	model.deployOptions.Package = "/tmp/explicit.tar"

	updated, _ := model.Update(composeMsg{result: workspace.ComposeResult{
		Ok: true, Registered: []string{"skill.review"}, ManifestPath: "/w/manifest.yaml",
	}})
	next := updated.(Model)
	if next.deployOptions.Package != "/tmp/explicit.tar" {
		t.Fatalf("compose discarded an unrelated explicit package: %#v", next.deployOptions)
	}
}

func TestComposeFailureKeepsSelection(t *testing.T) {
	// Nothing landed, so the user must not have to tick everything again.
	model := composeModel(t, t.TempDir())
	model.selected = map[string]bool{"home/.claude/skills/review": true}
	model.composing = true

	updated, _ := model.Update(composeMsg{err: workspace.ErrComposeBlocked})
	if next := updated.(Model); len(next.selected) != 1 {
		t.Fatalf("a failed write discarded the selection: %#v", next.selected)
	}
}

func TestHelpStatesTheWriteBoundary(t *testing.T) {
	readOnly := composeModel(t, "").helpView(newStyles(true))
	for _, needle := range []string{"--workspace", "可编辑事实源", "目标工具"} {
		if !strings.Contains(readOnly, needle) {
			t.Fatalf("read-only help omits %q:\n%s", needle, readOnly)
		}
	}
	composing := composeModel(t, "/tmp/ws").helpView(newStyles(true))
	for _, needle := range []string{".claude", "w 纳入", "u 更新", "X 移出", "b 预览", "a 应用", "目标工具", "apply", "安装恢复点"} {
		if !strings.Contains(composing, needle) {
			t.Fatalf("compose help omits %q:\n%s", needle, composing)
		}
	}
}

func TestInventoryAlwaysShowsWorkspaceAndGuidedStages(t *testing.T) {
	readOnly := composeModel(t, "").View()
	for _, needle := range []string{"资产库  未选择", "按 w 选择或创建"} {
		if !strings.Contains(readOnly, needle) {
			t.Fatalf("read-only inventory omits %q:\n%s", needle, readOnly)
		}
	}

	composing := composeModel(t, "/tmp/ws").View()
	for _, needle := range []string{"/tmp/ws", "目标工具", "w 纳入", "u 更新", "X 移出", "b 预览并应用"} {
		if !strings.Contains(composing, needle) {
			t.Fatalf("workspace inventory omits %q:\n%s", needle, composing)
		}
	}
}

func TestWorkspacePromptExplainsRoleTargetsAndStages(t *testing.T) {
	model := composeModel(t, "")
	updated, _ := model.Update(keyPress("w"))
	view := updated.(Model).View()
	for _, needle := range []string{"可编辑事实源", "目标工具", "选择资产", "确认应用"} {
		if !strings.Contains(view, needle) {
			t.Fatalf("workspace prompt omits %q:\n%s", needle, view)
		}
	}
}

// --- helpers ---

func composeModel(t *testing.T, workspaceRoot string) Model {
	t.Helper()
	home := t.TempDir()
	skill := filepath.Join(home, ".claude", "skills", "review")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# review\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := NewModel(inventory.Options{Home: home}).
		WithWorkspace(workspaceRoot).
		WithDeployment(apply.Options{Home: home})
	model.plain = true
	report, err := inventory.Scan(inventory.Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(scanMsg{generation: model.generation, report: report})
	return updated.(Model)
}

func firstAssetRow(t *testing.T, rows []treeRow) treeRow {
	t.Helper()
	return rows[assetRowIndex(t, rows)]
}

func assetRowIndex(t *testing.T, rows []treeRow) int {
	t.Helper()
	for index, row := range rows {
		if row.kind == rowAsset && row.asset != nil &&
			row.asset.Status == inventory.AssetCandidate {
			return index
		}
	}
	t.Fatal("fixture produced no candidate asset row")
	return 0
}

func keyPress(value string) tea.KeyMsg {
	if value == " " {
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
