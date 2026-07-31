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

func TestHomeViewExplainsPurposeStateAndTasks(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	view := model.View()
	for _, needle := range []string{
		"AI 编程资产管理器",
		"统一管理 Claude、Codex、Grok",
		"资产库",
		"资产状态",
		"整理本机资产",
		"预览并应用资产库",
		"安装检查与撤销",
		"迁移到其他设备",
		"关于与更新",
		"偏好设置",
	} {
		if !strings.Contains(view, needle) {
			t.Fatalf("home view omits %q:\n%s", needle, view)
		}
	}
}

func TestHomeViewShowsUnifiedAssetAndDeploymentState(t *testing.T) {
	model := readyTestModel().
		WithWorkspace("/tmp/assets").
		WithHome(true).
		WithMaintenance(true)
	model.catalog = workspace.CatalogReport{
		Ok: true,
		Summary: workspace.CatalogSummary{
			Unmanaged: 2, Managed: 3, SourceChanged: 4, LibraryOnly: 5, Blocked: 1,
		},
	}
	model.doctorStatus = statusReady
	model.doctorReport = apply.DoctorReport{
		Ok: true,
		Deployment: &apply.DoctorDeployment{
			Package: "personal.tar.gz", Version: "0.2.0",
			Targets: []string{"claude", "codex"},
		},
	}

	view := model.View()
	for _, needle := range []string{
		"未纳管 2", "已纳管 3", "待更新 4", "仅库内 5",
		"不可纳管 1", "本机问题 1",
		"personal.tar.gz 0.2.0 · claude,codex · 正常",
	} {
		if !strings.Contains(view, needle) {
			t.Fatalf("home view omits %q:\n%s", needle, view)
		}
	}
	if !strings.Contains(view, "待更新 4\n          仅库内 5") {
		t.Fatalf("unified state is not split for narrow terminals:\n%s", view)
	}
}

func TestHomeInitRunsReadOnlyInventoryAndInstallationChecks(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, ".claude", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("# review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, home)
	model := NewModel(inventory.Options{Home: home}).WithHome(true).WithMaintenance(true)

	raw := model.Init()()
	batch, ok := raw.(tea.BatchMsg)
	if !ok {
		t.Fatalf("init message = %T, want tea.BatchMsg", raw)
	}
	var foundScan, foundDoctor bool
	for _, command := range batch {
		switch result := command().(type) {
		case scanMsg:
			foundScan = result.err == nil
		case doctorMsg:
			foundDoctor = result.err == nil
		}
	}
	if !foundScan || !foundDoctor {
		t.Fatalf("init checks = scan %v doctor %v", foundScan, foundDoctor)
	}
	if after := snapshotTree(t, home); !reflect.DeepEqual(after, before) {
		t.Fatalf("home initialization changed home:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestHomeOrganizeRequiresAnExplicitAssetLibrary(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	updated, command := model.Update(keyPress("enter"))
	next := updated.(Model)
	if command == nil || !next.choosingWorkspace ||
		next.afterWorkspace != homeActionOrganize {
		t.Fatalf(
			"organize state = choosing=%v after=%d command nil=%v",
			next.choosingWorkspace,
			next.afterWorkspace,
			command == nil,
		)
	}
	if view := next.View(); !strings.Contains(view, "选择资产库") {
		t.Fatalf("asset library prompt missing:\n%s", view)
	}
}

func TestHomeOrganizeWithAssetLibraryEntersLocalAssets(t *testing.T) {
	model := readyTestModel().
		WithWorkspace("/tmp/assets").
		WithHome(true).
		WithMaintenance(true)
	updated, command := model.Update(keyPress("enter"))
	next := updated.(Model)
	if command != nil || next.screen != screenInventory {
		t.Fatalf("screen=%d command nil=%v", next.screen, command == nil)
	}
	if !strings.Contains(next.View(), "本机 AI 资产") {
		t.Fatalf("local assets title missing:\n%s", next.View())
	}
}

func TestMainKeyReturnsToHome(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	model.screen = screenInventory
	updated, command := model.Update(keyPress("m"))
	next := updated.(Model)
	if command != nil || next.screen != screenHome {
		t.Fatalf("screen=%d command nil=%v", next.screen, command == nil)
	}
}

func TestHomeHelpUsesUserLanguage(t *testing.T) {
	model := readyTestModel().WithHome(true).WithMaintenance(true)
	updated, _ := model.Update(keyPress("?"))
	view := updated.(Model).View()
	for _, needle := range []string{"加入资产库", "预览变化", "撤销", "事实源", "偏好设置"} {
		if !strings.Contains(view, needle) {
			t.Fatalf("home help omits %q:\n%s", needle, view)
		}
	}
}

func TestHomeViewGoldenByLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language language
		golden   string
	}{
		{name: "zh-CN", language: languageZhCN, golden: "home.zh-CN.golden"},
		{name: "en", language: languageEnglish, golden: "home.en.golden"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := readyTestModel().
				WithHome(true).
				WithMaintenance(true).
				withLanguage(test.language)
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
