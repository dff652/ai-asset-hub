package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/migration"
	"github.com/dff652/ai-asset-hub/internal/preferences"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestN75CoreViewsPreserveRequiredInformationAtSupportedWidths(t *testing.T) {
	restoreVersion := setVersionTestBuild()
	defer restoreVersion()

	const (
		libraryPath = "/srv/aiah/operator-assets/this-path-is-intentionally-long-enough-to-exceed-one-hundred-columns/asset-library"
		channelPath = "/mnt/aiah/team-channel/this-path-is-intentionally-long-enough-to-exceed-one-hundred-columns/releases"
		packagePath = "/tmp/aiah/packages/personal-2026.07.31-personal.tar"
		changePath  = "home/.claude/skills/review/SKILL.md"
	)
	digest := strings.Repeat("a", 64)

	for _, selectedLanguage := range []language{languageZhCN, languageEnglish} {
		for _, width := range []int{100, 60} {
			name := fmt.Sprintf("%s/%d", selectedLanguage, width)
			t.Run(name, func(t *testing.T) {
				assertViewContains(t, "home",
					sizedModel(
						readyTestModel().
							WithWorkspace(libraryPath).
							WithHome(true).
							WithMaintenance(true).
							withLanguage(selectedLanguage),
						width,
					).View(),
					libraryPath,
				)

				inventoryModel := sizedModel(
					readyTestModel().
						WithWorkspace(libraryPath).
						withLanguage(selectedLanguage),
					width,
				)
				inventoryModel.cursor = assetRowIndex(t, inventoryModel.visibleRows())
				selectedAsset := inventoryModel.visibleRows()[inventoryModel.cursor].asset
				assertViewContains(t, "inventory", inventoryModel.View(),
					libraryPath,
					selectedAsset.LogicalPath,
					selectedAsset.Files[0],
				)

				diffReport := successfulDiffReport()
				diffReport.Targets = []string{"claude", "codex"}
				diffReport.Changes[0].Path = changePath
				diffReport.Changes[0].SHA256 = digest
				deployment := sizedModel(
					deploymentModel(
						apply.Options{
							Package: packagePath,
							Home:    "/tmp/aiah/home",
							Targets: []string{"claude", "codex"},
						},
						diffReport,
					).withLanguage(selectedLanguage),
					width,
				)
				deployment.diffCursor = 1
				assertViewContains(t, "change preview", deployment.View(),
					packagePath, "claude", "codex", changePath, digest,
				)

				blockedReport := apply.Report{
					SchemaVersion: 1,
					Kind:          "apply",
					Ok:            false,
					DryRun:        true,
					Targets:       []string{"claude", "codex"},
					Findings: []workspace.Finding{{
						Code:     "blocked_n75",
						Severity: workspace.SeverityError,
						Message:  "blocked before any write",
						Paths:    []string{changePath},
					}},
				}
				blocked := sizedModel(
					deploymentModel(
						apply.Options{
							Package: packagePath,
							Home:    "/tmp/aiah/home",
							Targets: []string{"claude", "codex"},
						},
						blockedReport,
					).withLanguage(selectedLanguage),
					width,
				)
				assertViewContains(t, "blocked change preview", blocked.View(),
					packagePath, "claude", "codex",
					"blocked_n75", "blocked before any write", changePath,
				)

				health := sizedModel(
					doctorTestModel("/tmp/aiah/home", healthGoldenReport()).
						withLanguage(selectedLanguage),
					width,
				)
				assertViewContains(t, "install check", health.View(),
					"backup-123", "personal.tar", "1.2.3", "personal",
				)

				migrationModel := sizedModel(
					migrationGoldenModel().withLanguage(selectedLanguage),
					width,
				)
				migrationModel.workspace = libraryPath
				migrationModel.migrationFlow.report.Library.Root = libraryPath
				migrationModel.migrationFlow.channel = channelPath
				migrationModel.migrationFlow.report.Channel.Path = channelPath
				migrationModel.migrationFlow.report.Channel.Latest.SHA256 = digest
				assertViewContains(t, "migration status", migrationModel.View(),
					libraryPath, channelPath, "personal", "1.2.3",
					"claude", "codex", digest,
				)

				versions := migrationModel
				versions.migrationFlow.mode = migrationModeVersions
				versions.migrationFlow.versionsStatus = statusReady
				versions.migrationFlow.versionsReport = channel.ListReport{
					Ok: true,
					Releases: []channel.Release{{
						Name: "personal", Version: "1.2.3", Profile: "personal",
						SHA256: digest,
					}},
				}
				assertViewContains(t, "migration versions", versions.View(),
					"personal", "1.2.3", digest,
				)

				devicePrivatePath := "home/.codex/device-private/this-path-is-intentionally-long-enough-to-exceed-one-hundred-columns/auth.json"
				preflight := migrationModel
				preflight.migrationFlow.mode = migrationModePreflight
				preflight.migrationFlow.preflightStatus = statusReady
				preflight.migrationFlow.preflightProfile = "personal"
				preflight.migrationFlow.preflightCursor = 1
				preflight.migrationFlow.preflightReport = migration.PreflightReport{
					Ok: true,
					Summary: migration.PreflightSummary{
						TargetCount: 1, DevicePrivateItems: 1,
					},
					Targets: []migration.TargetPreflight{{
						Target: "claude", Supported: true, Emitted: 3,
					}},
					DevicePrivate: []migration.DevicePrivateItem{{
						LogicalPath: devicePrivatePath,
						Source:      "codex",
						Type:        "config",
						Status:      "device-private",
					}},
				}
				assertViewContains(t, "migration preflight", preflight.View(),
					"personal", "claude", devicePrivatePath,
					"codex", "config", "device-private",
				)

				versionModel := sizedModel(
					versionGoldenModel(selectedLanguage, true),
					width,
				)
				assertViewContains(t, "version and update", versionModel.View(),
					"0.1.2", "0.1.3", "backup-123",
					"https://github.example/releases/tag/v0.1.3",
					"ai-asset-hub/v0.1.3/scripts/install.sh",
					"AIAH_VERSION=0.1.3 sh",
				)

				settings := sizedModel(
					readyTestModel().WithHome(true).WithMaintenance(true).
						withLanguage(selectedLanguage),
					width,
				)
				settings.screen = screenSettings
				settings.preferencePath = "/tmp/aiah/config/preferences.json"
				settings.settingsDraft = preferences.Defaults()
				settings.settingsDraft.PreferredAssetLibrary = libraryPath
				assertViewContains(t, "settings", settings.View(),
					libraryPath, settings.preferencePath,
				)
			})
		}
	}
}

func TestN75WriteConfirmationsPreserveScopeAtSupportedWidths(t *testing.T) {
	const (
		libraryPath = "/srv/aiah/operator-assets/this-path-is-intentionally-long-enough-to-exceed-one-hundred-columns/asset-library"
		channelPath = "/mnt/aiah/team-channel/this-path-is-intentionally-long-enough-to-exceed-one-hundred-columns/releases"
		packagePath = "/tmp/aiah/packages/this-path-is-intentionally-long-enough-to-exceed-one-hundred-columns/personal-2026.07.31-personal.tar"
	)
	for _, selectedLanguage := range []language{languageZhCN, languageEnglish} {
		for _, width := range []int{100, 60} {
			name := fmt.Sprintf("%s/%d", selectedLanguage, width)
			t.Run(name, func(t *testing.T) {
				report := successfulDiffReport()
				report.Targets = []string{"claude", "codex"}
				applyModel := sizedModel(
					deploymentModel(
						apply.Options{
							Package: packagePath,
							Home:    "/tmp/aiah/home",
							Targets: []string{"claude", "codex"},
						},
						report,
					).withLanguage(selectedLanguage),
					width,
				)
				applyModel.confirming = true
				applyModel.confirmInput.SetValue("apply")
				assertViewContains(t, "apply confirmation", applyModel.View(),
					packagePath, "claude", "codex", "apply",
				)

				rollback := sizedModel(
					doctorTestModel("/tmp/aiah/home", healthGoldenReport()).
						withLanguage(selectedLanguage),
					width,
				)
				rollback.rollbackConfirming = true
				rollback.rollbackInput.SetValue("rollback")
				assertViewContains(t, "rollback confirmation", rollback.View(),
					"personal.tar", "1.2.3", "personal", "backup-123", "rollback",
				)

				publish := sizedModel(
					migrationGoldenModel().withLanguage(selectedLanguage),
					width,
				)
				publish.migrationFlow.channel = channelPath
				publish.migrationFlow.publishPackage = packagePath
				publish.migrationFlow.publishConfirming = true
				publish.migrationFlow.publishInput.SetValue("publish")
				assertViewContains(t, "publish confirmation", publish.View(),
					packagePath, channelPath, "publish",
				)

				for _, action := range []string{manageUpdate, manageRemove} {
					manage := sizedModel(
						readyTestModel().
							WithWorkspace(libraryPath).
							withLanguage(selectedLanguage),
						width,
					)
					asset := manage.report.Assets[0]
					manage.catalog = workspace.CatalogReport{
						Ok: true,
						Items: []workspace.CatalogItem{{
							ID:          "rules.claude",
							LogicalPath: asset.LogicalPath,
							State:       workspace.LibrarySourceChanged,
							Asset:       &asset,
						}},
					}
					manage.selected = map[string]bool{asset.LogicalPath: true}
					manage.manageAction = action
					manage.confirmManage = true
					manage.manageInput.SetValue(action)
					assertViewContains(t, action+" confirmation", manage.View(),
						libraryPath, action, "1",
					)
				}
			})
		}
	}
}

func TestN75FakeHomePreferenceLifecycleAndCorruptionRecovery(t *testing.T) {
	root := t.TempDir()
	fakeHome := filepath.Join(root, "fake-home")
	if err := os.Mkdir(fakeHome, 0o700); err != nil {
		t.Fatal(err)
	}
	store := preferences.StoreOptions{
		Home:       fakeHome,
		ConfigPath: filepath.Join(root, "fake-config", "aiah", "preferences.json"),
	}
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeOutside := snapshotTree(t, outside)

	model, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if model.language != languageZhCN ||
		model.currentPreferences != preferences.Defaults() {
		t.Fatalf("first launch = language %q preferences %#v",
			model.language, model.currentPreferences)
	}
	if _, err := os.Stat(filepath.Dir(store.ConfigPath)); !os.IsNotExist(err) {
		t.Fatalf("first launch created fake config directory: %v", err)
	}

	document := preferences.Defaults()
	document.Language = preferences.LanguageEnglish
	document.Density = preferences.DensityDetailed
	message := savePreferencesCommand(store, document)().(preferencesSaveMsg)
	if message.err != nil {
		t.Fatal(message.err)
	}
	restarted, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.language != languageEnglish ||
		restarted.density != preferences.DensityDetailed {
		t.Fatalf("restart ignored saved preferences: language=%q density=%q",
			restarted.language, restarted.density)
	}

	corrupt := []byte(`{"schemaVersion":`)
	if err := os.WriteFile(store.ConfigPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.currentPreferences != preferences.Defaults() ||
		!reflect.DeepEqual(
			recovered.preferenceWarnings,
			[]preferences.WarningCode{preferences.WarningInvalidDocument},
		) {
		t.Fatalf("corruption recovery = preferences %#v warnings %v",
			recovered.currentPreferences, recovered.preferenceWarnings)
	}
	if body := readPreferenceBytes(t, store.ConfigPath); !reflect.DeepEqual(body, corrupt) {
		t.Fatal("corrupt preferences were implicitly rewritten")
	}
	if afterOutside := snapshotTree(t, outside); !reflect.DeepEqual(afterOutside, beforeOutside) {
		t.Fatalf("fake HOME/config lifecycle polluted outside tree:\nbefore=%#v\nafter=%#v",
			beforeOutside, afterOutside)
	}
}

func sizedModel(model Model, width int) Model {
	model.width = width
	model.height = 30
	model.plain = true
	return model
}

func assertViewContains(t *testing.T, name, view string, required ...string) {
	t.Helper()
	for _, value := range required {
		if !strings.Contains(view, value) {
			t.Fatalf("%s omitted required value %q:\n%s", name, value, view)
		}
	}
}
