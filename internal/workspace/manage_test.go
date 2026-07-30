package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/inventory"
)

func TestCatalogUnifiesDiscoveredAndLibraryState(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
		".codex/skills/new/SKILL.md":     "# new\n",
	})
	root := t.TempDir()
	_, err := Compose(ComposeOptions{
		WorkspaceRoot: root,
		Home:          home,
		Assets:        []inventory.Asset{skillAsset("home/.claude/skills/review")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".claude", "skills", "review", "SKILL.md"),
		[]byte("# changed\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	report, err := Catalog(CatalogOptions{
		WorkspaceRoot: root,
		Home:          home,
		Assets: []inventory.Asset{
			skillAsset("home/.claude/skills/review"),
			codexSkillAsset("home/.codex/skills/new"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ok || report.Summary.SourceChanged != 1 || report.Summary.Unmanaged != 1 {
		t.Fatalf("catalog summary = %#v", report.Summary)
	}
}

func TestCatalogBlocksUnreadableLibraryAssetInsteadOfOfferingUpdate(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
	})
	root := t.TempDir()
	asset := skillAsset("home/.claude/skills/review")
	if _, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil); err != nil {
		t.Fatal(err)
	}
	libraryPath := filepath.Join(root, "assets", "skills", "review")
	if err := os.RemoveAll(libraryPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), libraryPath); err != nil {
		t.Fatal(err)
	}

	report, err := Catalog(CatalogOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Blocked != 1 || report.Summary.SourceChanged != 0 {
		t.Fatalf("catalog summary = %#v", report.Summary)
	}
	if len(report.Items) != 1 || report.Items[0].State != LibraryBlocked {
		t.Fatalf("catalog items = %#v", report.Items)
	}
	if len(report.Items[0].Findings) != 1 ||
		report.Items[0].Findings[0].Code != "library_asset_not_regular" {
		t.Fatalf("catalog findings = %#v", report.Items[0].Findings)
	}
}

func TestUpdateAssetsReplacesWholeAssetAndNeverWritesSource(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# old\n",
		".claude/skills/review/notes.md": "old\n",
	})
	root := t.TempDir()
	asset := multiFileSkillAsset("home/.claude/skills/review")
	_, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills", "review", "SKILL.md"), []byte("# new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, ".claude", "skills", "review", "notes.md")); err != nil {
		t.Fatal(err)
	}
	asset.Files = []string{"home/.claude/skills/review/SKILL.md"}
	sourceBefore := snapshotTree(t, home)

	result, err := UpdateAssets(UpdateOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil)
	if err != nil || !result.Ok || strings.Join(result.Updated, ",") != "skill.review" {
		t.Fatalf("update result=%#v err=%v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "assets", "skills", "review", "SKILL.md"))
	if err != nil || string(body) != "# new\n" {
		t.Fatalf("updated body=%q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "assets", "skills", "review", "notes.md")); !os.IsNotExist(err) {
		t.Fatal("whole-asset update retained a stale file")
	}
	if diff := treeDiff(sourceBefore, snapshotTree(t, home)); diff != "" {
		t.Fatalf("update modified the discovered source:\n%s", diff)
	}
}

func TestUpdateAssetsRestoresPreviousContentWhenValidationFails(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# old\n",
	})
	root := t.TempDir()
	asset := skillAsset("home/.claude/skills/review")
	if _, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills", "review", "SKILL.md"), []byte("# new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, root)

	_, err := UpdateAssets(UpdateOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, func(string, string) error { return errors.New("reject") })
	if !errors.Is(err, ErrManageBlocked) {
		t.Fatalf("err=%v, want ErrManageBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, root)); diff != "" {
		t.Fatalf("failed update changed the asset library:\n%s", diff)
	}
}

func TestRemoveAssetsUpdatesManifestAndRemovesLibraryContent(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
	})
	root := t.TempDir()
	asset := skillAsset("home/.claude/skills/review")
	if _, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home, Assets: []inventory.Asset{asset},
	}, nil); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "manifest.yaml")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw = append([]byte("# keep this comment\n"), raw...)
	if err := os.WriteFile(manifest, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RemoveAssets(RemoveOptions{
		WorkspaceRoot: root, AssetIDs: []string{"skill.review"},
	}, nil)
	if err != nil || !result.Ok {
		t.Fatalf("remove result=%#v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, "assets", "skills", "review")); !os.IsNotExist(err) {
		t.Fatal("removed asset content still exists")
	}
	after, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "# keep this comment") {
		t.Fatal("comment was lost while editing manifest")
	}
	document, _, err := LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Assets) != 0 || len(document.Profiles["personal"].Include) != 0 {
		t.Fatalf("removed asset remains referenced: %#v", document)
	}
}

func TestRemoveAssetsRestoresContentWhenValidationFails(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".claude/skills/review/SKILL.md": "# review\n",
	})
	root := t.TempDir()
	if _, err := Compose(ComposeOptions{
		WorkspaceRoot: root, Home: home,
		Assets: []inventory.Asset{skillAsset("home/.claude/skills/review")},
	}, nil); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, root)
	_, err := RemoveAssets(RemoveOptions{
		WorkspaceRoot: root, AssetIDs: []string{"skill.review"},
	}, func(string, string) error { return errors.New("reject") })
	if !errors.Is(err, ErrManageBlocked) {
		t.Fatalf("err=%v, want ErrManageBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, root)); diff != "" {
		t.Fatalf("failed removal changed the asset library:\n%s", diff)
	}
}
