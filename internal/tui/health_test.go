package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestDoctorCommandMatchesCoreAndWritesNothing(t *testing.T) {
	home := t.TempDir()
	pkg := buildTUIFixturePackage(t, "workspace-valid")
	applied, err := apply.Apply(apply.Options{
		Package: pkg, Home: home, Targets: []string{"claude"},
	})
	if err != nil || !applied.Ok {
		t.Fatalf("apply fixture: err=%v report=%#v", err, applied)
	}
	before := snapshotTree(t, home)
	expected, err := apply.Doctor(apply.DoctorOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}

	raw := doctorCommand(apply.DoctorOptions{Home: home})()
	message, ok := raw.(doctorMsg)
	if !ok {
		t.Fatalf("message type = %T, want doctorMsg", raw)
	}
	if message.err != nil || !reflect.DeepEqual(message.report, expected) {
		t.Fatalf("TUI doctor diverged from core:\n got=%#v\nwant=%#v\nerr=%v",
			message.report, expected, message.err)
	}
	if after := snapshotTree(t, home); !reflect.DeepEqual(after, before) {
		t.Fatalf("doctor command wrote to home:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDoctorViewShowsCoreReport(t *testing.T) {
	report := apply.DoctorReport{
		Ok: true,
		Summary: apply.DoctorSummary{
			Deployment: true, Backups: 2, Unchanged: 1, LocallyModified: 1,
		},
		Deployment: &apply.DoctorDeployment{
			BackupID: "backup-123", Package: "personal", Version: "1",
			Profile: "personal", ProducedBy: "aiah test", AppliedAt: "now",
		},
		Drift: []apply.DriftEntry{{
			Path: "home/.claude/managed.md", Status: "locally-modified",
		}},
		Findings: []workspace.Finding{{
			Code: "deployment_drift", Severity: workspace.SeverityWarning,
			Message: "Core doctor message.", Paths: []string{"home/.claude/managed.md"},
		}},
	}
	model := doctorTestModel(t.TempDir(), report)
	rows := model.healthRows()
	for index, row := range rows {
		if row.finding != nil {
			model.healthCursor = index
			break
		}
	}
	view := model.View()
	for _, want := range []string{
		"doctor", "backup-123", "deployment_drift", "warning",
		"Core doctor message.", "home/.claude/managed.md",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor view omits %q:\n%s", want, view)
		}
	}
}

func TestRollbackRequiresHealthyCurrentDeploymentAndTypedConfirmation(t *testing.T) {
	home := t.TempDir()
	blocked := doctorTestModel(home, apply.DoctorReport{
		Ok: false,
		Deployment: &apply.DoctorDeployment{
			BackupID: "backup-123",
		},
		Findings: []workspace.Finding{{
			Code: "invalid_backup", Severity: workspace.SeverityError,
			Message: "bad backup",
		}},
	})
	updated, command := blocked.Update(keyPress("x"))
	blocked = updated.(Model)
	if command != nil || blocked.rollbackConfirming || !blocked.noticeIsWarn ||
		!strings.Contains(blocked.notice, "安装检查") {
		t.Fatalf("failed doctor allowed rollback: %#v command=%v", blocked, command != nil)
	}

	model := doctorTestModel(home, apply.DoctorReport{
		Ok: true,
		Deployment: &apply.DoctorDeployment{
			BackupID: "backup-123",
		},
	})
	updated, command = model.Update(keyPress("x"))
	model = updated.(Model)
	if command != nil || !model.rollbackConfirming || model.rollbacking {
		t.Fatalf("single x reached rollback: confirming=%v running=%v command=%v",
			model.rollbackConfirming, model.rollbacking, command != nil)
	}
	for _, character := range "rollback" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || model.rollbackConfirming || !model.rollbacking {
		t.Fatalf("typed confirmation did not start rollback: confirming=%v running=%v command=%v",
			model.rollbackConfirming, model.rollbacking, command != nil)
	}
}

func TestRollbackCommandUsesCoreAndRefreshesHealth(t *testing.T) {
	home := t.TempDir()
	pkg := buildTUIFixturePackage(t, "workspace-valid")
	options := apply.Options{Package: pkg, Home: home, Targets: []string{"claude"}}
	applied, err := apply.Apply(options)
	if err != nil || !applied.Ok || applied.BackupID == "" {
		t.Fatalf("apply fixture: err=%v report=%#v", err, applied)
	}
	doctor, err := apply.Doctor(apply.DoctorOptions{Home: home})
	if err != nil || !doctor.Ok || doctor.Deployment == nil {
		t.Fatalf("doctor fixture: err=%v report=%#v", err, doctor)
	}
	model := doctorTestModel(home, doctor)
	updated, _ := model.Update(keyPress("x"))
	model = updated.(Model)
	for _, character := range "rollback" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	raw := command()
	message, ok := raw.(rollbackMsg)
	if !ok {
		t.Fatalf("message type = %T, want rollbackMsg", raw)
	}
	if message.err != nil || !message.report.Ok ||
		message.report.BackupID != applied.BackupID {
		t.Fatalf("rollback: err=%v report=%#v", message.err, message.report)
	}
	target := filepath.Join(home, ".claude", "skills", "review", "SKILL.md")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("rollback did not remove installed target: %v", err)
	}

	updated, refresh := model.Update(message)
	model = updated.(Model)
	if refresh == nil || model.doctorStatus != statusLoading ||
		model.status != statusLoading || model.rollbackResult == nil {
		t.Fatalf("rollback did not refresh health and inventory: %#v command=%v",
			model, refresh != nil)
	}
}

func TestSuccessfulApplySchedulesDoctor(t *testing.T) {
	home := t.TempDir()
	model := deploymentModel(
		apply.Options{Package: "fixture.tar", Home: home},
		successfulDiffReport(),
	)
	model = model.WithMaintenance(true)
	report := successfulDiffReport()
	report.DryRun = false
	report.BackupID = "backup-123"
	updated, command := model.Update(applyMsg{report: report})
	model = updated.(Model)
	if command == nil || model.doctorStatus != statusLoading {
		t.Fatalf("successful apply did not schedule doctor: %#v command=%v", model, command != nil)
	}
	raw := command()
	batch, ok := raw.(tea.BatchMsg)
	if !ok {
		t.Fatalf("refresh command returned %T, want tea.BatchMsg", raw)
	}
	foundDoctor := false
	for _, item := range batch {
		if _, ok := item().(doctorMsg); ok {
			foundDoctor = true
		}
	}
	if !foundDoctor {
		t.Fatal("successful apply refresh omitted doctor")
	}
}

func TestDeploymentOnlyModelDoesNotExposeMaintenance(t *testing.T) {
	model := deploymentModel(
		apply.Options{Package: "fixture.tar", Home: t.TempDir()},
		successfulDiffReport(),
	)
	report := successfulDiffReport()
	report.BackupID = "backup-123"
	model.applyResult = &report

	view := model.View()
	if strings.Contains(view, "h doctor") || strings.Contains(view, "按 h 进入 doctor") {
		t.Fatalf("deployment-only view exposed maintenance:\n%s", view)
	}
	updated, command := model.Update(keyPress("h"))
	model = updated.(Model)
	if command != nil || model.screen != screenDeployment || !model.noticeIsWarn {
		t.Fatalf("deployment-only model enabled maintenance: %#v command=%v",
			model, command != nil)
	}
}

func doctorTestModel(home string, report apply.DoctorReport) Model {
	model := NewModel(inventory.Options{Home: home}).WithMaintenance(true)
	model.plain = true
	model.screen = screenHealth
	updated, _ := model.Update(doctorMsg{report: report})
	return updated.(Model)
}
