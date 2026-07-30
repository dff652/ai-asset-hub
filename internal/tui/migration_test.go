package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/channel"
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

func TestMigrationViewExplainsReadOnlyAlignment(t *testing.T) {
	model := readyTestModel().
		WithWorkspace("/tmp/assets").
		WithHome(true).
		WithMaintenance(true)
	model.screen = screenMigration
	model.migrationStatus = statusReady
	model.migrationReport = migration.Report{
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
		"迁移到其他设备", "只读状态", "personal / 1.2.3",
		"claude、codex", "/mnt/aiah", "与资产库版本一致",
		"不会生成、发布、取回或应用",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("migration view omits %q:\n%s", want, view)
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
	if focus == nil || !model.choosingChannel {
		t.Fatal("c did not open the channel path prompt")
	}
	for _, character := range missing {
		updated, _ = model.Update(keyPress(string(character)))
		model = updated.(Model)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || model.choosingChannel || model.migrationStatus != statusLoading {
		t.Fatalf("channel confirmation state=%#v command nil=%v", model, command == nil)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("read-only channel prompt created a directory: %v", err)
	}
	message := command()
	updated, _ = model.Update(message)
	if next := updated.(Model); next.migrationStatus != statusFailed {
		t.Fatalf("missing channel did not surface as a read failure: %#v", next.migrationReport)
	}
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
