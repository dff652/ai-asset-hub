package migration

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/channel"
)

func TestInspectReportsAlignedLibraryInstallationAndChannelWithoutWriting(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join("..", "..", "testdata", "workspace-valid")
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	built, err := build.Build(build.Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Root:     root,
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil || !built.Ok || built.Package == nil {
		t.Fatalf("build err=%v report=%#v", err, built)
	}
	pkg := filepath.Join(out, built.Package.Archive)
	home := t.TempDir()
	applied, err := apply.Apply(apply.Options{
		Package: pkg, Home: home, Targets: []string{"claude"},
	})
	if err != nil || !applied.Ok {
		t.Fatalf("apply err=%v report=%#v", err, applied)
	}
	channelRoot := t.TempDir()
	published, err := channel.Publish(channel.PublishOptions{
		Package: pkg, Channel: channelRoot,
	})
	if err != nil || !published.Ok {
		t.Fatalf("publish err=%v report=%#v", err, published)
	}
	before := treeDigest(t, root, home, channelRoot)

	report, err := Inspect(Options{
		WorkspaceRoot: root,
		Home:          home,
		Channel:       channelRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ok || report.Library.Name != "fixture-personal" ||
		report.Library.Version != "2026.07.1" ||
		report.Library.AssetCount != 2 {
		t.Fatalf("library status = %#v", report.Library)
	}
	if !report.Installation.Present ||
		strings.Join(report.Installation.Targets, ",") != "claude" ||
		report.Alignment.Installation != "same-version" {
		t.Fatalf("installation status=%#v alignment=%#v",
			report.Installation, report.Alignment)
	}
	if !report.Channel.Selected || report.Channel.ReleaseCount != 1 ||
		report.Channel.Latest == nil ||
		report.Alignment.Channel != "same-version" {
		t.Fatalf("channel status=%#v alignment=%#v", report.Channel, report.Alignment)
	}
	if after := treeDigest(t, root, home, channelRoot); after != before {
		t.Fatalf("read-only inspection changed files:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestInspectWithoutChannelDoesNotPretendTheLibraryIsPublished(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "workspace-valid"))); err != nil {
		t.Fatal(err)
	}
	report, err := Inspect(Options{WorkspaceRoot: root, Home: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if report.Channel.Selected || report.Channel.Latest != nil ||
		report.Alignment.Channel != "channel-not-selected" {
		t.Fatalf("channel status=%#v alignment=%#v", report.Channel, report.Alignment)
	}
	if report.Installation.Present ||
		report.Alignment.Installation != "not-installed" {
		t.Fatalf("installation status=%#v alignment=%#v",
			report.Installation, report.Alignment)
	}
}

func treeDigest(t *testing.T, roots ...string) string {
	t.Helper()
	var entries []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if entry.IsDir() {
				entries = append(entries, fmt.Sprintf("%s:d:%s", root, relative))
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entries = append(entries, fmt.Sprintf(
				"%s:f:%s:%x", root, relative, sha256.Sum256(body),
			))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n")
}
