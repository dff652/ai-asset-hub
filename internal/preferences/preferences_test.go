package preferences

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultsAreNonBusinessAndSafe(t *testing.T) {
	want := Document{
		SchemaVersion: 1,
		Language:      LanguageAuto,
		Density:       DensityStandard,
	}
	if got := Defaults(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Defaults() = %#v, want %#v", got, want)
	}
}

func TestDefaultPathUsesOperatorConfigDirectory(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configRoot, "aiah", "preferences.json")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(configRoot, "aiah")); !os.IsNotExist(err) {
		t.Fatalf("DefaultPath created a directory: %v", err)
	}
}

func TestLoadMissingUsesDefaultsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "not-created", "preferences.json")
	report := Load(StoreOptions{ConfigPath: path})
	if report.Found || !report.Valid || len(report.Warnings) != 0 {
		t.Fatalf("missing report = %#v", report)
	}
	if !reflect.DeepEqual(report.Preferences, Defaults()) {
		t.Fatalf("missing preferences = %#v, want defaults", report.Preferences)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("Load created the config directory: %v", err)
	}
}

func TestLoadStrictDocumentFailuresUseDefaultsAndPreserveBytes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{"schemaVersion":`},
		{
			name: "unknown field",
			body: `{"schemaVersion":1,"language":"auto","density":"standard","recent":[]}`,
		},
		{
			name: "wrong version",
			body: `{"schemaVersion":2,"language":"auto","density":"standard"}`,
		},
		{
			name: "bad language",
			body: `{"schemaVersion":1,"language":"fr","density":"standard"}`,
		},
		{
			name: "bad density",
			body: `{"schemaVersion":1,"language":"auto","density":"compact"}`,
		},
		{
			name: "relative library",
			body: `{"schemaVersion":1,"language":"auto","density":"standard","preferredAssetLibrary":"assets"}`,
		},
		{
			name: "trailing JSON",
			body: `{"schemaVersion":1,"language":"auto","density":"standard"} {}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := newStore(t)
			writePreferenceFile(t, options.ConfigPath, []byte(test.body), 0o600)
			before := readFile(t, options.ConfigPath)

			report := Load(options)

			if !report.Found || report.Valid ||
				!reflect.DeepEqual(report.Warnings, []WarningCode{WarningInvalidDocument}) {
				t.Fatalf("invalid report = %#v", report)
			}
			if !reflect.DeepEqual(report.Preferences, Defaults()) {
				t.Fatalf("invalid preferences = %#v, want defaults", report.Preferences)
			}
			if after := readFile(t, options.ConfigPath); !reflect.DeepEqual(after, before) {
				t.Fatal("Load rewrote the invalid preference document")
			}
		})
	}
}

func TestLoadRejectsSymlinkAndOverbroadModes(t *testing.T) {
	t.Run("file symlink", func(t *testing.T) {
		options := newStore(t)
		target := filepath.Join(t.TempDir(), "target.json")
		writePreferenceFile(t, target, validBody(LanguageEnglish, DensityDetailed, ""), 0o600)
		if err := os.Symlink(target, options.ConfigPath); err != nil {
			t.Fatal(err)
		}
		report := Load(options)
		assertUnsafeLoad(t, report)
	})

	t.Run("directory symlink", func(t *testing.T) {
		root := t.TempDir()
		actual := filepath.Join(root, "actual")
		if err := os.Mkdir(actual, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(actual, "preferences.json")
		writePreferenceFile(t, path, validBody(LanguageEnglish, DensityDetailed, ""), 0o600)
		link := filepath.Join(root, "linked-config")
		if err := os.Symlink(actual, link); err != nil {
			t.Fatal(err)
		}
		report := Load(StoreOptions{ConfigPath: filepath.Join(link, "preferences.json")})
		assertUnsafeLoad(t, report)
	})

	t.Run("file mode", func(t *testing.T) {
		options := newStore(t)
		writePreferenceFile(
			t,
			options.ConfigPath,
			validBody(LanguageEnglish, DensityDetailed, ""),
			0o644,
		)
		assertUnsafeLoad(t, Load(options))
	})

	t.Run("directory mode", func(t *testing.T) {
		options := newStore(t)
		writePreferenceFile(
			t,
			options.ConfigPath,
			validBody(LanguageEnglish, DensityDetailed, ""),
			0o600,
		)
		if err := os.Chmod(filepath.Dir(options.ConfigPath), 0o755); err != nil {
			t.Fatal(err)
		}
		assertUnsafeLoad(t, Load(options))
	})
}

func TestLoadValidDocumentAndWarnsWithoutCreatingStaleLibrary(t *testing.T) {
	options := newStore(t)
	missingLibrary := filepath.Join(options.Home, "missing-assets")
	document := Document{
		SchemaVersion:         1,
		Language:              LanguageEnglish,
		Density:               DensityDetailed,
		PreferredAssetLibrary: missingLibrary,
	}
	writePreferenceFile(
		t,
		options.ConfigPath,
		validBody(document.Language, document.Density, document.PreferredAssetLibrary),
		0o600,
	)

	report := Load(options)

	if !report.Found || !report.Valid ||
		!reflect.DeepEqual(report.Warnings, []WarningCode{WarningPreferredLibrary}) {
		t.Fatalf("stale library report = %#v", report)
	}
	if !reflect.DeepEqual(report.Preferences, document) {
		t.Fatalf("stale library preferences = %#v, want %#v", report.Preferences, document)
	}
	if _, err := os.Stat(missingLibrary); !os.IsNotExist(err) {
		t.Fatalf("Load created the stale preferred library: %v", err)
	}
}

func TestSaveCreatesPrivateStoreAndNormalizesPreferredLibrary(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	actualLibrary := filepath.Join(home, "asset-library")
	if err := os.Mkdir(actualLibrary, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "library-link")
	if err := os.Symlink(actualLibrary, link); err != nil {
		t.Fatal(err)
	}
	options := StoreOptions{
		ConfigPath: filepath.Join(root, "config", "aiah", "preferences.json"),
		Home:       home,
	}
	document := Document{
		SchemaVersion:         1,
		Language:              LanguageZhCN,
		Density:               DensityDetailed,
		PreferredAssetLibrary: link,
	}

	normalized, err := Save(options, document)
	if err != nil {
		t.Fatal(err)
	}

	if normalized.PreferredAssetLibrary != actualLibrary {
		t.Fatalf(
			"normalized library = %q, want %q",
			normalized.PreferredAssetLibrary,
			actualLibrary,
		)
	}
	assertMode(t, filepath.Dir(options.ConfigPath), 0o700)
	assertMode(t, options.ConfigPath, 0o600)
	report := Load(options)
	if !report.Valid || !reflect.DeepEqual(report.Preferences, normalized) ||
		len(report.Warnings) != 0 {
		t.Fatalf("saved report = %#v, want %#v", report, normalized)
	}
	body := string(readFile(t, options.ConfigPath))
	if strings.Contains(body, "library-link") || !strings.HasSuffix(body, "\n") {
		t.Fatalf("saved body is not normalized JSON: %q", body)
	}
}

func TestSaveRejectsInvalidOrUnsafeDocumentsBeforeWriting(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(t *testing.T, options StoreOptions, document *Document)
		wantKind error
	}{
		{
			name: "wrong schema",
			mutate: func(_ *testing.T, _ StoreOptions, document *Document) {
				document.SchemaVersion = 2
			},
			wantKind: ErrInvalidPreferences,
		},
		{
			name: "invalid enum",
			mutate: func(_ *testing.T, _ StoreOptions, document *Document) {
				document.Language = Language("fr")
			},
			wantKind: ErrInvalidPreferences,
		},
		{
			name: "tilde library",
			mutate: func(_ *testing.T, _ StoreOptions, document *Document) {
				document.PreferredAssetLibrary = "~/assets"
			},
			wantKind: ErrInvalidPreferences,
		},
		{
			name: "missing library",
			mutate: func(_ *testing.T, options StoreOptions, document *Document) {
				document.PreferredAssetLibrary = filepath.Join(options.Home, "missing")
			},
			wantKind: ErrInvalidPreferences,
		},
		{
			name: "managed tool library",
			mutate: func(t *testing.T, options StoreOptions, document *Document) {
				managed := filepath.Join(options.Home, ".codex")
				if err := os.Mkdir(managed, 0o755); err != nil {
					t.Fatal(err)
				}
				document.PreferredAssetLibrary = managed
			},
			wantKind: ErrInvalidPreferences,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := newStore(t)
			document := Defaults()
			test.mutate(t, options, &document)

			_, err := Save(options, document)

			if !errors.Is(err, test.wantKind) {
				t.Fatalf("Save error = %v, want %v", err, test.wantKind)
			}
			if _, err := os.Stat(options.ConfigPath); !os.IsNotExist(err) {
				t.Fatalf("rejected save created a preference file: %v", err)
			}
		})
	}
}

func TestSaveUsesOperatorHomeWhenSafetyBoundaryIsNotInjected(t *testing.T) {
	operatorHome := t.TempDir()
	t.Setenv("HOME", operatorHome)
	managed := filepath.Join(operatorHome, ".claude")
	if err := os.Mkdir(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	configRoot := t.TempDir()
	options := StoreOptions{
		ConfigPath: filepath.Join(configRoot, "aiah", "preferences.json"),
	}
	document := Defaults()
	document.PreferredAssetLibrary = managed

	if _, err := Save(options, document); !errors.Is(err, ErrInvalidPreferences) {
		t.Fatalf("Save error = %v, want ErrInvalidPreferences", err)
	}
	if _, err := os.Stat(filepath.Dir(options.ConfigPath)); !os.IsNotExist(err) {
		t.Fatalf("rejected save created the config directory: %v", err)
	}
}

func TestSaveRejectsSymlinkStoreWithoutChangingTargets(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		options := newStore(t)
		target := filepath.Join(t.TempDir(), "target.json")
		original := []byte("target-must-not-change\n")
		writePreferenceFile(t, target, original, 0o600)
		if err := os.Symlink(target, options.ConfigPath); err != nil {
			t.Fatal(err)
		}

		_, err := Save(options, Defaults())

		if !errors.Is(err, ErrStoreBlocked) {
			t.Fatalf("Save error = %v, want ErrStoreBlocked", err)
		}
		if got := readFile(t, target); !reflect.DeepEqual(got, original) {
			t.Fatal("symlink target changed")
		}
	})

	t.Run("directory", func(t *testing.T) {
		root := t.TempDir()
		targetDirectory := filepath.Join(root, "target")
		if err := os.Mkdir(targetDirectory, 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "aiah")
		if err := os.Symlink(targetDirectory, link); err != nil {
			t.Fatal(err)
		}

		_, err := Save(
			StoreOptions{ConfigPath: filepath.Join(link, "preferences.json")},
			Defaults(),
		)

		if !errors.Is(err, ErrStoreBlocked) {
			t.Fatalf("Save error = %v, want ErrStoreBlocked", err)
		}
		if entries, err := os.ReadDir(targetDirectory); err != nil || len(entries) != 0 {
			t.Fatalf("symlinked directory changed: entries=%v err=%v", entries, err)
		}
	})
}

func TestSaveRejectsOverbroadExistingStore(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		options := newStore(t)
		if err := os.Chmod(filepath.Dir(options.ConfigPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Save(options, Defaults()); !errors.Is(err, ErrStoreBlocked) {
			t.Fatalf("Save error = %v, want ErrStoreBlocked", err)
		}
	})

	t.Run("file", func(t *testing.T) {
		options := newStore(t)
		original := validBody(LanguageEnglish, DensityDetailed, "")
		writePreferenceFile(t, options.ConfigPath, original, 0o644)
		if _, err := Save(options, Defaults()); !errors.Is(err, ErrStoreBlocked) {
			t.Fatalf("Save error = %v, want ErrStoreBlocked", err)
		}
		if got := readFile(t, options.ConfigPath); !reflect.DeepEqual(got, original) {
			t.Fatal("overbroad existing file changed")
		}
	})
}

func TestSaveInterruptedBeforeRenamePreservesOldFileAndCleansStage(t *testing.T) {
	options := newStore(t)
	oldDocument := Defaults()
	if _, err := Save(options, oldDocument); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, options.ConfigPath)
	newDocument := Defaults()
	newDocument.Language = LanguageEnglish
	newDocument.Density = DensityDetailed
	interrupted := errors.New("injected interruption")
	hookCalled := false

	_, err := save(options, newDocument, &saveHooks{
		beforeRename: func(temporaryPath, targetPath string) error {
			hookCalled = true
			assertMode(t, temporaryPath, 0o600)
			if targetPath != options.ConfigPath {
				t.Fatalf("target = %q, want %q", targetPath, options.ConfigPath)
			}
			staged := readFile(t, temporaryPath)
			if !strings.Contains(string(staged), `"language": "en"`) {
				t.Fatalf("staged document is incomplete: %q", staged)
			}
			if current := readFile(t, targetPath); !reflect.DeepEqual(current, before) {
				t.Fatal("target changed before atomic rename")
			}
			return interrupted
		},
	})

	if !hookCalled || !errors.Is(err, ErrStoreBlocked) {
		t.Fatalf("interrupted save = (called=%v, err=%v)", hookCalled, err)
	}
	if after := readFile(t, options.ConfigPath); !reflect.DeepEqual(after, before) {
		t.Fatal("interrupted save changed the old preference file")
	}
	assertNoStageFiles(t, filepath.Dir(options.ConfigPath))
}

func TestSaveAtomicallyReplacesCompleteDocument(t *testing.T) {
	options := newStore(t)
	if _, err := Save(options, Defaults()); err != nil {
		t.Fatal(err)
	}
	replacement := Defaults()
	replacement.Language = LanguageEnglish
	replacement.Density = DensityDetailed

	got, err := Save(options, replacement)

	if err != nil || !reflect.DeepEqual(got, replacement) {
		t.Fatalf("Save = (%#v, %v), want (%#v, nil)", got, err, replacement)
	}
	report := Load(options)
	if !report.Valid || !reflect.DeepEqual(report.Preferences, replacement) {
		t.Fatalf("replacement report = %#v", report)
	}
	assertMode(t, options.ConfigPath, 0o600)
	assertNoStageFiles(t, filepath.Dir(options.ConfigPath))
}

func TestResolveUsesOverrideCurrentAndInjectedLocalePrecedence(t *testing.T) {
	tests := []struct {
		name             string
		currentLanguage  Language
		overrideLanguage Language
		locale           LocaleEnvironment
		want             Language
	}{
		{
			name:            "LC_ALL Chinese",
			currentLanguage: LanguageAuto,
			locale: LocaleEnvironment{
				LCAll: "zh_CN.UTF-8", LCMessages: "en_US.UTF-8", Lang: "en_US.UTF-8",
			},
			want: LanguageZhCN,
		},
		{
			name:            "LC_ALL wins",
			currentLanguage: LanguageAuto,
			locale: LocaleEnvironment{
				LCAll: "C", LCMessages: "zh_CN.UTF-8", Lang: "zh_CN.UTF-8",
			},
			want: LanguageEnglish,
		},
		{
			name:            "LC_MESSAGES fallback",
			currentLanguage: LanguageAuto,
			locale:          LocaleEnvironment{LCMessages: "zh-TW"},
			want:            LanguageZhCN,
		},
		{
			name:            "LANG fallback",
			currentLanguage: LanguageAuto,
			locale:          LocaleEnvironment{Lang: "zh"},
			want:            LanguageZhCN,
		},
		{
			name:            "unknown locale English",
			currentLanguage: LanguageAuto,
			locale:          LocaleEnvironment{Lang: "de_DE.UTF-8"},
			want:            LanguageEnglish,
		},
		{
			name:            "stored language ignores locale",
			currentLanguage: LanguageEnglish,
			locale:          LocaleEnvironment{LCAll: "zh_CN.UTF-8"},
			want:            LanguageEnglish,
		},
		{
			name:             "override wins",
			currentLanguage:  LanguageEnglish,
			overrideLanguage: LanguageZhCN,
			locale:           LocaleEnvironment{LCAll: "en_US.UTF-8"},
			want:             LanguageZhCN,
		},
		{
			name:             "auto override uses locale",
			currentLanguage:  LanguageEnglish,
			overrideLanguage: LanguageAuto,
			locale:           LocaleEnvironment{LCAll: "zh_CN.UTF-8"},
			want:             LanguageZhCN,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := Defaults()
			current.Language = test.currentLanguage
			current.Density = DensityDetailed
			current.PreferredAssetLibrary = "/example/assets"
			effective, err := Resolve(ResolveOptions{
				Current:          current,
				Locale:           test.locale,
				LanguageOverride: test.overrideLanguage,
				DensityOverride:  DensityStandard,
			})
			if err != nil {
				t.Fatal(err)
			}
			if effective.Language != test.want ||
				effective.Density != DensityStandard ||
				effective.PreferredAssetLibrary != current.PreferredAssetLibrary {
				t.Fatalf("Resolve = %#v, want language %q and injected current values", effective, test.want)
			}
		})
	}
}

func TestResolveRejectsInvalidInjectedState(t *testing.T) {
	tests := []ResolveOptions{
		{Current: Document{}},
		{Current: Defaults(), LanguageOverride: Language("fr")},
		{Current: Defaults(), DensityOverride: Density("compact")},
	}
	for _, options := range tests {
		if _, err := Resolve(options); !errors.Is(err, ErrInvalidPreferences) {
			t.Fatalf("Resolve(%#v) error = %v, want ErrInvalidPreferences", options, err)
		}
	}
}

func newStore(t *testing.T) StoreOptions {
	t.Helper()
	root := t.TempDir()
	configDirectory := filepath.Join(root, "aiah")
	if err := os.Mkdir(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	return StoreOptions{
		ConfigPath: filepath.Join(configDirectory, "preferences.json"),
		Home:       home,
	}
}

func validBody(language Language, density Density, library string) []byte {
	document := `{"schemaVersion":1,"language":"` + string(language) +
		`","density":"` + string(density) + `"`
	if library != "" {
		document += `,"preferredAssetLibrary":"` + filepath.ToSlash(library) + `"`
	}
	return []byte(document + "}\n")
}

func writePreferenceFile(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func assertUnsafeLoad(t *testing.T, report LoadReport) {
	t.Helper()
	if report.Valid ||
		!reflect.DeepEqual(report.Warnings, []WarningCode{WarningUnsafeStore}) ||
		!reflect.DeepEqual(report.Preferences, Defaults()) {
		t.Fatalf("unsafe report = %#v", report)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}

func assertNoStageFiles(t *testing.T, directory string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(directory, ".preferences-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("staged preference files remain: %v", matches)
	}
}
