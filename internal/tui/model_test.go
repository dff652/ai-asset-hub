package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dff652/ai-asset-hub/internal/inventory"
)

func TestUpdateNavigationAndFiltering(t *testing.T) {
	t.Run("cursor stays within boundaries", func(t *testing.T) {
		model := readyTestModel()
		model = pressKey(model, "up")
		if model.cursor != 0 {
			t.Fatalf("cursor after up at start = %d, want 0", model.cursor)
		}
		model = pressKey(model, "G")
		last := len(model.visibleRows()) - 1
		if model.cursor != last {
			t.Fatalf("cursor after G = %d, want %d", model.cursor, last)
		}
		model = pressKey(model, "down")
		if model.cursor != last {
			t.Fatalf("cursor after down at end = %d, want %d", model.cursor, last)
		}
	})

	t.Run("empty filter result resets cursor without changing report", func(t *testing.T) {
		model := readyTestModel()
		model = pressKey(model, "G")
		original := model.report
		model = pressKey(model, "/")
		for _, character := range "does-not-exist" {
			model = pressKey(model, string(character))
		}
		if rows := model.visibleRows(); len(rows) != 0 {
			t.Fatalf("visible rows = %d, want 0", len(rows))
		}
		if model.cursor != 0 {
			t.Fatalf("cursor = %d, want 0", model.cursor)
		}
		if !reflect.DeepEqual(model.report, original) {
			t.Fatal("filter mutated the inventory report")
		}
	})

	t.Run("filter matches asset path", func(t *testing.T) {
		model := pressKey(readyTestModel(), "/")
		for _, character := range "review" {
			model = pressKey(model, string(character))
		}
		var assets []string
		for _, row := range model.visibleRows() {
			if row.asset != nil {
				assets = append(assets, row.asset.LogicalPath)
			}
		}
		want := []string{"home/.codex/skills/review"}
		if !reflect.DeepEqual(assets, want) {
			t.Fatalf("assets = %#v, want %#v", assets, want)
		}
	})

	t.Run("findings filter keeps only affected assets", func(t *testing.T) {
		model := pressKey(readyTestModel(), "f")
		var assets []string
		for _, row := range model.visibleRows() {
			if row.asset != nil {
				assets = append(assets, row.asset.LogicalPath)
			}
		}
		want := []string{"home/.grok/config.toml"}
		if !reflect.DeepEqual(assets, want) {
			t.Fatalf("assets = %#v, want %#v", assets, want)
		}
	})

	t.Run("groups collapse and expand", func(t *testing.T) {
		model := readyTestModel()
		before := len(model.visibleRows())
		model = pressKey(model, "left")
		afterCollapse := len(model.visibleRows())
		if afterCollapse >= before {
			t.Fatalf("collapsed rows = %d, want fewer than %d", afterCollapse, before)
		}
		model = pressKey(model, "right")
		if afterExpand := len(model.visibleRows()); afterExpand != before {
			t.Fatalf("expanded rows = %d, want %d", afterExpand, before)
		}
	})
}

func TestUnattachedFindingsRemainVisible(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".agents", "skills", "orphan", "notes.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(path, []byte("# no SKILL.md\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	report, err := inventory.Scan(inventory.Options{Home: home})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(report.Assets) != 0 || len(report.Findings) != 1 ||
		report.Findings[0].Code != inventory.FindingIncompleteSkill {
		t.Fatalf("fixture is not an unattached incomplete-skill finding: %#v", report)
	}

	model := NewModel(inventory.Options{Home: home})
	next, _ := model.Update(scanMsg{generation: model.generation, report: report})
	model = next.(Model)
	assertVisibleFinding(t, model, inventory.FindingIncompleteSkill)

	model = pressKey(model, "f")
	assertVisibleFinding(t, model, inventory.FindingIncompleteSkill)

	model = pressKey(model, "/")
	for _, character := range "orphan" {
		model = pressKey(model, string(character))
	}
	assertVisibleFinding(t, model, inventory.FindingIncompleteSkill)
}

func TestQuitFromHelp(t *testing.T) {
	model := pressKey(readyTestModel(), "?")
	next, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if command == nil {
		t.Fatal("q from help returned no command")
	}
	message := command()
	if _, ok := message.(tea.QuitMsg); !ok {
		t.Fatalf("q from help command returned %T, want tea.QuitMsg", message)
	}
	if !next.(Model).showHelp {
		t.Fatal("quit unexpectedly mutated help state before program exit")
	}
}

func TestReloadIgnoresStaleScanResult(t *testing.T) {
	model := readyTestModel()
	next, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = next.(Model)
	if command == nil || model.status != statusLoading || model.generation != 2 {
		t.Fatalf("reload state = status %d generation %d command nil=%v", model.status, model.generation, command == nil)
	}

	stale := testReport()
	stale.ProducedBy = "stale"
	next, _ = model.Update(scanMsg{generation: 1, report: stale})
	model = next.(Model)
	if model.report.ProducedBy == "stale" || model.status != statusLoading {
		t.Fatalf("stale scan result was accepted: %#v", model.report)
	}
}

func TestScanCommandIsReadOnly(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(path, []byte("# review\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	before := snapshotTree(t, home)

	message := scanCommand(inventory.Options{Home: home}, 7)()
	result, ok := message.(scanMsg)
	if !ok {
		t.Fatalf("message type = %T, want scanMsg", message)
	}
	if result.err != nil {
		t.Fatalf("scan: %v", result.err)
	}
	if result.generation != 7 || result.report.Summary.CandidateAssets != 1 {
		t.Fatalf("scan result = generation %d candidates %d", result.generation, result.report.Summary.CandidateAssets)
	}

	after := snapshotTree(t, home)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only scan changed home:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestRunRejectsNonTTYAndMissingTERM(t *testing.T) {
	t.Run("non terminal streams", func(t *testing.T) {
		err := run(
			Options{Home: t.TempDir(), Input: strings.NewReader(""), Output: &strings.Builder{}},
			func(string) string { return "xterm-256color" },
			func(uintptr) bool { return true },
		)
		if !errors.Is(err, ErrNotTTY) {
			t.Fatalf("error = %v, want ErrNotTTY", err)
		}
	})

	t.Run("missing TERM", func(t *testing.T) {
		input, err := os.CreateTemp(t.TempDir(), "input")
		if err != nil {
			t.Fatal(err)
		}
		defer input.Close()
		output, err := os.CreateTemp(t.TempDir(), "output")
		if err != nil {
			t.Fatal(err)
		}
		defer output.Close()
		interactive := interactiveTerminal(
			Options{Home: t.TempDir(), Input: input, Output: output},
			func(string) string { return "" },
			func(uintptr) bool { return true },
		)
		if interactive {
			t.Fatal("terminal without TERM was accepted")
		}
	})
}

func TestRunRejectsNonTTYBeforeCreatingExplicitWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new-workspace")
	err := run(
		Options{
			Home: t.TempDir(), Workspace: root,
			Input: strings.NewReader(""), Output: &strings.Builder{},
		},
		func(string) string { return "xterm-256color" },
		func(uintptr) bool { return false },
	)
	if !errors.Is(err, ErrNotTTY) {
		t.Fatalf("error = %v, want ErrNotTTY", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("non-TTY invocation created the workspace: %v", err)
	}
}

func TestInventoryViewGolden(t *testing.T) {
	model := readyTestModel()
	model.width = 100
	model.height = 16
	model.cursor = 5
	model.plain = true

	got := model.View()
	path := filepath.Join("testdata", "inventory.golden")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v\n--- got ---\n%s", err, got)
	}
	if got != strings.TrimSuffix(string(want), "\n") {
		t.Fatalf("view differs from %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func readyTestModel() Model {
	model := NewModel(inventory.Options{Home: "/unused"})
	model.plain = true
	next, _ := model.Update(scanMsg{generation: model.generation, report: testReport()})
	return next.(Model)
}

func pressKey(model Model, value string) Model {
	var message tea.KeyMsg
	switch value {
	case "up":
		message = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		message = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		message = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		message = tea.KeyMsg{Type: tea.KeyRight}
	default:
		message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
	}
	next, _ := model.Update(message)
	return next.(Model)
}

func assertVisibleFinding(t *testing.T, model Model, code inventory.FindingCode) {
	t.Helper()
	for _, row := range model.visibleRows() {
		if row.finding != nil && row.finding.Code == code {
			return
		}
	}
	t.Fatalf("finding %q is not visible in rows: %#v", code, model.visibleRows())
}

func testReport() inventory.Report {
	return inventory.Report{
		SchemaVersion: 1,
		Kind:          "inventory",
		ProducedBy:    "aiah test",
		Summary: inventory.Summary{
			TotalAssets:      4,
			CandidateAssets:  3,
			ExcludedAssets:   1,
			CandidateByType:  map[inventory.AssetType]int{inventory.TypeRules: 1, inventory.TypeSkill: 2},
			ProjectMigration: inventory.ProjectNotScanned,
			DeviceMigration:  inventory.DeviceInventoryOnly,
		},
		Assets: []inventory.Asset{
			{
				LogicalPath: "home/.claude/CLAUDE.md", Source: inventory.SourceClaude,
				Scope: inventory.ScopeGlobal, Type: inventory.TypeRules,
				Portability: inventory.PortabilityAdapterRequired, Sensitivity: inventory.SensitivityPrivate,
				Status: inventory.AssetCandidate, Files: []string{"home/.claude/CLAUDE.md"},
			},
			{
				LogicalPath: "home/.codex/skills/review", Source: inventory.SourceCodex,
				Scope: inventory.ScopeGlobal, Type: inventory.TypeSkill,
				Portability: inventory.PortabilityAdapterRequired, Sensitivity: inventory.SensitivityPrivate,
				Status: inventory.AssetCandidate, Files: []string{"home/.codex/skills/review/SKILL.md"},
			},
			{
				LogicalPath: "home/.grok/config.toml", Source: inventory.SourceGrok,
				Scope: inventory.ScopeGlobal, Type: inventory.TypeConfig,
				Portability: inventory.PortabilityExcluded, Sensitivity: inventory.SensitivitySecret,
				Status: inventory.AssetExcluded, Files: []string{"home/.grok/config.toml"},
			},
			{
				LogicalPath: "home/.agents/skills/shared", Source: inventory.SourceShared,
				Scope: inventory.ScopeGlobal, Type: inventory.TypeSkill,
				Portability: inventory.PortabilityAdapterRequired, Sensitivity: inventory.SensitivityPublic,
				Status: inventory.AssetCandidate, Files: []string{"home/.agents/skills/shared/SKILL.md"},
			},
		},
		Findings: []inventory.Finding{
			{
				Code: inventory.FindingSuspectedSecret, Severity: inventory.SeverityError,
				Message: "secret", Paths: []string{"home/.grok/config.toml"},
			},
		},
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%04o", info.Mode().Type(), info.Mode().Perm())
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(body))
		}
		snapshot[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}
