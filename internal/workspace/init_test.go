package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitScaffoldsAValidatableWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ai-assets")
	report, err := Init(InitOptions{Directory: root})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !report.Ok || report.Name != "ai-assets" || report.Version != DefaultInitVersion {
		t.Fatalf("report = %#v", report)
	}

	// The scaffold must parse as a manifest with no hand editing; a starter
	// workspace whose first command fails is worse than no starter at all.
	document, _, err := LoadManifest(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatalf("generated manifest does not load: %v", err)
	}
	if document.SchemaVersion != 1 || document.Name != "ai-assets" {
		t.Fatalf("document = %#v", document)
	}
	if _, ok := document.Profiles[DefaultInitProfile]; !ok {
		t.Fatalf("profile %q missing from %#v", DefaultInitProfile, document.Profiles)
	}

	for _, container := range assetContainers {
		path := filepath.Join(root, "assets", container)
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("assets/%s is not a directory: %v", container, err)
		}
	}
	if len(report.Created) != len(assetContainers)+2 { // containers + assets/ + manifest.yaml
		t.Fatalf("created = %v", report.Created)
	}
}

func TestInitIsIdempotentAndNeverRewritesTheManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ai-assets")
	if _, err := Init(InitOptions{Directory: root}); err != nil {
		t.Fatalf("first init: %v", err)
	}

	// A manifest is the source of truth for someone's whole library. Once it
	// exists, a scaffold has nothing worth losing it for.
	manifestPath := filepath.Join(root, "manifest.yaml")
	edited := "schemaVersion: 1\nname: edited\nversion: \"9.9.9\"\nassets: []\nprofiles:\n  work:\n    include: []\n"
	if err := os.WriteFile(manifestPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Init(InitOptions{Directory: root})
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if !report.Ok || len(report.Created) != 0 {
		t.Fatalf("second init created %v", report.Created)
	}
	body, err := os.ReadFile(manifestPath)
	if err != nil || string(body) != edited {
		t.Fatalf("the edited manifest was rewritten: %q", body)
	}
	found := false
	for _, entry := range report.Existing {
		if entry == "manifest.yaml" {
			found = true
		}
	}
	if !found {
		t.Fatalf("existing = %v, want manifest.yaml reported", report.Existing)
	}
}

func TestInitRefusesOccupiedPathsAndWritesNothing(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, root string)
		// wantMessage anchors the actionable half of the refusal. Dropping the
		// symlink branch would still fail closed via the "not a directory"
		// check, so only the message proves the specific guidance survives.
		wantMessage string
	}{
		{
			name:        "assets is a regular file",
			wantMessage: "is not a directory",
			prepare: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "assets"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:        "a container is a regular file",
			wantMessage: "is not a directory",
			prepare: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "assets", "skills"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:        "a container is a symlink",
			wantMessage: "is a symlink",
			prepare: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
					t.Fatal(err)
				}
				outside := t.TempDir()
				if err := os.Symlink(outside, filepath.Join(root, "assets", "skills")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:        "manifest is a symlink",
			wantMessage: "manifest.yaml is a symlink",
			prepare: func(t *testing.T, root string) {
				outside := filepath.Join(t.TempDir(), "real.yaml")
				if err := os.WriteFile(outside, []byte("schemaVersion: 1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "manifest.yaml")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "ai-assets")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			testCase.prepare(t, root)
			before := listTree(t, root)

			_, err := Init(InitOptions{Directory: root})
			if !errors.Is(err, ErrInitBlocked) {
				t.Fatalf("err = %v, want ErrInitBlocked", err)
			}
			if !strings.Contains(err.Error(), testCase.wantMessage) {
				t.Fatalf("message %q does not contain %q", err.Error(), testCase.wantMessage)
			}
			if after := listTree(t, root); after != before {
				t.Fatalf("a refused init wrote:\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestInitRejectsManagedToolDirectoriesAndWritesNothing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	for _, boundary := range []string{home, project, t.TempDir()} {
		for _, name := range managedToolDirectories {
			candidate := filepath.Join(boundary, name, "library")
			_, err := Init(InitOptions{
				Directory: candidate,
				Home:      home,
				Project:   project,
			})
			if !errors.Is(err, ErrInitBlocked) {
				t.Fatalf("%s under %s error = %v, want ErrInitBlocked", name, boundary, err)
			}
			if !strings.Contains(err.Error(), "overlaps a managed tool directory") {
				t.Fatalf("managed directory error = %q", err)
			}
			if _, statErr := os.Stat(filepath.Join(boundary, name)); !os.IsNotExist(statErr) {
				t.Fatalf("rejected managed directory %s was created: %v", name, statErr)
			}
		}
	}

	sibling := filepath.Join(home, ".claude-workspace")
	if _, err := Init(InitOptions{Directory: sibling, Home: home, Project: project}); err != nil {
		t.Fatalf("safe sibling was rejected: %v", err)
	}
}

func TestInitRejectsSymlinkIntoManagedToolDirectory(t *testing.T) {
	home := t.TempDir()
	managed := filepath.Join(home, ".claude")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(managed, link); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(link, "library")
	before := listTree(t, home)
	_, err := Init(InitOptions{Directory: candidate, Home: home})
	if !errors.Is(err, ErrInitBlocked) {
		t.Fatalf("symlinked managed path error = %v, want ErrInitBlocked", err)
	}
	if after := listTree(t, home); after != before {
		t.Fatalf("rejected symlink path wrote:\nbefore=%s\nafter=%s", before, after)
	}

	outside := t.TempDir()
	target := t.TempDir()
	managedLink := filepath.Join(outside, ".grok")
	if err := os.Symlink(target, managedLink); err != nil {
		t.Fatal(err)
	}
	_, err = Init(InitOptions{Directory: filepath.Join(managedLink, "library"), Home: home})
	if !errors.Is(err, ErrInitBlocked) {
		t.Fatalf("named managed symlink error = %v, want ErrInitBlocked", err)
	}
	if _, statErr := os.Stat(filepath.Join(target, "library")); !os.IsNotExist(statErr) {
		t.Fatalf("rejected named symlink wrote into its target: %v", statErr)
	}
}

func TestInitDerivesAndValidatesTheManifestName(t *testing.T) {
	cases := []struct {
		directory string
		want      string
	}{
		{"My AI Assets", "my-ai-assets"},
		{"ai_assets", "ai-assets"},
		{"Team--Shared", "team-shared"},
		{"2026.assets", "2026-assets"},
	}
	for _, testCase := range cases {
		t.Run(testCase.directory, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), testCase.directory)
			report, err := Init(InitOptions{Directory: root})
			if err != nil {
				t.Fatalf("init: %v", err)
			}
			if report.Name != testCase.want {
				t.Fatalf("name = %q, want %q", report.Name, testCase.want)
			}
			if !namePattern.MatchString(report.Name) {
				t.Fatalf("derived name %q does not match the schema pattern", report.Name)
			}
		})
	}
}

func TestInitRejectsUnusableNames(t *testing.T) {
	// A name that cannot be derived must be asked for, not guessed: it lands in
	// the manifest and the package filename, where a wrong guess is permanent.
	root := filepath.Join(t.TempDir(), "...")
	if _, err := Init(InitOptions{Directory: root}); !errors.Is(err, ErrInitBlocked) {
		t.Fatalf("err = %v, want ErrInitBlocked", err)
	}
	// An explicit name still has to satisfy the manifest schema.
	if _, err := Init(InitOptions{
		Directory: filepath.Join(t.TempDir(), "ws"), Name: "Not Valid",
	}); !errors.Is(err, ErrInitBlocked) {
		t.Fatalf("err = %v, want ErrInitBlocked", err)
	}
}

func TestInitRejectsVersionsThatCannotBeEmittedOrPublished(t *testing.T) {
	// The version is written into a double-quoted YAML scalar and later becomes
	// a path segment under packages/<name>/<version>/<profile>/. An unchecked
	// value produced a manifest that would not parse while init still reported
	// success, which is the one thing a scaffold must never do.
	for _, bad := range []string{`a"b`, "x\nname: hijacked", "has space", "-leading", "sla/sh"} {
		root := filepath.Join(t.TempDir(), "ws")
		_, err := Init(InitOptions{Directory: root, Version: bad})
		if !errors.Is(err, ErrInitBlocked) {
			t.Fatalf("version %q was accepted (err=%v)", bad, err)
		}
		if _, statErr := os.Stat(filepath.Join(root, "manifest.yaml")); statErr == nil {
			t.Fatalf("version %q was refused but a manifest was still written", bad)
		}
	}
	// A realistic dated version must still be accepted.
	root := filepath.Join(t.TempDir(), "ws")
	report, err := Init(InitOptions{Directory: root, Version: "2026.08.1"})
	if err != nil || report.Version != "2026.08.1" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if _, _, err := LoadManifest(filepath.Join(root, "manifest.yaml")); err != nil {
		t.Fatalf("dated version produced an unparseable manifest: %v", err)
	}
}

func TestInitIsDeterministic(t *testing.T) {
	// Two runs must produce identical bytes, so the scaffold never shows up as a
	// spurious diff and never depends on the clock.
	first := filepath.Join(t.TempDir(), "ws")
	second := filepath.Join(t.TempDir(), "ws")
	for _, root := range []string{first, second} {
		if _, err := Init(InitOptions{Directory: root}); err != nil {
			t.Fatalf("init %s: %v", root, err)
		}
	}
	left, err := os.ReadFile(filepath.Join(first, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.ReadFile(filepath.Join(second, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(left) != string(right) {
		t.Fatal("two scaffolds produced different manifests")
	}
}

func TestInitRequiresADirectory(t *testing.T) {
	if _, err := Init(InitOptions{}); !errors.Is(err, ErrInitBlocked) {
		t.Fatalf("err = %v, want ErrInitBlocked", err)
	}
}

func listTree(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, relative)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return strings.Join(entries, "\n")
}
