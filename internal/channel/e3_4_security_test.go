package channel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishRecoversAReleaseTreeAheadOfTheIndex(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	first, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if err := os.Remove(filepath.Join(channelRoot, indexName)); err != nil {
		t.Fatal(err)
	}
	releaseBefore := snapshotTree(t, filepath.Join(channelRoot, filepath.FromSlash(first.Path)))

	recovered, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("recover publish: %v", err)
	}
	if !recovered.Ok || !recovered.Unchanged {
		t.Fatalf("report = %#v", recovered)
	}
	if diff := treeDiff(
		releaseBefore,
		snapshotTree(t, filepath.Join(channelRoot, filepath.FromSlash(first.Path))),
	); diff != "" {
		t.Fatalf("recovery rewrote the immutable release:\n%s", diff)
	}
	listed, err := List(ListOptions{Channel: channelRoot, Name: first.Name})
	if err != nil || len(listed.Releases) != 1 {
		t.Fatalf("recovered index: err=%v report=%#v", err, listed)
	}
}

func TestPullRejectsAPathEscapingChannelIndex(t *testing.T) {
	parent := t.TempDir()
	trusted := filepath.Join(parent, "trusted")
	attacker := filepath.Join(parent, "attacker")
	out := filepath.Join(parent, "out")
	for _, directory := range []string{trusted, attacker, out} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source := buildPackage(t, "workspace-2b", "personal")
	published, err := Publish(PublishOptions{Package: source.archive, Channel: trusted})
	if err != nil {
		t.Fatalf("publish trusted release: %v", err)
	}
	writeMaliciousIndex(t, attacker, Index{
		SchemaVersion: 1,
		Kind:          "channel",
		Releases: []Release{{
			Name:    published.Name,
			Version: published.Version,
			Profile: published.Profile,
			Archive: filepath.Base(source.archive),
			SHA256:  published.SHA256,
			Path:    filepath.ToSlash(filepath.Join("..", "trusted", published.Path)),
		}},
	})
	before := snapshotTree(t, out)

	_, err = Pull(PullOptions{
		Channel: attacker,
		Name:    published.Name,
		Version: published.Version,
		Profile: published.Profile,
		Out:     out,
	})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; an escaping index path was accepted", err)
	}
	if diff := treeDiff(before, snapshotTree(t, out)); diff != "" {
		t.Fatalf("a rejected malicious channel wrote output:\n%s", diff)
	}
}

func TestPullRejectsASymlinkedReleaseDirectory(t *testing.T) {
	parent := t.TempDir()
	trusted := filepath.Join(parent, "trusted")
	attacker := filepath.Join(parent, "attacker")
	out := filepath.Join(parent, "out")
	for _, directory := range []string{trusted, attacker, out} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source := buildPackage(t, "workspace-2b", "personal")
	published, err := Publish(PublishOptions{Package: source.archive, Channel: trusted})
	if err != nil {
		t.Fatalf("publish trusted release: %v", err)
	}
	releaseParent := filepath.Join(
		attacker, "packages", published.Name, published.Version,
	)
	if err := os.MkdirAll(releaseParent, 0o755); err != nil {
		t.Fatal(err)
	}
	trustedRelease := filepath.Join(trusted, filepath.FromSlash(published.Path))
	if err := os.Symlink(trustedRelease, filepath.Join(releaseParent, published.Profile)); err != nil {
		t.Fatal(err)
	}
	writeMaliciousIndex(t, attacker, Index{
		SchemaVersion: 1,
		Kind:          "channel",
		Releases: []Release{{
			Name:    published.Name,
			Version: published.Version,
			Profile: published.Profile,
			Archive: filepath.Base(source.archive),
			SHA256:  published.SHA256,
			Path:    published.Path,
		}},
	})
	before := snapshotTree(t, out)

	_, err = Pull(PullOptions{
		Channel: attacker,
		Name:    published.Name,
		Version: published.Version,
		Profile: published.Profile,
		Out:     out,
	})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; a symlinked release directory was accepted", err)
	}
	if diff := treeDiff(before, snapshotTree(t, out)); diff != "" {
		t.Fatalf("a rejected symlinked release wrote output:\n%s", diff)
	}
}

func TestPublishRejectsASymlinkedChannelParents(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(channelRoot, packagesDir)); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, outside)

	_, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; a symlinked publish parent was accepted", err)
	}
	if diff := treeDiff(before, snapshotTree(t, outside)); diff != "" {
		t.Fatalf("a rejected publish wrote outside the channel:\n%s", diff)
	}
}

func TestChannelIndexRejectsNonCanonicalReleaseRecords(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	trusted := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: trusted})
	if err != nil {
		t.Fatal(err)
	}
	valid := Release{
		Name: published.Name, Version: published.Version, Profile: published.Profile,
		Archive: filepath.Base(source.archive), SHA256: published.SHA256, Path: published.Path,
	}
	tests := []struct {
		name   string
		mutate func(Release) []Release
	}{
		{
			name: "unsafe coordinate",
			mutate: func(release Release) []Release {
				release.Name = "../escape"
				return []Release{release}
			},
		},
		{
			name: "archive mismatch",
			mutate: func(release Release) []Release {
				release.Archive = "another.tar"
				return []Release{release}
			},
		},
		{
			name: "path mismatch",
			mutate: func(release Release) []Release {
				release.Path = releasePath(release.Name, "another", release.Profile)
				return []Release{release}
			},
		},
		{
			name: "invalid digest",
			mutate: func(release Release) []Release {
				release.SHA256 = "short"
				return []Release{release}
			},
		},
		{
			name: "duplicate coordinate",
			mutate: func(release Release) []Release {
				return []Release{release, release}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			channelRoot := t.TempDir()
			writeMaliciousIndex(t, channelRoot, Index{
				SchemaVersion: 1, Kind: "channel", Releases: test.mutate(valid),
			})
			if _, err := List(ListOptions{Channel: channelRoot}); !errors.Is(err, ErrChannelBlocked) {
				t.Fatalf("err = %v; malformed release records were accepted", err)
			}
		})
	}
}

func TestChannelIndexRejectsASymlink(t *testing.T) {
	channelRoot := t.TempDir()
	outside := filepath.Join(t.TempDir(), indexName)
	if err := os.WriteFile(outside, []byte(`{"schemaVersion":1,"kind":"channel","releases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(channelRoot, indexName)); err != nil {
		t.Fatal(err)
	}
	if _, err := List(ListOptions{Channel: channelRoot}); !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; a symlinked channel index was accepted", err)
	}
}

func TestChannelIndexRejectsTrailingJSONAndOversizedInput(t *testing.T) {
	t.Run("trailing JSON", func(t *testing.T) {
		channelRoot := t.TempDir()
		body := []byte(
			`{"schemaVersion":1,"kind":"channel","releases":[]} {"extra":true}`,
		)
		if err := os.WriteFile(filepath.Join(channelRoot, indexName), body, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := List(ListOptions{Channel: channelRoot}); !errors.Is(err, ErrChannelBlocked) {
			t.Fatalf("err = %v; trailing JSON was accepted", err)
		}
	})
	t.Run("oversized", func(t *testing.T) {
		channelRoot := t.TempDir()
		indexPath := filepath.Join(channelRoot, indexName)
		file, err := os.Create(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxIndexBytes + 1); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := List(ListOptions{Channel: channelRoot}); !errors.Is(err, ErrChannelBlocked) {
			t.Fatalf("err = %v; oversized index was accepted", err)
		}
	})
}

func writeMaliciousIndex(t *testing.T, root string, index Index) {
	t.Helper()
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, indexName), append(body, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
