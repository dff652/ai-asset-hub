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
		"e 换机检查",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("migration view omits %q:\n%s", want, view)
		}
	}
}

func TestMigrationWriteActionsRequireReadableStatus(t *testing.T) {
	for _, pressed := range []string{"e", "p", "v"} {
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

func TestMigrationPreflightUsesTheProfileAndWritesNothing(t *testing.T) {
	workspaceRoot := copyMigrationFixture(t, "workspace-valid")
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".codex", "auth.json"),
		[]byte(`{"token":"device-private"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	model := migrationTestModel(t, workspaceRoot, home, t.TempDir())
	beforeWorkspace := snapshotTree(t, workspaceRoot)
	beforeHome := snapshotTree(t, home)

	updated, focus := model.Update(keyPress("e"))
	model = updated.(Model)
	if focus == nil || !model.choosingProfile ||
		model.profilePurpose != profileForPreflight ||
		!strings.Contains(model.View(), "不会生成安装包") {
		t.Fatalf("preflight profile state=choosing %v purpose=%d command nil=%v\n%s",
			model.choosingProfile, model.profilePurpose, focus == nil, model.View())
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || model.building ||
		model.migrationFlow.mode != migrationModePreflight ||
		model.migrationFlow.preflightStatus != statusLoading {
		t.Fatalf("preflight state=mode %d status %d building %v command nil=%v",
			model.migrationFlow.mode,
			model.migrationFlow.preflightStatus,
			model.building,
			command == nil,
		)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "dist")); !os.IsNotExist(err) {
		t.Fatalf("starting preflight created dist: %v", err)
	}

	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.migrationFlow.preflightStatus != statusReady ||
		!model.migrationFlow.preflightReport.Ok ||
		model.migrationFlow.preflightReport.Summary.DevicePrivateItems != 1 {
		t.Fatalf("preflight report=%#v err=%v",
			model.migrationFlow.preflightReport,
			model.migrationFlow.preflightErr,
		)
	}
	view := model.View()
	for _, want := range []string{
		"换机前置检查", "检查阶段零写入", "本机不迁移 1",
		"目标工具 · claude", "本机不迁移 · home/.codex/auth.json",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("preflight view omits %q:\n%s", want, view)
		}
	}
	if !reflect.DeepEqual(beforeWorkspace, snapshotTree(t, workspaceRoot)) ||
		!reflect.DeepEqual(beforeHome, snapshotTree(t, home)) {
		t.Fatal("preflight changed the workspace or target HOME")
	}
}

func TestMigrationPublishRequiresTypedConfirmation(t *testing.T) {
	workspaceRoot := copyMigrationFixture(t, "workspace-valid")
	home := t.TempDir()
	channelRoot := t.TempDir()
	model := migrationTestModel(t, workspaceRoot, home, channelRoot)

	updated, focus := model.Update(keyPress("p"))
	model = updated.(Model)
	if focus == nil || !model.choosingProfile || model.profilePurpose != profileForPublish {
		t.Fatalf("publish profile state=choosing %v purpose=%d command nil=%v",
			model.choosingProfile, model.profilePurpose, focus == nil)
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

func TestMigrationPullChecksTheSelectedPackageBeforeDiffAndDoesNotWriteHome(t *testing.T) {
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
	updated, packageCheckCommand := model.Update(pullCommand())
	model = updated.(Model)
	if packageCheckCommand == nil ||
		model.screen != screenMigration ||
		model.migrationFlow.mode != migrationModePreflight ||
		model.migrationFlow.preflightStatus != statusLoading ||
		!model.hasPackagePreflight() ||
		model.deployOptions.Package != "" ||
		model.packageFromBuild {
		t.Fatalf("pull did not enter package check: screen=%d mode=%d status=%d package=%q generated=%v command nil=%v",
			model.screen,
			model.migrationFlow.mode,
			model.migrationFlow.preflightStatus,
			model.deployOptions.Package,
			model.packageFromBuild,
			packageCheckCommand == nil,
		)
	}
	updated, _ = model.Update(packageCheckCommand())
	model = updated.(Model)
	if model.migrationFlow.preflightStatus != statusReady ||
		!model.migrationFlow.preflightReport.Ok ||
		model.migrationFlow.preflightReport.Subject.Source != "package" ||
		model.migrationFlow.preflightReport.Subject.Version != "2026.07.1" {
		t.Fatalf("package check report=%#v err=%v",
			model.migrationFlow.preflightReport,
			model.migrationFlow.preflightErr,
		)
	}
	for _, want := range []string{
		"取回版本检查", "已绑定所选版本、资产组合与 SHA256",
		"Enter 进入变更预览", "2026.07.1",
	} {
		if view := model.View(); !strings.Contains(view, want) {
			t.Fatalf("package check view omits %q:\n%s", want, view)
		}
	}
	if strings.Contains(model.View(), "正在检查目标设备") {
		t.Fatalf("completed package check retained a loading notice:\n%s", model.View())
	}
	if !reflect.DeepEqual(beforeHome, snapshotTree(t, home)) {
		t.Fatal("versions, pull, or package check wrote the target HOME")
	}

	updated, diffCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if diffCommand == nil || model.screen != screenDeployment {
		t.Fatalf("a successful package check did not enter diff: screen=%d command nil=%v",
			model.screen, diffCommand == nil)
	}
	if model.deployOptions.Package == "" ||
		model.deployOptions.ExpectedSHA256 != model.migrationFlow.pulledReport.SHA256 {
		t.Fatalf("diff was not bound to pulled package: %#v", model.deployOptions)
	}
	updated, _ = model.Update(diffCommand())
	model = updated.(Model)
	if model.diffStatus != statusReady || !model.diffReport.Ok {
		t.Fatalf("pulled package diff failed: status=%d err=%v report=%#v",
			model.diffStatus, model.deployErr, model.diffReport)
	}
	if !reflect.DeepEqual(beforeHome, snapshotTree(t, home)) {
		t.Fatal("versions, pull, package check, or diff wrote the target HOME before typed apply")
	}
}

func TestPulledPackageBlockersPreventEnteringDiff(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	model.screen = screenMigration
	model.migrationFlow.mode = migrationModePreflight
	model.migrationFlow.pulledReport = channel.PullReport{
		Package: filepath.Join(t.TempDir(), "selected.tar"),
		SHA256:  strings.Repeat("a", 64),
	}
	model.migrationFlow.preflightStatus = statusReady
	model.migrationFlow.preflightReport = migration.PreflightReport{Ok: false}
	model.deployOptions = apply.Options{
		Home: t.TempDir(),
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(Model)
	if command != nil || next.screen != screenMigration ||
		!next.noticeIsWarn ||
		!strings.Contains(next.notice, "目标设备检查未通过") {
		t.Fatalf("blocked package entered diff: screen=%d command nil=%v notice=%q",
			next.screen, command == nil, next.notice)
	}
	if strings.Contains(next.View(), "Enter 进入变更预览") {
		t.Fatalf("blocked package view still offered continuation:\n%s", next.View())
	}
}

func TestPulledPackageCheckDoesNotExposePublishOrVersionShortcuts(t *testing.T) {
	for _, pressed := range []string{"p", "v"} {
		model := readyTestModel().WithHome(true).WithMaintenance(true)
		model.screen = screenMigration
		model.migrationFlow.mode = migrationModePreflight
		model.migrationFlow.status = statusReady
		model.migrationFlow.preflightStatus = statusReady
		model.migrationFlow.preflightReport = migration.PreflightReport{Ok: true}
		model.migrationFlow.pulledReport = channel.PullReport{
			Package: filepath.Join(t.TempDir(), "selected.tar"),
		}

		updated, command := model.Update(keyPress(pressed))
		next := updated.(Model)
		if command != nil || next.migrationFlow.mode != migrationModePreflight ||
			next.choosingProfile {
			t.Fatalf("key %q escaped package check: mode=%d choosing=%v command nil=%v",
				pressed, next.migrationFlow.mode, next.choosingProfile, command == nil)
		}
	}
}

func TestMigrationHomeKeyWorksInEveryMode(t *testing.T) {
	for _, mode := range []migrationMode{
		migrationModeStatus,
		migrationModeVersions,
		migrationModePreflight,
	} {
		model := readyTestModel().WithHome(true).WithMaintenance(true)
		model.screen = screenMigration
		model.migrationFlow.mode = mode
		model.migrationFlow.status = statusReady
		model.migrationFlow.versionsStatus = statusReady
		model.migrationFlow.preflightStatus = statusReady

		updated, command := model.Update(keyPress("m"))
		next := updated.(Model)
		if command != nil || next.screen != screenHome {
			t.Fatalf("mode %d did not return home: screen=%d command nil=%v",
				mode, next.screen, command == nil)
		}
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
	for index, item := range NewModel(inventory.Options{}).homeItems() {
		if item.action == action {
			return index
		}
	}
	t.Fatalf("home action %d not found", action)
	return 0
}
