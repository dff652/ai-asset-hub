package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/preferences"
)

func TestPrepareModelLoadsPreferencesAndLocaleWithoutWriting(t *testing.T) {
	t.Run("missing Chinese locale", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "config", "aiah", "preferences.json")
		model, err := prepareModel(
			Options{Home: t.TempDir(), ConfigPath: path},
			localeGetter("zh_CN.UTF-8"),
			true,
		)
		if err != nil {
			t.Fatal(err)
		}
		if model.language != languageZhCN ||
			model.currentPreferences != preferences.Defaults() {
			t.Fatalf("model preferences = language %q document %#v", model.language, model.currentPreferences)
		}
		if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Fatalf("startup created the preference directory: %v", err)
		}
	})

	t.Run("missing non-Chinese locale", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config", "aiah", "preferences.json")
		model, err := prepareModel(
			Options{Home: t.TempDir(), ConfigPath: path},
			localeGetter("en_US.UTF-8"),
			true,
		)
		if err != nil {
			t.Fatal(err)
		}
		if model.language != languageEnglish {
			t.Fatalf("language = %q, want English", model.language)
		}
		if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Fatalf("startup created the preference directory: %v", err)
		}
	})

	t.Run("saved preference", func(t *testing.T) {
		store := testPreferenceStore(t)
		document := preferences.Defaults()
		document.Language = preferences.LanguageEnglish
		if _, err := preferences.Save(store, document); err != nil {
			t.Fatal(err)
		}
		before := snapshotTree(t, filepath.Dir(filepath.Dir(store.ConfigPath)))

		model, err := prepareModel(
			Options{Home: store.Home, ConfigPath: store.ConfigPath},
			localeGetter("zh_CN.UTF-8"),
			true,
		)
		if err != nil {
			t.Fatal(err)
		}

		if model.language != languageEnglish || model.currentPreferences != document {
			t.Fatalf("loaded model = language %q document %#v", model.language, model.currentPreferences)
		}
		after := snapshotTree(t, filepath.Dir(filepath.Dir(store.ConfigPath)))
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("preference loading wrote the store:\nbefore=%#v\nafter=%#v", before, after)
		}
	})
}

func TestLanguageOverrideWinsWithoutChangingStoredPreference(t *testing.T) {
	store := testPreferenceStore(t)
	document := preferences.Defaults()
	document.Language = preferences.LanguageZhCN
	if _, err := preferences.Save(store, document); err != nil {
		t.Fatal(err)
	}
	before := readPreferenceBytes(t, store.ConfigPath)

	model, err := prepareModel(
		Options{
			Home:       store.Home,
			ConfigPath: store.ConfigPath,
			Language:   preferences.LanguageEnglish,
		},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	if model.language != languageEnglish ||
		model.languageOverride != preferences.LanguageEnglish {
		t.Fatalf("override model = language %q override %q", model.language, model.languageOverride)
	}
	if after := readPreferenceBytes(t, store.ConfigPath); !reflect.DeepEqual(after, before) {
		t.Fatal("process-local language override changed the preference file")
	}
}

func TestSettingsPreviewCancelAndExplicitSaveBoundary(t *testing.T) {
	store := testPreferenceStoreWithoutDirectory(t)
	model, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	model = openSettings(t, model)
	model.settingsCursor = 2

	updated, command := model.Update(keyPress("enter"))
	model = updated.(Model)
	if command != nil || model.language != languageEnglish || !model.settingsDirty {
		t.Fatalf(
			"preview = language %q dirty %v command nil=%v",
			model.language,
			model.settingsDirty,
			command == nil,
		)
	}
	if _, err := os.Stat(store.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("language preview wrote the preference file: %v", err)
	}

	updated, command = model.Update(keyPress("esc"))
	model = updated.(Model)
	if command != nil || model.screen != screenHome || model.language != languageZhCN {
		t.Fatalf("cancel = screen %d language %q command nil=%v", model.screen, model.language, command == nil)
	}
	if _, err := os.Stat(store.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("cancel wrote the preference file: %v", err)
	}

	model = openSettings(t, model)
	model.settingsCursor = 2
	updated, _ = model.Update(keyPress("enter"))
	model = updated.(Model)
	model.settingsCursor = 3
	updated, command = model.Update(keyPress("enter"))
	model = updated.(Model)
	if command == nil || !model.settingsSaving {
		t.Fatalf("explicit save did not start: command nil=%v saving=%v", command == nil, model.settingsSaving)
	}
	if _, err := os.Stat(store.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("save command wrote before execution: %v", err)
	}

	updated, _ = model.Update(command())
	model = updated.(Model)
	report := preferences.Load(store)
	if model.settingsSaving || model.settingsDirty || model.settingsErr != nil ||
		model.language != languageEnglish ||
		report.Preferences.Language != preferences.LanguageEnglish {
		t.Fatalf("saved model = %#v report = %#v", model, report)
	}
	if !strings.Contains(model.View(), "偏好已保存") &&
		!strings.Contains(model.View(), "Preferences saved") {
		t.Fatalf("save result is not visible:\n%s", model.View())
	}
}

func TestSettingsSavePreservesPreferencesNotEditedInN73(t *testing.T) {
	store := testPreferenceStore(t)
	library := filepath.Join(store.Home, "assets")
	if err := os.Mkdir(library, 0o755); err != nil {
		t.Fatal(err)
	}
	document := preferences.Defaults()
	document.Language = preferences.LanguageZhCN
	document.Density = preferences.DensityDetailed
	document.PreferredAssetLibrary = library
	if _, err := preferences.Save(store, document); err != nil {
		t.Fatal(err)
	}
	model, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	model = openSettings(t, model)
	model.settingsCursor = 2
	updated, _ := model.Update(keyPress("enter"))
	model = updated.(Model)
	model.settingsCursor = 3
	updated, command := model.Update(keyPress("enter"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)

	report := preferences.Load(store)
	if report.Preferences.Language != preferences.LanguageEnglish ||
		report.Preferences.Density != preferences.DensityDetailed ||
		report.Preferences.PreferredAssetLibrary != library {
		t.Fatalf("language save changed hidden preferences: %#v", report.Preferences)
	}
}

func TestSettingsSaveFailureRestoresEffectivePreferences(t *testing.T) {
	root := t.TempDir()
	configDirectory := filepath.Join(root, "aiah")
	if err := os.Mkdir(configDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	store := preferences.StoreOptions{
		ConfigPath: filepath.Join(configDirectory, "preferences.json"),
		Home:       t.TempDir(),
	}
	model, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	model = openSettings(t, model)
	model.settingsCursor = 2
	updated, _ := model.Update(keyPress("enter"))
	model = updated.(Model)
	if model.language != languageEnglish {
		t.Fatalf("preview language = %q, want English", model.language)
	}
	model.settingsCursor = 3
	updated, command := model.Update(keyPress("enter"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)

	if model.language != languageZhCN || model.settingsErr == nil ||
		model.settingsDraft != model.currentPreferences || model.settingsDirty {
		t.Fatalf(
			"failed save did not restore state: language=%q err=%v dirty=%v draft=%#v current=%#v",
			model.language,
			model.settingsErr,
			model.settingsDirty,
			model.settingsDraft,
			model.currentPreferences,
		)
	}
	if !strings.Contains(model.View(), "保存失败") {
		t.Fatalf("save failure is not visible:\n%s", model.View())
	}
	if _, err := os.Stat(store.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("failed save created a preference file: %v", err)
	}
}

func TestSettingsResetRepairsStalePreferredLibraryOnlyAfterSave(t *testing.T) {
	store := testPreferenceStore(t)
	library := filepath.Join(store.Home, "assets")
	if err := os.Mkdir(library, 0o755); err != nil {
		t.Fatal(err)
	}
	document := preferences.Defaults()
	document.Language = preferences.LanguageEnglish
	document.Density = preferences.DensityDetailed
	document.PreferredAssetLibrary = library
	if _, err := preferences.Save(store, document); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(library); err != nil {
		t.Fatal(err)
	}
	before := readPreferenceBytes(t, store.ConfigPath)
	model, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(
		model.preferenceWarnings,
		[]preferences.WarningCode{preferences.WarningPreferredLibrary},
	) {
		t.Fatalf("warnings = %v", model.preferenceWarnings)
	}
	model = openSettings(t, model)
	model.settingsCursor = 4
	updated, _ := model.Update(keyPress("enter"))
	model = updated.(Model)
	if model.settingsDraft != preferences.Defaults() || !model.settingsDirty {
		t.Fatalf("reset draft = %#v dirty=%v", model.settingsDraft, model.settingsDirty)
	}
	if after := readPreferenceBytes(t, store.ConfigPath); !reflect.DeepEqual(after, before) {
		t.Fatal("reset preview changed the preference file")
	}
	model.settingsCursor = 3
	updated, command := model.Update(keyPress("enter"))
	model = updated.(Model)
	updated, _ = model.Update(command())
	model = updated.(Model)

	report := preferences.Load(store)
	if !reflect.DeepEqual(report.Preferences, preferences.Defaults()) ||
		len(report.Warnings) != 0 || len(model.preferenceWarnings) != 0 {
		t.Fatalf("reset report = %#v model warnings = %v", report, model.preferenceWarnings)
	}
}

func TestPreferenceWarningIsVisibleFromHomeAndSettings(t *testing.T) {
	store := testPreferenceStore(t)
	if _, err := preferences.Save(store, preferences.Defaults()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.ConfigPath, []byte(`{"schemaVersion":`), 0o600); err != nil {
		t.Fatal(err)
	}
	model, err := prepareModel(
		Options{Home: store.Home, ConfigPath: store.ConfigPath},
		localeGetter("zh_CN.UTF-8"),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(model.View(), "偏好文件需要处理") {
		t.Fatalf("home warning missing:\n%s", model.View())
	}
	model = openSettings(t, model)
	if !strings.Contains(model.View(), "格式、版本或取值无效") {
		t.Fatalf("settings warning detail missing:\n%s", model.View())
	}
}

func TestSettingsViewGoldenByLanguage(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		golden string
	}{
		{name: "zh-CN", locale: "zh_CN.UTF-8", golden: "settings.zh-CN.golden"},
		{name: "en", locale: "en_US.UTF-8", golden: "settings.en.golden"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := preferences.LoadReport{
				Preferences: preferences.Defaults(),
				ConfigPath:  "/tmp/aiah/preferences.json",
				Valid:       true,
				Warnings:    []preferences.WarningCode{},
			}
			locale := preferences.LocaleEnvironment{Lang: test.locale}
			effective, err := preferences.Resolve(preferences.ResolveOptions{
				Current: report.Preferences,
				Locale:  locale,
			})
			if err != nil {
				t.Fatal(err)
			}
			model := NewModel(inventory.Options{}).
				WithHome(true).
				WithMaintenance(true).
				withPreferences(
					preferences.StoreOptions{
						ConfigPath: report.ConfigPath,
					},
					report,
					locale,
					"",
					effective,
				)
			model = openSettings(t, model)
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

func openSettings(t *testing.T, model Model) Model {
	t.Helper()
	model.homeCursor = homeActionIndex(t, homeActionSettings)
	updated, command := model.Update(keyPress("enter"))
	next := updated.(Model)
	if command != nil || next.screen != screenSettings {
		t.Fatalf("open settings = screen %d command nil=%v", next.screen, command == nil)
	}
	return next
}

func testPreferenceStore(t *testing.T) preferences.StoreOptions {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	return preferences.StoreOptions{
		ConfigPath: filepath.Join(root, "config", "aiah", "preferences.json"),
		Home:       home,
	}
}

func testPreferenceStoreWithoutDirectory(t *testing.T) preferences.StoreOptions {
	t.Helper()
	return testPreferenceStore(t)
}

func readPreferenceBytes(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func localeGetter(language string) func(string) string {
	return func(name string) string {
		if name == "LANG" {
			return language
		}
		return ""
	}
}
