package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/preferences"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestDeploymentRequiresTypedApplyConfirmation(t *testing.T) {
	options := apply.Options{Package: "fixture.tar", Home: t.TempDir(), Targets: []string{"claude"}}
	model := deploymentModel(options, successfulDiffReport())

	updated, command := model.Update(keyPress("a"))
	model = updated.(Model)
	if command != nil || !model.confirming || model.applying {
		t.Fatalf("single a reached apply: confirming=%v applying=%v command=%v",
			model.confirming, model.applying, command != nil)
	}

	for _, character := range "apply" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || model.confirming || !model.applying {
		t.Fatalf("typed confirmation did not start apply: confirming=%v applying=%v command=%v",
			model.confirming, model.applying, command != nil)
	}

	updated, second := model.Update(keyPress("a"))
	if second != nil || !updated.(Model).applying {
		t.Fatal("apply could be triggered again while already running")
	}
}

func TestDeploymentRejectsWrongConfirmation(t *testing.T) {
	model := deploymentModel(
		apply.Options{Package: "fixture.tar", Home: t.TempDir()},
		successfulDiffReport(),
	)
	updated, _ := model.Update(keyPress("a"))
	model = updated.(Model)
	for _, character := range "yes" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command != nil || !model.confirming || model.applying ||
		!model.noticeIsWarn || !strings.Contains(model.notice, "apply") {
		t.Fatalf("wrong confirmation was not rejected: %#v command=%v", model, command != nil)
	}
}

func TestDeploymentGroupsChangesAndCollapses(t *testing.T) {
	report := successfulDiffReport()
	report.Summary = apply.Summary{Staged: 4, Create: 1, Update: 1, Unchanged: 1, Skipped: 1}
	report.Changes = []apply.Change{
		{Path: "home/create", Action: "create"},
		{Path: "home/update", Action: "update"},
		{Path: "home/unchanged", Action: "unchanged"},
		{Path: "home/skipped", Action: "skipped"},
	}
	model := deploymentModel(apply.Options{Package: "fixture.tar", Home: t.TempDir()}, report)
	rows := model.deploymentRows()
	for _, group := range []string{
		"action:create", "action:update", "action:unchanged", "action:skipped",
	} {
		found := false
		for _, row := range rows {
			if row.kind == deploymentGroupRow && row.key == group {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing change group %q in %#v", group, rows)
		}
	}

	model.diffCursor = 0
	before := len(model.deploymentRows())
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if after := len(updated.(Model).deploymentRows()); after >= before {
		t.Fatalf("collapse kept %d rows, before %d", after, before)
	}
}

func TestDensityOnlyChangesOptionalDiffExpansion(t *testing.T) {
	report := successfulDiffReport()
	report.Summary = apply.Summary{
		Staged: 4, Create: 1, Update: 1, Unchanged: 1, Skipped: 1,
	}
	report.Changes = []apply.Change{
		{Path: "home/create", Action: "create"},
		{Path: "home/update", Action: "update"},
		{Path: "home/unchanged", Action: "unchanged"},
		{Path: "home/skipped", Action: "skipped"},
	}
	standard := deploymentModel(
		apply.Options{Package: "fixture.tar", Home: t.TempDir()},
		report,
	).withLanguage(languageEnglish)
	standard.density = preferences.DensityStandard
	standard.resetDiffExpansionForDensity()
	detailed := deploymentModel(
		apply.Options{Package: "fixture.tar", Home: t.TempDir()},
		report,
	).withLanguage(languageEnglish)
	detailed.density = preferences.DensityDetailed
	detailed.resetDiffExpansionForDensity()

	standardView := standard.View()
	detailedView := detailed.View()
	for _, necessary := range []string{
		"fixture.tar", "Create 1", "Update 1", "Unchanged 1", "Skipped 1",
		"Create (1)", "Update (1)", "Unchanged (1)", "Skipped (1)",
	} {
		if !strings.Contains(standardView, necessary) ||
			!strings.Contains(detailedView, necessary) {
			t.Fatalf("density hid necessary information %q", necessary)
		}
	}
	for _, optional := range []string{"home/unchanged", "home/skipped"} {
		if strings.Contains(standardView, optional) {
			t.Fatalf("standard density expanded optional detail %q", optional)
		}
		if !strings.Contains(detailedView, optional) {
			t.Fatalf("detailed density did not expand optional detail %q", optional)
		}
	}

	standard.confirming = true
	detailed.confirming = true
	if standard.View() != detailed.View() {
		t.Fatal("density changed the write confirmation screen")
	}
}

func TestDensityKeepsNonDiffAndBlockedScreensEquivalent(t *testing.T) {
	blockedReport := apply.Report{
		SchemaVersion: 1,
		Kind:          "apply",
		Ok:            false,
		DryRun:        true,
		Findings: []workspace.Finding{{
			Code: "blocked_fixture", Severity: workspace.SeverityError,
			Message: "blocked for test", Paths: []string{"home/.claude/blocked"},
		}},
	}
	blocked := deploymentModel(
		apply.Options{Package: "blocked.tar", Home: "/unused"},
		blockedReport,
	).withLanguage(languageEnglish)

	home := readyTestModel().WithHome(true).WithMaintenance(true).
		withLanguage(languageEnglish)
	inventoryModel := readyTestModel().withLanguage(languageEnglish)
	health := doctorTestModel("/unused", healthGoldenReport()).
		withLanguage(languageEnglish)
	migrationModel := migrationGoldenModel().withLanguage(languageEnglish)
	versionModel := versionGoldenModel(languageEnglish, true)

	tests := []struct {
		name  string
		model Model
	}{
		{name: "home", model: home},
		{name: "inventory", model: inventoryModel},
		{name: "health", model: health},
		{name: "migration", model: migrationModel},
		{name: "version", model: versionModel},
		{name: "blocked deployment", model: blocked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			standard := test.model
			detailed := test.model
			detailed.diffExpanded = cloneExpansionMap(test.model.diffExpanded)
			detailed.density = preferences.DensityDetailed
			detailed.resetDiffExpansionForDensity()
			if standard.View() != detailed.View() {
				t.Fatalf("density changed %s outside optional diff detail", test.name)
			}
		})
	}
}

func cloneExpansionMap(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func TestDiffCommandMatchesCoreAndWritesNothing(t *testing.T) {
	pkg := buildTUIFixturePackage(t, "workspace-valid")
	home := t.TempDir()
	options := apply.Options{
		Package: pkg,
		Home:    home,
		Targets: []string{"claude", "codex"},
	}
	before := snapshotTree(t, home)
	expected, err := apply.Diff(options)
	if err != nil {
		t.Fatal(err)
	}
	raw := diffCommand(options)()
	message, ok := raw.(diffMsg)
	if !ok {
		t.Fatalf("message type = %T, want diffMsg", raw)
	}
	if message.err != nil || !reflect.DeepEqual(message.report, expected) {
		t.Fatalf("TUI diff diverged from core:\n got=%#v\nwant=%#v\nerr=%v",
			message.report, expected, message.err)
	}
	if after := snapshotTree(t, home); !reflect.DeepEqual(after, before) {
		t.Fatalf("diff command wrote to home:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestApplySuccessShowsBackupAndRollbackCommand(t *testing.T) {
	pkg := buildTUIFixturePackage(t, "workspace-valid")
	home := t.TempDir()
	options := apply.Options{Package: pkg, Home: home, Targets: []string{"claude"}}
	diff := diffCommand(options)().(diffMsg)
	if diff.err != nil || !diff.report.Ok {
		t.Fatalf("diff: err=%v report=%#v", diff.err, diff.report)
	}
	model := deploymentModel(options, diff.report)

	updated, _ := model.Update(keyPress("a"))
	model = updated.(Model)
	for _, character := range "apply" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil {
		t.Fatal("typed confirmation returned no apply command")
	}
	raw := command()
	message, ok := raw.(applyMsg)
	if !ok {
		t.Fatalf("message type = %T, want applyMsg", raw)
	}
	if message.err != nil || !message.report.Ok || message.report.BackupID == "" {
		t.Fatalf("apply: err=%v report=%#v", message.err, message.report)
	}
	if !reflect.DeepEqual(message.report.Targets, []string{"claude"}) {
		t.Fatalf("TUI apply changed selected targets: %#v", message.report.Targets)
	}
	updated, refresh := model.Update(message)
	model = updated.(Model)
	if refresh == nil || model.status != statusLoading {
		t.Fatal("successful apply did not schedule an inventory refresh")
	}

	view := model.View()
	for _, want := range []string{
		"backupId)  " + message.report.BackupID,
		rollbackCommand(options, message.report.BackupID),
		"应用完成",
		"目标工具  claude",
		"写入 " + fmt.Sprint(message.report.Summary.Written),
		"下一步",
		"安装检查",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("success view omits %q:\n%s", want, view)
		}
	}
}

func TestNoOpApplyResultSaysNoRollbackIsNeeded(t *testing.T) {
	options := apply.Options{Package: "fixture.tar", Home: t.TempDir()}
	model := deploymentModel(options, successfulDiffReport())
	report := successfulDiffReport()
	report.DryRun = false
	report.Targets = []string{"codex"}
	report.Summary = apply.Summary{Staged: 1, Unchanged: 1}
	report.Changes[0].Action = "unchanged"
	updated, _ := model.Update(applyMsg{report: report})
	view := updated.(Model).View()
	for _, want := range []string{"backupId)  —", "无需撤销", "目标工具  codex", "下一步"} {
		if !strings.Contains(view, want) {
			t.Fatalf("no-op result omits %q:\n%s", want, view)
		}
	}
}

func TestDeploymentViewGoldenByLanguage(t *testing.T) {
	tests := []struct {
		name       string
		language   language
		confirming bool
		golden     string
	}{
		{
			name: "preview zh-CN", language: languageZhCN,
			golden: "deployment.preview.zh-CN.golden",
		},
		{
			name: "preview en", language: languageEnglish,
			golden: "deployment.preview.en.golden",
		},
		{
			name: "confirm zh-CN", language: languageZhCN, confirming: true,
			golden: "deployment.confirm.zh-CN.golden",
		},
		{
			name: "confirm en", language: languageEnglish, confirming: true,
			golden: "deployment.confirm.en.golden",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := deploymentModel(
				apply.Options{Package: "fixture.tar", Home: "/unused", Targets: []string{"claude"}},
				successfulDiffReport(),
			).WithMaintenance(true).withLanguage(test.language)
			model.confirming = test.confirming
			model.width = 100
			model.height = 18
			model.plain = true

			got := model.View()
			path := filepath.Join("testdata", test.golden)
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v\n--- got ---\n%s", err, got)
			}
			if normalizeTerminalGolden(got) != normalizeTerminalGolden(string(want)) {
				t.Fatalf(
					"view differs from %s:\n--- got ---\n%s\n--- want ---\n%s",
					path,
					got,
					want,
				)
			}
		})
	}
}

func normalizeTerminalGolden(value string) string {
	lines := strings.Split(strings.TrimSuffix(value, "\n"), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	return strings.Join(lines, "\n")
}

func TestEnglishApplyResultPreservesBackupAndUndoCommand(t *testing.T) {
	options := apply.Options{
		Package: "fixture.tar", Home: "/tmp/home", Targets: []string{"claude", "codex"},
	}
	model := deploymentModel(options, successfulDiffReport()).
		WithMaintenance(true).
		withLanguage(languageEnglish)
	report := successfulDiffReport()
	report.DryRun = false
	report.Targets = []string{"claude", "codex"}
	report.BackupID = "backup-123"
	report.Summary = apply.Summary{Staged: 1, Written: 1}
	model.applyResult = &report

	view := model.View()
	for _, want := range []string{
		"Apply complete",
		"Targets  claude, codex",
		"Install restore point (backupId)  backup-123",
		"Undo command  " + rollbackCommand(options, report.BackupID),
		"Press h for install check",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("English result omits %q:\n%s", want, view)
		}
	}
}

func TestDeploymentFailureShowsCoreFindingsVerbatim(t *testing.T) {
	t.Run("symlink target", func(t *testing.T) {
		pkg := buildTUIFixturePackage(t, "workspace-valid")
		home := t.TempDir()
		if err := os.Symlink(t.TempDir(), filepath.Join(home, ".claude")); err != nil {
			t.Fatal(err)
		}
		options := apply.Options{Package: pkg, Home: home, Targets: []string{"claude"}}
		report, err := apply.Diff(options)
		if err != nil {
			t.Fatal(err)
		}
		assertFailedReportVisible(t, deploymentModel(options, report), report)
	})

	t.Run("mcp conflict", func(t *testing.T) {
		t.Setenv("EXAMPLE_SERVICE_TOKEN", "resolved-example-service-token")
		pkg := buildTUIFixturePackage(t, "workspace-2b")
		home := t.TempDir()
		project := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(home, ".claude.json"),
			[]byte(`{"mcpServers":{"example":{"command":"conflicting-command"}}}`),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		options := apply.Options{
			Package: pkg,
			Home:    home,
			Project: project,
			Targets: []string{"claude"},
		}
		report, err := apply.Diff(options)
		if err != nil {
			t.Fatal(err)
		}
		assertFailedReportVisible(t, deploymentModel(options, report), report)
	})
}

func TestRollbackCommandIncludesEveryInstallRoot(t *testing.T) {
	options := apply.Options{
		Home:    filepath.Join("/tmp", "home with space"),
		Project: filepath.Join("/tmp", "project's root"),
	}
	command := rollbackCommand(options, "backup-123")
	for _, want := range []string{
		"--home '/tmp/home with space'",
		"--project '/tmp/project'\"'\"'s root'",
		"--backup backup-123",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("rollback command omits %q: %s", want, command)
		}
	}
}

func deploymentModel(options apply.Options, report apply.Report) Model {
	model := NewModel(inventory.Options{Home: options.Home, Project: options.Project}).
		WithDeployment(options)
	model.plain = true
	updated, _ := model.Update(diffMsg{report: report})
	return updated.(Model)
}

func successfulDiffReport() apply.Report {
	return apply.Report{
		SchemaVersion: 1,
		Kind:          "apply",
		Ok:            true,
		DryRun:        true,
		Summary:       apply.Summary{Staged: 1, Create: 1},
		Changes: []apply.Change{{
			Path: "home/.claude/skills/review/SKILL.md", Action: "create", SHA256: "abc",
		}},
	}
}

func assertFailedReportVisible(t *testing.T, model Model, report apply.Report) {
	t.Helper()
	if report.Ok || len(report.Findings) == 0 {
		t.Fatalf("fixture did not produce a failed report: %#v", report)
	}
	view := model.View()
	for _, finding := range report.Findings {
		for _, want := range append(
			[]string{finding.Code, string(finding.Severity), finding.Message},
			finding.Paths...,
		) {
			if !strings.Contains(view, want) {
				t.Fatalf("view changed or hid finding text %q:\n%s", want, view)
			}
		}
	}
	updated, command := model.Update(keyPress("a"))
	if command != nil || updated.(Model).confirming {
		t.Fatal("failed diff allowed apply confirmation")
	}
}

func buildTUIFixturePackage(t *testing.T, fixture string) string {
	t.Helper()
	out := t.TempDir()
	report, err := build.Build(build.Options{
		Manifest: filepath.Join("..", "..", "testdata", fixture, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil || !report.Ok {
		t.Fatalf("build: err=%v report=%#v", err, report)
	}
	return filepath.Join(out, report.Package.Archive)
}
