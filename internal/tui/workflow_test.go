package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/inventory"
)

func TestWorkspacePromptCreatesOnlyTheConfirmedPath(t *testing.T) {
	model := composeModel(t, "")
	updated, _ := model.Update(keyPress("w"))
	model = updated.(Model)

	for _, character := range "~/portable-assets" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if !model.choosingWorkspace {
		t.Fatal("left arrow cancelled path input instead of moving the cursor")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if _, err := os.Stat(filepath.Join(model.options.Home, "portable-assets")); !os.IsNotExist(err) {
		t.Fatalf("typing a path created it before confirmation: %v", err)
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || !model.preparingWorkspace || model.choosingWorkspace {
		t.Fatalf("confirm state = preparing %v choosing %v command nil=%v",
			model.preparingWorkspace, model.choosingWorkspace, command == nil)
	}
	message := command()
	updated, _ = model.Update(message)
	model = updated.(Model)
	want := filepath.Join(model.options.Home, "portable-assets")
	if model.workspace != want || model.preparingWorkspace || model.noticeIsWarn {
		t.Fatalf("prepared workspace = %q preparing=%v notice=%q warn=%v",
			model.workspace, model.preparingWorkspace, model.notice, model.noticeIsWarn)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("confirmed workspace does not exist: info=%v err=%v", info, err)
	}
}

func TestBuildWaitsForComposeToFinish(t *testing.T) {
	model := composeModel(t, t.TempDir())
	model.composing = true
	updated, command := model.Update(keyPress("b"))
	next := updated.(Model)
	if command != nil || next.choosingProfile || !next.noticeIsWarn ||
		!strings.Contains(next.notice, "加入资产库") {
		t.Fatalf("build raced compose: choosing=%v notice=%q command nil=%v",
			next.choosingProfile, next.notice, command == nil)
	}
}

func TestGuidedComposeBuildEntersDeploymentDiff(t *testing.T) {
	workspaceRoot := t.TempDir()
	model := composeModel(t, workspaceRoot)
	model.cursor = assetRowIndex(t, model.visibleRows())
	updated, _ := model.Update(keyPress(" "))
	model = updated.(Model)

	updated, compose := model.Update(keyPress("w"))
	model = updated.(Model)
	if compose == nil {
		t.Fatal("selected asset did not start compose")
	}
	updated, _ = model.Update(compose())
	model = updated.(Model)
	if model.noticeIsWarn {
		t.Fatalf("compose failed: %s", model.notice)
	}
	if !model.choosingProfile || model.profileInput.Value() != "personal" {
		t.Fatalf("profile prompt = choosing %v value %q command nil=%v",
			model.choosingProfile, model.profileInput.Value(), false)
	}
	updated, build := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if build == nil || !model.building {
		t.Fatal("confirming the profile did not start build")
	}
	raw := build()
	message, ok := raw.(buildMsg)
	if !ok {
		t.Fatalf("build command returned %T, want buildMsg", raw)
	}
	if message.err != nil || !message.report.Ok || message.packagePath == "" {
		t.Fatalf("build failed: err=%v report=%#v", message.err, message.report)
	}

	updated, diff := model.Update(message)
	model = updated.(Model)
	if diff == nil || model.screen != screenDeployment ||
		model.deployOptions.Package != message.packagePath || !model.packageFromBuild {
		t.Fatalf("build did not enter diff: screen=%d package=%q generated=%v command nil=%v",
			model.screen, model.deployOptions.Package, model.packageFromBuild, diff == nil)
	}
	if !strings.HasPrefix(message.packagePath, filepath.Join(workspaceRoot, "dist")+string(filepath.Separator)) {
		t.Fatalf("package escaped workspace dist: %q", message.packagePath)
	}

	updated, _ = model.Update(diff())
	model = updated.(Model)
	if model.diffStatus != statusReady || !model.diffReport.Ok {
		t.Fatalf("guided diff failed: status=%d err=%v report=%#v",
			model.diffStatus, model.deployErr, model.diffReport)
	}
}

func TestBuildFailureStaysInInventory(t *testing.T) {
	model := composeModel(t, t.TempDir())
	model.deployOptions.Package = "/tmp/old-generated.tar"
	model.packageFromBuild = true
	updated, _ := model.Update(buildMsg{
		report: buildFailureReport(),
	})
	next := updated.(Model)
	if next.screen != screenInventory || !next.noticeIsWarn ||
		!strings.Contains(next.notice, "unknown_profile") ||
		next.deployOptions.Package != "" || next.packageFromBuild {
		t.Fatalf("failed build state = screen %d notice %q warn=%v package=%q generated=%v",
			next.screen, next.notice, next.noticeIsWarn,
			next.deployOptions.Package, next.packageFromBuild)
	}
	updated, command := next.Update(keyPress("d"))
	if command != nil || !strings.Contains(updated.(Model).notice, "未指定安装包") {
		t.Fatal("failed rebuild still allowed the old generated package to enter diff")
	}
}

func TestGuidedBuildKeepsRequestedDeploymentTargets(t *testing.T) {
	model := NewModel(inventory.Options{Home: t.TempDir()}).
		WithDeployment(apply.Options{Targets: []string{"codex"}})
	if strings.Join(model.deployOptions.Targets, ",") != "codex" ||
		model.deployOptions.Package != "" || model.screen != screenInventory {
		t.Fatalf("guided deployment options were not retained: %#v", model.deployOptions)
	}
}

func buildFailureReport() build.Report {
	return build.Report{
		Ok: false,
		Findings: []build.Finding{{
			Code: "unknown_profile", Message: "Profile is not defined in the manifest.",
		}},
	}
}
