package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/migration"
)

func TestHomeMigrationRequiresAnExplicitAssetLibrary(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	model.homeCursor = homeActionIndex(t, homeActionMigration)
	updated, command := model.Update(keyPress("enter"))
	next := updated.(Model)
	if command == nil || !next.choosingWorkspace ||
		next.afterWorkspace != homeActionMigration {
		t.Fatalf("migration state=choosing %v after=%d command nil=%v",
			next.choosingWorkspace, next.afterWorkspace, command == nil)
	}
}

func TestMigrationViewExplainsAlignmentAndExplicitActions(t *testing.T) {
	model := readyTestModel().
		WithWorkspace("/tmp/assets").
		WithHome(true).
		WithMaintenance(true)
	model.screen = screenMigration
	model.migrationFlow.status = statusReady
	model.migrationFlow.report = migration.Report{
		Ok: true,
		Library: migration.LibraryStatus{
			Root: "/tmp/assets", Name: "personal", Version: "1.2.3",
			AssetCount: 4, Profiles: []string{"personal"}, Ok: true,
		},
		Installation: migration.InstallationStatus{
			Present: true, Ok: true, Package: "personal", Version: "1.2.3",
			Profile: "personal", Targets: []string{"claude", "codex"},
		},
		Channel: migration.ChannelStatus{
			Selected: true, Path: "/mnt/aiah", ReleaseCount: 2,
			Latest: &channel.Release{Version: "1.2.3", Profile: "personal", SHA256: "abc"},
		},
		Alignment: migration.Alignment{Installation: "same-version", Channel: "same-version"},
	}
	view := model.View()
	for _, want := range []string{
		"迁移到其他设备", "版本对齐", "personal / 1.2.3",
		"claude、codex", "/mnt/aiah", "与资产库版本一致",
		"状态读取保持只读", "p 发布当前版本", "v 查看/取回版本",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("migration view omits %q:\n%s", want, view)
		}
	}
}

func TestMigrationWriteActionsRequireReadableStatus(t *testing.T) {
	for _, pressed := range []string{"p", "v"} {
		model := readyTestModel().
			WithWorkspace("/tmp/assets").
			WithHome(true).
			WithMaintenance(true)
		model.screen = screenMigration
		model.migrationFlow.status = statusFailed
		model.migrationFlow.channel = t.TempDir()

		updated, command := model.Update(keyPress(pressed))
		next := updated.(Model)
		if command != nil || next.choosingProfile ||
			next.migrationFlow.mode != migrationModeStatus ||
			!next.noticeIsWarn ||
			!strings.Contains(next.notice, "迁移状态尚未可用") {
			t.Fatalf("key %q bypassed failed status: %#v", pressed, next)
		}
	}
}

func TestMigrationPublishRequiresTypedConfirmation(t *testing.T) {
	workspaceRoot := copyMigrationFixture(t, "workspace-valid")
	home := t.TempDir()
	channelRoot := t.TempDir()
	model := migrationTestModel(t, workspaceRoot, home, channelRoot)

	updated, focus := model.Update(keyPress("p"))
	model = updated.(Model)
	if focus == nil || !model.choosingProfile || model.buildPurpose != buildForPublish {
		t.Fatalf("publish profile state=choosing %v purpose=%d command nil=%v",
			model.choosingProfile, model.buildPurpose, focus == nil)
	}
	updated, buildCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if buildCommand == nil || !model.building {
		t.Fatal("profile confirmation did not start a publish build")
	}
	updated, _ = model.Update(buildCommand())
	model = updated.(Model)
	if !model.migrationFlow.publishConfirming || model.migrationFlow.publishPackage == "" {
		t.Fatalf("build did not enter publish confirmation: %#v", model)
	}

	before := snapshotTree(t, channelRoot)
	for _, character := range "wrong" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, publishCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if publishCommand != nil || !model.migrationFlow.publishConfirming || !model.noticeIsWarn {
		t.Fatalf("wrong confirmation started publish: confirming=%v notice=%q command nil=%v",
			model.migrationFlow.publishConfirming, model.notice, publishCommand == nil)
	}
	if !strings.Contains(model.View(), "必须完整输入 publish") {
		t.Fatalf("publish confirmation did not show the mismatch:\n%s", model.View())
	}
	if after := snapshotTree(t, channelRoot); !reflect.DeepEqual(before, after) {
		t.Fatalf("channel changed before typed confirmation:\nbefore=%#v\nafter=%#v", before, after)
	}

	for _, character := range "publish" {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, publishCommand = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if publishCommand == nil || !model.migrationFlow.publishing ||
		model.migrationFlow.publishConfirming {
		t.Fatalf("publish confirmation state=publishing %v confirming %v command nil=%v",
			model.migrationFlow.publishing,
			model.migrationFlow.publishConfirming,
			publishCommand == nil)
	}
	updated, refresh := model.Update(publishCommand())
	model = updated.(Model)
	if refresh == nil || model.migrationFlow.publishing ||
		!strings.Contains(model.notice, "已发布") {
		t.Fatalf("publish notice=%q publishing=%v refresh nil=%v",
			model.notice, model.migrationFlow.publishing, refresh == nil)
	}
	updated, _ = model.Update(refresh())
	model = updated.(Model)
	if model.migrationFlow.status != statusReady ||
		model.migrationFlow.report.Alignment.Channel != "same-version" {
		t.Fatalf("post-publish status=%d report=%#v",
			model.migrationFlow.status, model.migrationFlow.report)
	}
}

func TestMigrationPullEntersExistingDiffAndDoesNotWriteHome(t *testing.T) {
	workspaceRoot := copyMigrationFixture(t, "workspace-valid")
	channelRoot := t.TempDir()
	buildOut := t.TempDir()
	built, err := build.Build(build.Options{
		Manifest: filepath.Join(workspaceRoot, "manifest.yaml"),
		Root:     workspaceRoot,
		Profile:  "personal",
		OutDir:   buildOut,
	})
	if err != nil || !built.Ok || built.Package == nil {
		t.Fatalf("build: err=%v report=%#v", err, built)
	}
	archive := filepath.Join(buildOut, built.Package.Archive)
	if _, err := channel.Publish(channel.PublishOptions{
		Package: archive,
		Channel: channelRoot,
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	manifestPath := filepath.Join(workspaceRoot, "manifest.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	updatedManifest := strings.Replace(string(manifest), `version: "2026.07.1"`,
		`version: "2026.07.2"`, 1)
	if updatedManifest == string(manifest) {
		t.Fatal("fixture version was not updated")
	}
	if err := os.WriteFile(manifestPath, []byte(updatedManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	secondOut := t.TempDir()
	second, err := build.Build(build.Options{
		Manifest: manifestPath,
		Root:     workspaceRoot,
		Profile:  "personal",
		OutDir:   secondOut,
	})
	if err != nil || !second.Ok || second.Package == nil {
		t.Fatalf("second build: err=%v report=%#v", err, second)
	}
	if _, err := channel.Publish(channel.PublishOptions{
		Package: filepath.Join(secondOut, second.Package.Archive),
		Channel: channelRoot,
	}); err != nil {
		t.Fatalf("second publish: %v", err)
	}

	home := t.TempDir()
	model := migrationTestModel(t, workspaceRoot, home, channelRoot)
	beforeHome := snapshotTree(t, home)

	updated, listCommand := model.Update(keyPress("v"))
	model = updated.(Model)
	if listCommand == nil || model.migrationFlow.mode != migrationModeVersions ||
		model.migrationFlow.versionsStatus != statusLoading {
		t.Fatalf("versions state=mode %d status %d command nil=%v",
			model.migrationFlow.mode, model.migrationFlow.versionsStatus, listCommand == nil)
	}
	updated, _ = model.Update(listCommand())
	model = updated.(Model)
	if model.migrationFlow.versionsStatus != statusReady ||
		len(model.migrationFlow.versionsReport.Releases) != 2 ||
		model.migrationFlow.versionsCursor != 1 ||
		model.migrationFlow.selectedRelease.Version != "" {
		t.Fatalf("versions report=%#v err=%v",
			model.migrationFlow.versionsReport,
			model.migrationFlow.versionsErr)
	}

	updated, _ = model.Update(keyPress("up"))
	model = updated.(Model)
	updated, _ = model.Update(keyPress("enter"))
	model = updated.(Model)
	if !model.migrationFlow.choosingPullOut ||
		model.migrationFlow.selectedRelease.Version != "2026.07.1" {
		t.Fatalf("pull selection=%#v choosing=%v",
			model.migrationFlow.selectedRelease, model.migrationFlow.choosingPullOut)
	}
	incoming := t.TempDir()
	for _, character := range incoming {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, pullCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if pullCommand == nil || !model.migrationFlow.pulling ||
		model.migrationFlow.choosingPullOut {
		t.Fatalf("pull state=pulling %v choosing %v command nil=%v",
			model.migrationFlow.pulling,
			model.migrationFlow.choosingPullOut,
			pullCommand == nil)
	}
	updated, diffCommand := model.Update(pullCommand())
	model = updated.(Model)
	if diffCommand == nil || model.screen != screenDeployment ||
		model.deployOptions.Package == "" || model.packageFromBuild {
		t.Fatalf("pull did not enter diff: screen=%d package=%q generated=%v command nil=%v",
			model.screen, model.deployOptions.Package, model.packageFromBuild, diffCommand == nil)
	}
	updated, _ = model.Update(diffCommand())
	model = updated.(Model)
	if model.diffStatus != statusReady || !model.diffReport.Ok {
		t.Fatalf("pulled package diff failed: status=%d err=%v report=%#v",
			model.diffStatus, model.deployErr, model.diffReport)
	}
	if !reflect.DeepEqual(beforeHome, snapshotTree(t, home)) {
		t.Fatal("versions, pull, or diff wrote the target HOME before typed apply")
	}
}

func TestChannelPromptDoesNotCreateTheTypedDirectory(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "not-created")
	model := readyTestModel().WithWorkspace(t.TempDir()).WithHome(true).WithMaintenance(true)
	model.screen = screenMigration

	updated, focus := model.Update(keyPress("c"))
	model = updated.(Model)
	if focus == nil || !model.migrationFlow.choosingChannel {
		t.Fatal("c did not open the channel path prompt")
	}
	for _, character := range missing {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || model.migrationFlow.choosingChannel ||
		model.migrationFlow.status != statusLoading {
		t.Fatalf("channel confirmation state=%#v command nil=%v", model, command == nil)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("read-only channel prompt created a directory: %v", err)
	}
	message := command()
	updated, _ = model.Update(message)
	if next := updated.(Model); next.migrationFlow.status != statusFailed {
		t.Fatalf("missing channel did not surface as a read failure: %#v",
			next.migrationFlow.report)
	}
}

func migrationTestModel(t *testing.T, workspaceRoot, home, channelRoot string) Model {
	t.Helper()
	model := NewModel(inventory.Options{Home: home}).
		WithWorkspace(workspaceRoot).
		WithHome(true).
		WithMaintenance(true).
		WithDeployment(apply.Options{Home: home})
	model.plain = true
	model.screen = screenMigration
	model.migrationFlow.channel = channelRoot
	report, err := migration.Inspect(model.migrationOptions())
	if err != nil {
		t.Fatalf("migration inspect: %v", err)
	}
	model.migrationFlow.status = statusReady
	model.migrationFlow.report = report
	return model
}

func copyMigrationFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", name)
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return destination
}

func homeActionIndex(t *testing.T, action homeAction) int {
	t.Helper()
	for index, item := range homeItems() {
		if item.action == action {
			return index
		}
	}
	t.Fatalf("home action %d not found", action)
	return 0
}
