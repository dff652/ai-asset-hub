package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/version"
)

const testUpgradeCommand = "curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v0.1.3/scripts/install.sh | AIAH_VERSION=0.1.3 sh"

func TestVersionScreenIsLocalUntilCheckRequested(t *testing.T) {
	model := NewModel(inventoryOptions(t.TempDir())).WithMaintenance(true)
	updated, command := model.Update(keyPress("v"))
	model = updated.(Model)
	if model.screen != screenVersion {
		t.Fatalf("screen = %v, want version", model.screen)
	}
	if command == nil {
		t.Fatal("version screen did not schedule local deployment doctor")
	}
	raw := command()
	if _, ok := raw.(doctorMsg); !ok {
		t.Fatalf("opening version screen ran %T, want local doctor only", raw)
	}
	if model.updateChecking || model.updateChecked {
		t.Fatalf("opening version screen checked network: %#v", model)
	}

	updated, command = model.Update(keyPress("c"))
	model = updated.(Model)
	if command == nil || !model.updateChecking {
		t.Fatalf("explicit c did not start update check: %#v", model)
	}
}

func TestVersionViewShowsBuildDeploymentAndUpdateResult(t *testing.T) {
	original := []string{version.Version, version.Commit, version.Date}
	t.Cleanup(func() {
		version.Version, version.Commit, version.Date = original[0], original[1], original[2]
	})
	version.Version = "0.1.2"
	version.Commit = "1234567890abcdef"
	version.Date = "2026-07-29T00:00:00Z"

	model := NewModel(inventoryOptions(t.TempDir())).WithMaintenance(true)
	model.plain = true
	model.screen = screenVersion
	model.doctorStatus = statusReady
	model.doctorReport.Deployment = doctorDeployment("assets", "2026.07.1")
	updated, _ := model.Update(updateCheckMsg{report: updater.Report{
		Ok:              true,
		CurrentVersion:  "0.1.2",
		LatestVersion:   "0.1.3",
		Status:          updater.StatusUpdateAvailable,
		UpdateAvailable: true,
		ReleaseURL:      "https://github.example/releases/tag/v0.1.3",
		UpgradeCommand:  testUpgradeCommand,
	}})
	model = updated.(Model)

	view := model.View()
	for _, want := range []string{
		"aiah · 关于与更新", "0.1.2", "1234567890ab", "2026-07-29T00:00:00Z",
		"assets", "2026.07.1", "0.1.3", "有可用更新",
		"curl -fsSL \\", "'https://raw.githubusercontent.com/dff652/'\\",
		"'ai-asset-hub/v0.1.3/scripts/install.sh' |",
		"AIAH_VERSION=0.1.3 sh",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("version view omits %q:\n%s", want, view)
		}
	}
}

func TestDisplayUpgradeCommandIsCopyableAtNarrowWidth(t *testing.T) {
	lines := displayUpgradeCommand(testUpgradeCommand)
	if len(lines) != 4 {
		t.Fatalf("lines = %#v", lines)
	}
	for _, line := range lines {
		if len(line) > 52 {
			t.Fatalf("upgrade line is too wide (%d): %q", len(line), line)
		}
	}
	if got := strings.Join(lines, "\n"); got != "curl -fsSL \\\n"+
		"  'https://raw.githubusercontent.com/dff652/'\\\n"+
		"  'ai-asset-hub/v0.1.3/scripts/install.sh' |\n"+
		"  AIAH_VERSION=0.1.3 sh" {
		t.Fatalf("copyable command = %q", got)
	}
}

func TestVersionCheckFailureStaysVisible(t *testing.T) {
	model := NewModel(inventoryOptions(t.TempDir())).WithMaintenance(true)
	model.plain = true
	model.screen = screenVersion
	model.updateChecking = true
	updated, _ := model.Update(updateCheckMsg{err: errors.New("network unavailable")})
	model = updated.(Model)
	if model.updateChecking || !model.updateChecked || model.updateErr == nil ||
		!strings.Contains(model.View(), "network unavailable") {
		t.Fatalf("failed check was hidden: %#v\n%s", model, model.View())
	}
}

func TestVersionViewGoldenByLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language language
		checked  bool
		golden   string
	}{
		{
			name: "offline zh-CN", language: languageZhCN,
			golden: "version.offline.zh-CN.golden",
		},
		{
			name: "offline en", language: languageEnglish,
			golden: "version.offline.en.golden",
		},
		{
			name: "available zh-CN", language: languageZhCN, checked: true,
			golden: "version.available.zh-CN.golden",
		},
		{
			name: "available en", language: languageEnglish, checked: true,
			golden: "version.available.en.golden",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restoreVersion := setVersionTestBuild()
			t.Cleanup(restoreVersion)
			model := versionGoldenModel(test.language, test.checked)

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

func TestEnglishVersionStatusesFailureAndHelp(t *testing.T) {
	model := versionGoldenModel(languageEnglish, false)
	statuses := map[string]string{
		updater.StatusCurrent:         "Up to date",
		updater.StatusUpdateAvailable: "Update available",
		updater.StatusAhead:           "ahead of the latest Release",
		updater.StatusDevelopment:     "Development build",
		"unexpected":                  "Unknown status (unexpected)",
	}
	for status, want := range statuses {
		if got := model.updateStatusLabel(status); !strings.Contains(got, want) {
			t.Errorf("English status %q = %q, want %q", status, got, want)
		}
	}

	model.updateChecking = true
	updated, _ := model.Update(updateCheckMsg{err: errors.New("network unavailable")})
	model = updated.(Model)
	if view := model.View(); !strings.Contains(view, "Check failed: network unavailable") {
		t.Fatalf("English update failure is not visible:\n%s", view)
	}

	model.showHelp = true
	for _, want := range []string{
		"About and updates · Help",
		"Opening this page reads only the local program",
		"Only c queries the latest GitHub Release",
		"never replaces the current binary",
	} {
		if view := model.View(); !strings.Contains(view, want) {
			t.Fatalf("English version help omits %q:\n%s", want, view)
		}
	}
}

func TestDeploymentOnlyModelDoesNotExposeVersionScreen(t *testing.T) {
	model := deploymentModel(
		applyOptions("fixture.tar", t.TempDir()),
		successfulDiffReport(),
	)
	updated, command := model.Update(keyPress("v"))
	model = updated.(Model)
	if command != nil || model.screen != screenDeployment || !model.noticeIsWarn {
		t.Fatalf("deployment-only model enabled version screen: %#v command=%v",
			model, command != nil)
	}
}

func TestUpdateCheckMessageStopsSpinner(t *testing.T) {
	model := NewModel(inventoryOptions(t.TempDir())).WithMaintenance(true)
	model.updateChecking = true
	updated, command := model.Update(updateCheckMsg{report: updater.Report{Ok: true}})
	if command != nil {
		t.Fatalf("update result returned command %T", command)
	}
	next := updated.(Model)
	if next.updateChecking || !next.updateChecked {
		t.Fatalf("message did not finish check: %#v", next)
	}
}

func versionGoldenModel(selectedLanguage language, checked bool) Model {
	model := NewModel(inventory.Options{Home: "/unused"}).
		WithMaintenance(true).
		withLanguage(selectedLanguage)
	model.plain = true
	model.width = 100
	model.height = 20
	model.screen = screenVersion
	model.doctorStatus = statusReady
	if checked {
		model.doctorReport.Deployment = &apply.DoctorDeployment{
			Package: "personal", Version: "2026.07.1", Profile: "personal",
			BackupID: "backup-123",
		}
		model.updateChecked = true
		model.updateReport = updater.Report{
			Ok:              true,
			CurrentVersion:  "0.1.2",
			LatestVersion:   "0.1.3",
			Status:          updater.StatusUpdateAvailable,
			UpdateAvailable: true,
			ReleaseURL:      "https://github.example/releases/tag/v0.1.3",
			UpgradeCommand:  testUpgradeCommand,
		}
	}
	return model
}

func setVersionTestBuild() func() {
	original := []string{version.Version, version.Commit, version.Date}
	version.Version = "0.1.2"
	version.Commit = "1234567890abcdef"
	version.Date = "2026-07-29T00:00:00Z"
	return func() {
		version.Version, version.Commit, version.Date = original[0], original[1], original[2]
	}
}

func doctorDeployment(name, release string) *apply.DoctorDeployment {
	return &apply.DoctorDeployment{Package: name, Version: release}
}

func inventoryOptions(home string) inventory.Options {
	return inventory.Options{Home: home}
}

func applyOptions(pkg, home string) apply.Options {
	return apply.Options{Package: pkg, Home: home}
}
