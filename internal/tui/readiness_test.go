package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/readiness"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestReadinessCommandMatchesCoreAndWritesNothing(t *testing.T) {
	workspaceRoot := copyTUIWorkspaceFixture(t)
	home := t.TempDir()
	project := t.TempDir()
	before := map[string]map[string]string{
		"workspace": snapshotTree(t, workspaceRoot),
		"home":      snapshotTree(t, home),
		"project":   snapshotTree(t, project),
	}
	options := readiness.Options{
		WorkspaceRoot: workspaceRoot,
		Profile:       "personal",
		Home:          home,
		Project:       project,
	}
	expected, err := readiness.Inspect(options)
	if err != nil {
		t.Fatalf("core inspect: %v", err)
	}

	raw := readinessCommand(options)()
	message, ok := raw.(readinessMsg)
	if !ok {
		t.Fatalf("message type = %T, want readinessMsg", raw)
	}
	if message.err != nil || !reflect.DeepEqual(message.report, expected) {
		t.Fatalf("TUI readiness diverged from core:\n got=%#v\nwant=%#v\nerr=%v",
			message.report, expected, message.err)
	}
	for name, root := range map[string]string{
		"workspace": workspaceRoot, "home": home, "project": project,
	} {
		if after := snapshotTree(t, root); !reflect.DeepEqual(after, before[name]) {
			t.Fatalf("%s changed during readiness command:\nbefore=%#v\nafter=%#v",
				name, before[name], after)
		}
	}
}

func TestHomeReadinessRequiresWorkspaceThenProfile(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	model.homeCursor = homeActionIndex(t, homeActionReadiness)
	updated, command := model.Update(keyPress("enter"))
	next := updated.(Model)
	if command == nil || !next.choosingWorkspace ||
		next.afterWorkspace != homeActionReadiness {
		t.Fatalf("readiness without library: choosing=%v after=%d command nil=%v",
			next.choosingWorkspace, next.afterWorkspace, command == nil)
	}

	workspaceRoot := copyTUIWorkspaceFixture(t)
	model = readyTestModel().
		WithWorkspace(workspaceRoot).
		WithHome(true).
		WithMaintenance(true)
	model.homeCursor = homeActionIndex(t, homeActionReadiness)
	updated, command = model.Update(keyPress("enter"))
	next = updated.(Model)
	if command == nil || !next.choosingProfile ||
		next.profilePurpose != profileForReadiness {
		t.Fatalf("readiness profile prompt: choosing=%v purpose=%d command nil=%v",
			next.choosingProfile, next.profilePurpose, command == nil)
	}
	if view := next.View(); !strings.Contains(view, "检查迁移准备") {
		t.Fatalf("readiness profile title missing:\n%s", view)
	}
}

func TestReadinessOpenAndRefreshWriteNothing(t *testing.T) {
	workspaceRoot := copyTUIWorkspaceFixture(t)
	home := t.TempDir()
	project := t.TempDir()
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	before := map[string]map[string]string{
		"workspace": snapshotTree(t, workspaceRoot),
		"home":      snapshotTree(t, home),
		"project":   snapshotTree(t, project),
		"evidence":  snapshotTree(t, evidenceRoot),
	}

	model := NewModel(inventory.Options{Home: home, Project: project}).
		WithWorkspace(workspaceRoot).
		WithHome(true).
		WithMaintenance(true)
	model.plain = true
	updated, command := model.beginReadiness("personal")
	model = updated.(Model)
	if command == nil || model.screen != screenReadiness ||
		model.readinessFlow.status != statusLoading {
		t.Fatalf("begin readiness: screen=%d status=%d command nil=%v",
			model.screen, model.readinessFlow.status, command == nil)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.readinessFlow.status != statusReady || !model.readinessFlow.report.Ok {
		t.Fatalf("readiness report not ready: %#v err=%v",
			model.readinessFlow.report, model.readinessFlow.err)
	}
	if model.readinessFlow.report.BackupEvidence.Status != readiness.StatusMissing ||
		model.readinessFlow.report.RestoreExercise.Status != readiness.StatusMissing {
		t.Fatalf("expected missing evidence statuses: backup=%s restore=%s",
			model.readinessFlow.report.BackupEvidence.Status,
			model.readinessFlow.report.RestoreExercise.Status)
	}

	updated, refresh := model.Update(keyPress("r"))
	model = updated.(Model)
	if refresh == nil {
		t.Fatal("refresh did not schedule readiness command")
	}
	updated, _ = model.Update(refresh())
	model = updated.(Model)
	if model.readinessFlow.status != statusReady {
		t.Fatalf("refresh status=%d", model.readinessFlow.status)
	}

	for name, root := range map[string]string{
		"workspace": workspaceRoot, "home": home, "project": project, "evidence": evidenceRoot,
	} {
		if after := snapshotTree(t, root); !reflect.DeepEqual(after, before[name]) {
			t.Fatalf("%s changed on open/refresh:\nbefore=%#v\nafter=%#v",
				name, before[name], after)
		}
	}
}

func TestReadinessViewShowsThreeStatusesAndNextStep(t *testing.T) {
	model := readinessGoldenModel(languageZhCN)
	view := model.View()
	for _, needle := range []string{
		"换机与备份",
		"需关注",
		"[ready]",
		"[missing]",
		"可以打包",
		"已记录外部副本",
		"恢复已验证",
		"下一步：把安装包放到本机之外",
		"package-readiness",
	} {
		if !strings.Contains(view, needle) {
			t.Fatalf("readiness view omits %q:\n%s", needle, view)
		}
	}
	// Status is not color-only: enum codes remain in plain mode.
	if strings.Contains(view, "lipgloss") {
		t.Fatal("unexpected style token in plain view")
	}
}

func TestReadinessViewWidthsKeepStatusMeaning(t *testing.T) {
	for _, width := range []int{360, 900, 60, 100} {
		model := readinessGoldenModel(languageEnglish)
		model.width = width
		view := model.View()
		for _, needle := range []string{
			"[ready]", "[missing]", "Package ready", "External copy", "Restore exercise", "Next:",
		} {
			if !strings.Contains(view, needle) {
				t.Fatalf("width %d omits %q:\n%s", width, needle, view)
			}
		}
	}
}

func TestReadinessEvidencePathPromptAndMismatch(t *testing.T) {
	workspaceRoot := copyTUIWorkspaceFixture(t)
	home := t.TempDir()
	model := NewModel(inventory.Options{Home: home}).
		WithWorkspace(workspaceRoot).
		WithHome(true).
		WithMaintenance(true)
	model.plain = true
	updated, command := model.beginReadiness("personal")
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)

	// Create mismatch evidence after open; path still under .aiah/evidence.
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	wrong := strings.Repeat("0", 64)
	body := []byte(
		"schemaVersion: 1\nkind: backup-evidence\nsubject:\n  name: fixture-personal\n  version: \"2026.07.1\"\n  profile: personal\n  selectionSHA256: " +
			wrong +
			"\ncopy:\n  type: git-commit\n  reference: \"git-commit:private-should-not-leak\"\nrecordedAt: \"2026-08-01T00:00:00Z\"\n",
	)
	if err := os.WriteFile(filepath.Join(evidenceRoot, "backup.yaml"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	updated, _ = model.Update(keyPress("b"))
	model = updated.(Model)
	if model.readinessFlow.choosingEvidence != readinessEvidenceBackup {
		t.Fatalf("backup evidence input not open: %#v", model.readinessFlow)
	}
	for _, character := range "backup.yaml" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil {
		t.Fatal("evidence path did not re-run readiness")
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.readinessFlow.report.BackupEvidence.Status != readiness.StatusMismatch {
		t.Fatalf("backup status = %s, want mismatch", model.readinessFlow.report.BackupEvidence.Status)
	}
	view := model.View()
	if !strings.Contains(view, "[mismatch]") || strings.Contains(view, "private-should-not-leak") {
		t.Fatalf("mismatch view incorrect or leaked reference:\n%s", view)
	}
}

func TestReadinessReadyWithSubjectBoundEvidence(t *testing.T) {
	workspaceRoot := copyTUIWorkspaceFixture(t)
	home := t.TempDir()
	base, err := readiness.Inspect(readiness.Options{
		WorkspaceRoot: workspaceRoot, Profile: "personal", Home: home,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceRoot, "backup.yaml"), []byte(
		"schemaVersion: 1\nkind: backup-evidence\nsubject:\n  name: "+base.Subject.Name+
			"\n  version: \""+base.Subject.Version+"\"\n  profile: "+base.Subject.Profile+
			"\n  selectionSHA256: "+base.Subject.SelectionSHA256+
			"\ncopy:\n  type: git-commit\n  reference: \"git-commit:ok-ref\"\nrecordedAt: \"2026-08-01T00:00:00Z\"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	targets := strings.Join(base.MigrationPreflight.Targets, ", ")
	if err := os.WriteFile(filepath.Join(evidenceRoot, "restore.yaml"), []byte(
		"schemaVersion: 1\nkind: restore-exercise\nsubject:\n  name: "+base.Subject.Name+
			"\n  version: \""+base.Subject.Version+"\"\n  profile: "+base.Subject.Profile+
			"\n  selectionSHA256: "+base.Subject.SelectionSHA256+
			"\n  packageSHA256: "+strings.Repeat("c", 64)+
			"\ntargets: ["+targets+"]\nisolated:\n  home: true\n  project: true\nsteps:\n"+
			"  pull: {status: passed}\n  preflight: {status: passed}\n  diff: {status: passed}\n"+
			"  apply: {status: passed}\n  doctor: {status: passed}\n  rollback: {status: passed}\n"+
			"producedBy: aiah-test\ncompletedAt: \"2026-08-01T00:00:00Z\"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	model := NewModel(inventory.Options{Home: home}).
		WithWorkspace(workspaceRoot).
		WithHome(true).
		WithMaintenance(true)
	model.plain = true
	model.readinessFlow.backupEvidencePath = "backup.yaml"
	model.readinessFlow.restoreExercisePath = "restore.yaml"
	updated, command := model.beginReadiness("personal")
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)
	report := model.readinessFlow.report
	if report.Level != readiness.LevelReady ||
		report.BackupEvidence.Status != readiness.StatusRecorded ||
		report.RestoreExercise.Status != readiness.StatusPassed {
		t.Fatalf("ready report = level=%s backup=%s restore=%s",
			report.Level, report.BackupEvidence.Status, report.RestoreExercise.Status)
	}
	view := model.View()
	for _, needle := range []string{"[recorded]", "[passed]", "下一步：资产变更后请重新运行本检查"} {
		if !strings.Contains(view, needle) {
			t.Fatalf("ready view omits %q:\n%s", needle, view)
		}
	}
	if strings.Contains(view, "ok-ref") {
		t.Fatal("view leaked full external reference")
	}
}

func TestReadinessGoldenByLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language language
		golden   string
	}{
		{name: "zh-CN", language: languageZhCN, golden: "readiness.zh-CN.golden"},
		{name: "en", language: languageEnglish, golden: "readiness.en.golden"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := readinessGoldenModel(test.language)
			model.width = 100
			model.height = 30
			model.plain = true
			got := model.View()
			path := filepath.Join("testdata", test.golden)
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v\n--- got ---\n%s", err, got)
			}
			if got != strings.TrimSuffix(string(want), "\n") {
				t.Fatalf("view differs from %s:\n--- got ---\n%s\n--- want ---\n%s",
					path, got, want)
			}
		})
	}
}

func readinessGoldenModel(lang language) Model {
	model := readyTestModel().WithWorkspace("/tmp/assets").WithHome(true).WithMaintenance(true).
		withLanguage(lang)
	model.plain = true
	model.screen = screenReadiness
	model.readinessFlow.status = statusReady
	model.readinessFlow.profile = "personal"
	model.readinessFlow.report = readiness.Report{
		SchemaVersion: 1,
		Kind:          "migration-readiness",
		ProducedBy:    "aiah test",
		Ok:            true,
		Level:         readiness.LevelAttention,
		Subject: readiness.Subject{
			Name: "fixture-personal", Version: "2026.07.1", Profile: "personal",
			SelectionSHA256: strings.Repeat("a", 64),
		},
		PackageReadiness: readiness.PackageReadiness{
			Status: readiness.StatusReady, AssetCount: 2, FileCount: 2,
		},
		MigrationPreflight: readiness.MigrationPreflight{
			Status:  readiness.StatusReady,
			Targets: []string{"claude", "codex", "shared"},
		},
		BackupEvidence:  readiness.BackupEvidence{Status: readiness.StatusMissing},
		RestoreExercise: readiness.RestoreExercise{Status: readiness.StatusMissing, Targets: []string{}},
		Findings: []workspace.Finding{{
			Code: "package-readiness", Severity: workspace.SeverityInfo,
			Message: "package can be built",
		}},
	}
	return model
}

func copyTUIWorkspaceFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "workspace-valid")
	destination := t.TempDir()
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy workspace fixture: %v", err)
	}
	return destination
}
