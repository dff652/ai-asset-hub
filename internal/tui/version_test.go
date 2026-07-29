package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/version"
)

const testUpgradeCommand = "curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v0.1.3/scripts/install.sh | sh"

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
		"aiah · version", "0.1.2", "1234567890ab", "2026-07-29T00:00:00Z",
		"assets", "2026.07.1", "0.1.3", "update available",
		"curl -fsSL \\", "'https://raw.githubusercontent.com/dff652/'\\",
		"'ai-asset-hub/v0.1.3/scripts/install.sh' | sh",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("version view omits %q:\n%s", want, view)
		}
	}
}

func TestDisplayUpgradeCommandIsCopyableAtNarrowWidth(t *testing.T) {
	lines := displayUpgradeCommand(testUpgradeCommand)
	if len(lines) != 3 {
		t.Fatalf("lines = %#v", lines)
	}
	for _, line := range lines {
		if len(line) > 52 {
			t.Fatalf("upgrade line is too wide (%d): %q", len(line), line)
		}
	}
	if got := strings.Join(lines, "\n"); got != "curl -fsSL \\\n"+
		"  'https://raw.githubusercontent.com/dff652/'\\\n"+
		"  'ai-asset-hub/v0.1.3/scripts/install.sh' | sh" {
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

func doctorDeployment(name, release string) *apply.DoctorDeployment {
	return &apply.DoctorDeployment{Package: name, Version: release}
}

func inventoryOptions(home string) inventory.Options {
	return inventory.Options{Home: home}
}

func applyOptions(pkg, home string) apply.Options {
	return apply.Options{Package: pkg, Home: home}
}
