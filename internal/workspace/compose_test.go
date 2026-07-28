package workspace

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/inventory"
)

func TestComposeCopiesAssetsAndRegistersThem(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
		".claude/skills/review/notes.md": "notes\n",
	})
	root := t.TempDir()

	result, err := Compose(ComposeOptions{
		WorkspaceRoot: root,
		Home:          home,
		Name:          "ym-personal",
		Version:       "2026.07.1",
		Assets:        []inventory.Asset{multiFileSkillAsset("home/.claude/skills/review")},
	}, nil)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if !result.Ok || len(result.Registered) != 1 || result.Registered[0] != "skill.review" {
		t.Fatalf("result = %#v", result)
	}
	want := []string{"assets/skills/review/SKILL.md", "assets/skills/review/notes.md"}
	for _, relative := range want {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Fatalf("%s was not copied: %v", relative, err)
		}
	}

	document, _, err := LoadManifest(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatalf("load written manifest: %v", err)
	}
	if len(document.Assets) != 1 {
		t.Fatalf("assets = %#v", document.Assets)
	}
	asset := document.Assets[0]
	if asset.ID != "skill.review" || asset.Type != "skill" ||
		asset.Path != "assets/skills/review" || asset.Scope != "global" ||
		asset.Portability != "adapter-required" || asset.Sensitivity != "private" ||
		strings.Join(asset.Targets, ",") != "claude" {
		t.Fatalf("entry = %#v", asset)
	}
	if include := document.Profiles["personal"].Include; len(include) != 1 || include[0] != "skill.review" {
		t.Fatalf("profile include = %#v", document.Profiles)
	}
}

// TestComposeNeverWritesToolDirectories is the boundary test for ADR-0006 §2:
// the UI reads .claude/.codex/.grok and must never modify them.
func TestComposeNeverWritesToolDirectories(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md":      "# review\n",
		".codex/skills/lint/SKILL.md":         "# lint\n",
		".grok/skills/summarize/SKILL.md":     "# summarize\n",
		".agents/skills/feature-dev/SKILL.md": "# feature\n",
	})
	root := t.TempDir()
	before := snapshotTree(t, home)

	result, err := Compose(ComposeOptions{
		WorkspaceRoot: root,
		Home:          home,
		Assets: []inventory.Asset{
			skillAsset("home/.claude/skills/review"),
			codexSkillAsset("home/.codex/skills/lint"),
			sharedSkillAsset("home/.agents/skills/feature-dev"),
		},
	}, nil)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(result.Registered) != 3 {
		t.Fatalf("expected all three registered, got %#v", result)
	}
	if diff := treeDiff(before, snapshotTree(t, home)); diff != "" {
		t.Fatalf("compose modified the scanned home:\n%s", diff)
	}
}

func TestComposeRejectsManagedToolDirectoryAsWorkspace(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
	})
	managedRoot := filepath.Join(home, ".claude")
	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(managedRoot, link); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, managedRoot)

	for _, root := range []string{managedRoot, link} {
		result, err := Compose(ComposeOptions{
			WorkspaceRoot: root,
			Home:          home,
			Assets:        []inventory.Asset{skillAsset("home/.claude/skills/review")},
		}, nil)
		if !errors.Is(err, ErrComposeBlocked) || result.Ok {
			t.Fatalf("managed workspace %s result=%#v err=%v, want ErrComposeBlocked", root, result, err)
		}
	}
	if diff := treeDiff(before, snapshotTree(t, managedRoot)); diff != "" {
		t.Fatalf("managed tool directory changed:\n%s", diff)
	}
}

// TestComposeDoesNotOverwriteWorkspaceFiles anchors create-only (ADR-0006 §3).
func TestComposeDoesNotOverwriteWorkspaceFiles(t *testing.T) {
	home := fakeHome(t, map[string]string{".claude/skills/review/SKILL.md": "from home\n"})
	root := t.TempDir()
	existing := filepath.Join(root, "assets", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("hand written\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home,
		Assets: []inventory.Asset{skillAsset("home/.claude/skills/review")},
	}, nil)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	body, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hand written\n" {
		t.Fatalf("existing workspace file was overwritten: %q", body)
	}
	for _, created := range result.Created {
		if created == "assets/skills/review/SKILL.md" {
			t.Fatal("an untouched file was reported as created")
		}
	}
}

// TestComposeRollsBackWhenValidationFails anchors the transaction (§5): a
// rejected manifest must leave the workspace exactly as it was.
func TestComposeRollsBackWhenValidationFails(t *testing.T) {
	home := fakeHome(t, map[string]string{".claude/skills/review/SKILL.md": "# review\n"})
	root := t.TempDir()
	before := snapshotTree(t, root)

	_, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home,
		Assets: []inventory.Asset{skillAsset("home/.claude/skills/review")},
	}, func(string, string) error { return errors.New("validation says no") })
	if !errors.Is(err, ErrComposeBlocked) {
		t.Fatalf("err = %v, want ErrComposeBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, root)); diff != "" {
		t.Fatalf("failed compose left the workspace dirty:\n%s", diff)
	}
}

// TestComposeRollbackKeepsPreExistingFiles: rollback must remove only what this
// call created, never content that was already there.
func TestComposeRollbackKeepsPreExistingFiles(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
		".claude/skills/review/notes.md": "notes\n",
	})
	root := t.TempDir()
	kept := filepath.Join(root, "assets", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kept, []byte("hand written\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home,
		Assets: []inventory.Asset{multiFileSkillAsset("home/.claude/skills/review")},
	}, func(string, string) error { return errors.New("no") })
	if !errors.Is(err, ErrComposeBlocked) {
		t.Fatalf("err = %v", err)
	}
	body, err := os.ReadFile(kept)
	if err != nil {
		t.Fatalf("rollback deleted a pre-existing file: %v", err)
	}
	if string(body) != "hand written\n" {
		t.Fatalf("pre-existing file changed: %q", body)
	}
	if _, err := os.Stat(filepath.Join(root, "assets", "skills", "review", "notes.md")); !os.IsNotExist(err) {
		t.Fatal("rollback did not remove the file this call created")
	}
}

// TestComposePreservesCommentsAndUnknownFields anchors ADR-0006 §4.
func TestComposePreservesCommentsAndUnknownFields(t *testing.T) {
	home := fakeHome(t, map[string]string{".claude/skills/review/SKILL.md": "# review\n"})
	root := t.TempDir()
	original := `# hand written manifest, do not clobber
schemaVersion: 1
name: ym-personal
version: "2026.07.1"
futureField: keep-me   # a field this version does not know
assets: []
profiles:
  personal:
    include: []
`
	manifestPath := filepath.Join(root, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home,
		Assets: []inventory.Asset{skillAsset("home/.claude/skills/review")},
	}, nil); err != nil {
		t.Fatalf("compose: %v", err)
	}

	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	written := string(body)
	for _, needle := range []string{
		"# hand written manifest, do not clobber",
		"futureField: keep-me",
		"# a field this version does not know",
		"skill.review",
	} {
		if !strings.Contains(written, needle) {
			t.Fatalf("manifest lost %q:\n%s", needle, written)
		}
	}
}

func TestComposeFailsClosedOnUnusableManifestShape(t *testing.T) {
	home := fakeHome(t, map[string]string{".claude/skills/review/SKILL.md": "# review\n"})
	cases := map[string]string{
		"not a mapping":      "- just\n- a list\n",
		"assets not a list":  "schemaVersion: 1\nassets: {}\nprofiles: {}\n",
		"profiles not a map": "schemaVersion: 1\nassets: []\nprofiles: []\n",
		"include not a list": "schemaVersion: 1\nassets: []\nprofiles:\n  personal:\n    include: {}\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			manifestPath := filepath.Join(root, "manifest.yaml")
			if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			before := snapshotTree(t, root)
			_, err := Compose(ComposeOptions{
				WorkspaceRoot: root, Home: home,
				Assets: []inventory.Asset{skillAsset("home/.claude/skills/review")},
			}, nil)
			if !errors.Is(err, ErrComposeBlocked) {
				t.Fatalf("err = %v, want ErrComposeBlocked", err)
			}
			if diff := treeDiff(before, snapshotTree(t, root)); diff != "" {
				t.Fatalf("blocked compose still wrote:\n%s", diff)
			}
		})
	}
}

func TestComposeSkipsUnmappableAssets(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/keep/SKILL.md": "# keep\n",
		".claude/secret/token":         "sk-real\n",
	})
	root := t.TempDir()

	secret := skillAsset("home/.claude/secret")
	secret.Type = inventory.TypeCredential
	secret.Sensitivity = inventory.SensitivitySecret
	secret.Files = []string{"home/.claude/secret/token"}

	deviceScoped := skillAsset("home/.claude/skills/keep")
	deviceScoped.LogicalPath = "home/.claude/skills/device"
	deviceScoped.Scope = inventory.ScopeDevicePrivate

	result, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home,
		Assets: []inventory.Asset{skillAsset("home/.claude/skills/keep"), secret, deviceScoped},
	}, nil)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(result.Registered) != 1 || result.Registered[0] != "skill.keep" {
		t.Fatalf("registered = %#v", result.Registered)
	}
	codes := make(map[string]bool)
	for _, finding := range result.Findings {
		codes[finding.Code] = true
	}
	if !codes["asset_secret"] {
		t.Fatalf("a secret asset was not rejected: %#v", result.Findings)
	}
	if !codes["asset_device_scope"] {
		t.Fatalf("a device-scoped asset was not rejected: %#v", result.Findings)
	}
	// The secret's own bytes must not reach the workspace.
	if _, err := os.Stat(filepath.Join(root, "assets", "skills", "secret")); !os.IsNotExist(err) {
		t.Fatal("a rejected secret asset was copied into the workspace")
	}
}

func TestComposeRefusesToWriteOutsideTheWorkspace(t *testing.T) {
	home := fakeHome(t, map[string]string{".claude/skills/review/SKILL.md": "# review\n"})
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "manifest.yaml")

	_, err := Compose(ComposeOptions{
		WorkspaceRoot: root, ManifestPath: outside, Home: home,
		Assets: []inventory.Asset{skillAsset("home/.claude/skills/review")},
	}, nil)
	if !errors.Is(err, ErrComposeBlocked) {
		t.Fatalf("err = %v, want ErrComposeBlocked", err)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("compose wrote outside the workspace")
	}
}

func TestComposeSkipsAlreadyRegisteredIDs(t *testing.T) {
	home := fakeHome(t, map[string]string{".claude/skills/review/SKILL.md": "# review\n"})
	root := t.TempDir()
	asset := skillAsset("home/.claude/skills/review")

	if _, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil); err != nil {
		t.Fatalf("first compose: %v", err)
	}
	result, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil)
	if err != nil {
		t.Fatalf("second compose: %v", err)
	}
	if len(result.Registered) != 0 {
		t.Fatalf("a duplicate id was registered twice: %#v", result)
	}
	document, _, err := LoadManifest(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Assets) != 1 {
		t.Fatalf("manifest gained a duplicate entry: %#v", document.Assets)
	}
}

func TestDeriveID(t *testing.T) {
	cases := []struct{ manifestType, name, want string }{
		{"skill", "review", "skill.review"},
		{"skill", "feature-dev", "skill.feature-dev"},
		{"rules", "CLAUDE.md", "rules.claude"},
		{"hook", "pre_tool.sh", "hook.pre-tool"},
		{"agent", "Code Reviewer", "agent.code-reviewer"},
		{"skill", "...", ""},
	}
	for _, testCase := range cases {
		if got := deriveID(testCase.manifestType, testCase.name); got != testCase.want {
			t.Fatalf("deriveID(%q, %q) = %q, want %q",
				testCase.manifestType, testCase.name, got, testCase.want)
		}
	}
}

// --- helpers ---

func skillAsset(logicalPath string) inventory.Asset {
	return inventory.Asset{
		LogicalPath: logicalPath,
		Source:      inventory.SourceClaude,
		Scope:       inventory.ScopeGlobal,
		Type:        inventory.TypeSkill,
		Portability: inventory.PortabilityAdapterRequired,
		Sensitivity: inventory.SensitivityPrivate,
		Status:      inventory.AssetCandidate,
		Files:       []string{logicalPath + "/SKILL.md"},
	}
}

// multiFileSkillAsset covers assets whose tree has more than the entry file.
func multiFileSkillAsset(logicalPath string) inventory.Asset {
	asset := skillAsset(logicalPath)
	asset.Files = append(asset.Files, logicalPath+"/notes.md")
	return asset
}

func codexSkillAsset(logicalPath string) inventory.Asset {
	asset := skillAsset(logicalPath)
	asset.Source = inventory.SourceCodex
	return asset
}

func sharedSkillAsset(logicalPath string) inventory.Asset {
	asset := skillAsset(logicalPath)
	asset.Source = inventory.SourceShared
	return asset
}

func fakeHome(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for relative, body := range files {
		full := filepath.Join(home, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%04o", info.Mode().Type(), info.Mode().Perm())
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(body))
		}
		snapshot[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

func treeDiff(before, after map[string]string) string {
	var lines []string
	for path, value := range after {
		previous, existed := before[path]
		switch {
		case !existed:
			lines = append(lines, "created: "+path)
		case previous != value:
			lines = append(lines, "modified: "+path)
		}
	}
	for path := range before {
		if _, still := after[path]; !still {
			lines = append(lines, "removed: "+path)
		}
	}
	return strings.Join(lines, "\n")
}
