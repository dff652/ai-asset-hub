package channel

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
)

func TestPublishThenPullRoundTrip(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()

	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !published.Ok || published.Unchanged {
		t.Fatalf("publish report = %#v", published)
	}
	if published.Path != filepath.ToSlash(filepath.Join("packages", published.Name, published.Version, published.Profile)) {
		t.Fatalf("path = %q", published.Path)
	}

	out := t.TempDir()
	pulled, err := Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if !pulled.Ok || pulled.Version != published.Version || pulled.SHA256 != published.SHA256 {
		t.Fatalf("pull report = %#v", pulled)
	}
	if !pulled.ResolvedLatest {
		t.Fatal("an omitted --version was not reported as resolved by publish order")
	}
	// The retrieved archive must be byte-identical to what was published.
	original, err := os.ReadFile(source.archive)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := os.ReadFile(pulled.Package)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(retrieved) {
		t.Fatal("the retrieved archive differs from the published one")
	}
}

func TestPullRefusesToOverwriteOutputArtifacts(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	out := t.TempDir()
	target := filepath.Join(out, filepath.Base(source.archive))
	if err := os.WriteFile(target, []byte("keep this local file"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, out)

	_, err = Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v, want ErrChannelBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, out)); diff != "" {
		t.Fatalf("a refused pull changed the output directory:\n%s", diff)
	}
	body, err := os.ReadFile(target)
	if err != nil || string(body) != "keep this local file" {
		t.Fatalf("pre-existing artifact was not preserved: body=%q err=%v", body, err)
	}
}

func TestPullRefusesAPartialIdenticalArtifactSet(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	out := t.TempDir()
	archive, err := os.ReadFile(source.archive)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(out, filepath.Base(source.archive))
	if err := os.WriteFile(target, archive, 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, out)

	_, err = Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v, want ErrChannelBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, out)); diff != "" {
		t.Fatalf("a refused partial pull changed the output directory:\n%s", diff)
	}
}

func TestPullOfAnIdenticalArtifactSetIsIdempotent(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	out := t.TempDir()
	options := PullOptions{Channel: channelRoot, Name: published.Name, Out: out}
	first, err := Pull(options)
	if err != nil {
		t.Fatalf("first pull: %v", err)
	}
	before := snapshotTree(t, out)

	second, err := Pull(options)
	if err != nil {
		t.Fatalf("second pull: %v", err)
	}
	if !second.Ok || second.Package != first.Package || second.SHA256 != first.SHA256 {
		t.Fatalf("second report = %#v, first = %#v", second, first)
	}
	if diff := treeDiff(before, snapshotTree(t, out)); diff != "" {
		t.Fatalf("an idempotent pull changed the output directory:\n%s", diff)
	}
}

func TestPullRefusesAnOutputArtifactSymlink(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	out := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	archive, err := os.ReadFile(source.archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(out, filepath.Base(source.archive))
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, out)

	_, err = Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v, want ErrChannelBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, out)); diff != "" {
		t.Fatalf("a refused symlink pull changed the output directory:\n%s", diff)
	}
	body, err := os.ReadFile(outside)
	if err != nil || string(body) != string(archive) {
		t.Fatalf("symlink target changed: err=%v", err)
	}
}

// TestPublishIsImmutable anchors ADR-0007 §3.
func TestPublishIsImmutable(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	if _, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	before := snapshotTree(t, channelRoot)

	t.Run("identical republish is a no-op", func(t *testing.T) {
		report, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
		if err != nil {
			t.Fatalf("republish: %v", err)
		}
		if !report.Ok || !report.Unchanged {
			t.Fatalf("report = %#v", report)
		}
		if diff := treeDiff(before, snapshotTree(t, channelRoot)); diff != "" {
			t.Fatalf("an identical republish wrote:\n%s", diff)
		}
	})

	t.Run("different content under the same version is refused", func(t *testing.T) {
		tampered := tamperedCopy(t, source)
		_, err := Publish(PublishOptions{Package: tampered.archive, Channel: channelRoot})
		if !errors.Is(err, ErrChannelBlocked) {
			t.Fatalf("err = %v, want ErrChannelBlocked", err)
		}
		if diff := treeDiff(before, snapshotTree(t, channelRoot)); diff != "" {
			t.Fatalf("a refused publish still wrote:\n%s", diff)
		}
	})
}

// TestPublishRefusesACorruptSource: a channel that hands out a broken package
// is worse than one that refuses to accept it (ADR-0007 §4).
func TestPublishRefusesACorruptSource(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	// Corrupt the archive but leave the recorded checksum alone.
	appendBytes(t, source.archive, []byte("junk"))
	channelRoot := t.TempDir()
	before := snapshotTree(t, channelRoot)

	_, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v, want ErrChannelBlocked", err)
	}
	if diff := treeDiff(before, snapshotTree(t, channelRoot)); diff != "" {
		t.Fatalf("a corrupt package reached the channel:\n%s", diff)
	}
}

// TestPullRejectsATamperedChannel anchors the retrieval-side check: verification
// runs against what actually landed, and a mismatch leaves nothing behind.
func TestPullRejectsATamperedChannel(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	inChannel := filepath.Join(channelRoot, filepath.FromSlash(published.Path),
		published.Name+"-"+published.Version+"-"+published.Profile+".tar")
	appendBytes(t, inChannel, []byte("tampered"))

	out := t.TempDir()
	_, err = Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v, want ErrChannelBlocked", err)
	}
	remaining := snapshotTree(t, out)
	if len(remaining) != 1 { // the root itself
		t.Fatalf("a rejected pull left files behind: %v", remaining)
	}
}

// TestPullRejectsAConsistentButUnloadablePackage isolates the half of
// verification the digest comparisons cannot reach. Every checksum in this
// channel agrees -- the .sha256 beside the archive and the digest in the index
// both describe the bytes that are actually there -- so the only thing left to
// notice is that those bytes are not a package at all.
func TestPullRejectsAConsistentButUnloadablePackage(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	releaseDir := filepath.Join(channelRoot, filepath.FromSlash(published.Path))
	base := published.Name + "-" + published.Version + "-" + published.Profile
	archive := filepath.Join(releaseDir, base+".tar")

	if err := os.WriteFile(archive, []byte("not a tar at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := digestOfFile(t, archive)
	line := fmt.Sprintf("%s  %s\n", digest, base+".tar")
	if err := os.WriteFile(filepath.Join(releaseDir, base+".sha256"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	rewriteIndexDigest(t, channelRoot, digest)

	out := t.TempDir()
	if _, err := Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out}); !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; a self-consistent non-package was accepted", err)
	}
	if remaining := snapshotTree(t, out); len(remaining) != 1 {
		t.Fatalf("a rejected pull left files behind: %v", remaining)
	}
}

// TestPublishRejectsAConsistentButUnloadablePackage is the same isolation on the
// publish side: a channel must not accept bytes it cannot read back as a package.
func TestPublishRejectsAConsistentButUnloadablePackage(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	if err := os.WriteFile(source.archive, []byte("not a tar at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("%s  %s\n", digestOfFile(t, source.archive), filepath.Base(source.archive))
	if err := os.WriteFile(source.sha, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	channelRoot := t.TempDir()
	before := snapshotTree(t, channelRoot)

	if _, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot}); !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; a self-consistent non-package was published", err)
	}
	if diff := treeDiff(before, snapshotTree(t, channelRoot)); diff != "" {
		t.Fatalf("a refused publish still wrote:\n%s", diff)
	}
}

// TestPullVerifiesAgainstTheIndexDigest isolates the other direction: archive
// and its .sha256 agree with each other, so only the digest the channel index
// recorded at publish time can reveal the swap.
func TestPullVerifiesAgainstTheIndexDigest(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	releaseDir := filepath.Join(channelRoot, filepath.FromSlash(published.Path))
	base := published.Name + "-" + published.Version + "-" + published.Profile
	archive := filepath.Join(releaseDir, base+".tar")
	appendBytes(t, archive, []byte("swapped"))
	// Keep the shipped checksum consistent with the swapped archive.
	line := fmt.Sprintf("%s  %s\n", digestOfFile(t, archive), base+".tar")
	if err := os.WriteFile(filepath.Join(releaseDir, base+".sha256"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	if _, err := Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: out}); !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; the index digest was not compared", err)
	}
	if remaining := snapshotTree(t, out); len(remaining) != 1 {
		t.Fatalf("a rejected pull left files behind: %v", remaining)
	}
}

// TestPullResolvesByPublishOrderNotVersionOrder anchors ADR-0007 §5: aiah must
// not invent a version ordering, so "latest" is the most recently published.
func TestPullResolvesByPublishOrderNotVersionOrder(t *testing.T) {
	channelRoot := t.TempDir()
	// Publish .9 first, then .10. Publish order says .10; a string comparison
	// says .9, because "9" sorts after "1". The two answers must diverge or the
	// test cannot tell the rule apart from the guess this project refuses to
	// make.
	for _, release := range []string{"2026.07.9", "2026.07.10"} {
		source := buildPackageVersion(t, "workspace-2b", "personal", release)
		if _, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot}); err != nil {
			t.Fatalf("publish %s: %v", release, err)
		}
	}
	report, err := Pull(PullOptions{
		Channel: channelRoot, Name: "fixture-phase2b", Out: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if report.Version != "2026.07.10" {
		t.Fatalf("version = %q, want the most recently published 2026.07.10 "+
			"(a string comparison would wrongly pick 2026.07.9)", report.Version)
	}
}

func TestPullByExplicitVersion(t *testing.T) {
	channelRoot := t.TempDir()
	for _, release := range []string{"2026.07.1", "2026.07.2"} {
		source := buildPackageVersion(t, "workspace-2b", "personal", release)
		if _, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot}); err != nil {
			t.Fatalf("publish %s: %v", release, err)
		}
	}
	report, err := Pull(PullOptions{
		Channel: channelRoot, Name: "fixture-phase2b", Version: "2026.07.1", Out: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if report.Version != "2026.07.1" || report.ResolvedLatest {
		t.Fatalf("report = %#v", report)
	}
}

func TestPullMissingReleaseIsNotFound(t *testing.T) {
	channelRoot := t.TempDir()
	_, err := Pull(PullOptions{Channel: channelRoot, Name: "absent", Out: t.TempDir()})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPullDetectsAnIndexAheadOfTheTree(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	published, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(channelRoot, filepath.FromSlash(published.Path))); err != nil {
		t.Fatal(err)
	}
	_, err = Pull(PullOptions{Channel: channelRoot, Name: published.Name, Out: t.TempDir()})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; a listed-but-absent release must fail loudly", err)
	}
}

func TestListReportsPublishOrder(t *testing.T) {
	channelRoot := t.TempDir()
	for _, release := range []string{"2026.07.3", "2026.07.1", "2026.07.2"} {
		source := buildPackageVersion(t, "workspace-2b", "personal", release)
		if _, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot}); err != nil {
			t.Fatalf("publish %s: %v", release, err)
		}
	}
	report, err := List(ListOptions{Channel: channelRoot})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := make([]string, 0, len(report.Releases))
	for _, release := range report.Releases {
		got = append(got, release.Version)
	}
	if strings.Join(got, ",") != "2026.07.3,2026.07.1,2026.07.2" {
		t.Fatalf("releases = %v; list must report publish order, not a sorted order", got)
	}
}

func TestListFiltersByName(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	channelRoot := t.TempDir()
	if _, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot}); err != nil {
		t.Fatal(err)
	}
	report, err := List(ListOptions{Channel: channelRoot, Name: "absent"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(report.Releases) != 0 {
		t.Fatalf("filter returned %#v", report.Releases)
	}
}

func TestPublishRequiresTheSiblingArtifacts(t *testing.T) {
	source := buildPackage(t, "workspace-2b", "personal")
	if err := os.Remove(source.lock); err != nil {
		t.Fatal(err)
	}
	channelRoot := t.TempDir()
	_, err := Publish(PublishOptions{Package: source.archive, Channel: channelRoot})
	if !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v, want ErrChannelBlocked", err)
	}
}

func TestPublishRejectsUnsafeCoordinates(t *testing.T) {
	// A coordinate becomes a directory name, so anything that could escape the
	// channel is refused rather than sanitised.
	for _, value := range []string{"", "..", "../evil", "a/b", ".hidden", strings.Repeat("x", 129)} {
		if validCoordinate(value) {
			t.Fatalf("validCoordinate(%q) = true", value)
		}
	}
	for _, value := range []string{"ym-personal", "2026.07.1", "personal", "A1"} {
		if !validCoordinate(value) {
			t.Fatalf("validCoordinate(%q) = false", value)
		}
	}
}

func TestParseSHAFileRejectsMismatchedName(t *testing.T) {
	digest := strings.Repeat("a", 64)
	if _, ok := parseSHAFile([]byte(digest+"  other.tar\n"), "wanted.tar"); ok {
		t.Fatal("a checksum naming a different file was accepted")
	}
	if _, ok := parseSHAFile([]byte("short  wanted.tar\n"), "wanted.tar"); ok {
		t.Fatal("a short digest was accepted")
	}
	if _, ok := parseSHAFile([]byte(strings.Repeat("z", 64)+"  wanted.tar\n"), "wanted.tar"); ok {
		t.Fatal("a non-hex digest was accepted")
	}
	if got, ok := parseSHAFile([]byte(digest+"  wanted.tar\n"), "wanted.tar"); !ok || got != digest {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestChannelIndexRejectsUnknownShape(t *testing.T) {
	channelRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(channelRoot, indexName),
		[]byte(`{"schemaVersion":2,"kind":"channel","releases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(ListOptions{Channel: channelRoot}); !errors.Is(err, ErrChannelBlocked) {
		t.Fatalf("err = %v; an unrecognised index must fail closed", err)
	}
}

// --- helpers ---

type sourcePackage struct {
	dir      string
	archive  string
	lock     string
	manifest string
	sha      string
}

func buildPackage(t *testing.T, fixture, profile string) sourcePackage {
	t.Helper()
	return buildPackageVersion(t, fixture, profile, "")
}

// buildPackageVersion builds a fixture workspace, optionally overriding the
// manifest version so a test can publish several releases.
func buildPackageVersion(t *testing.T, fixture, profile, overrideVersion string) sourcePackage {
	t.Helper()
	root := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "testdata", fixture), root)
	manifestPath := filepath.Join(root, "manifest.yaml")
	if overrideVersion != "" {
		body, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		const anchor = `version: "2026.07.2"`
		if !strings.Contains(string(body), anchor) {
			t.Fatal("fixture manifest version anchor changed; update the helper")
		}
		// Checking for the anchor rather than comparing before/after: overriding
		// to the fixture's own version is a legitimate no-op replace.
		updated := strings.Replace(string(body), anchor, `version: "`+overrideVersion+`"`, 1)
		if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outDir := t.TempDir()
	report, err := build.Build(build.Options{Manifest: manifestPath, Profile: profile, OutDir: outDir})
	if err != nil || !report.Ok {
		t.Fatalf("build fixture: err=%v ok=%v", err, report.Ok)
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "*.tar"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want one tar, got %v (%v)", matches, err)
	}
	base := strings.TrimSuffix(matches[0], ".tar")
	return sourcePackage{
		dir: outDir, archive: matches[0],
		lock: base + ".lock.json", manifest: base + ".manifest.json", sha: base + ".sha256",
	}
}

// tamperedCopy produces a package with the same coordinates but different
// bytes, with its checksum updated so only the immutability rule can stop it.
func tamperedCopy(t *testing.T, source sourcePackage) sourcePackage {
	t.Helper()
	destination := t.TempDir()
	copied := sourcePackage{dir: destination}
	for _, pair := range [][2]*string{
		{&source.archive, &copied.archive}, {&source.lock, &copied.lock},
		{&source.manifest, &copied.manifest}, {&source.sha, &copied.sha},
	} {
		body, err := os.ReadFile(*pair[0])
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(destination, filepath.Base(*pair[0]))
		if err := os.WriteFile(target, body, 0o644); err != nil {
			t.Fatal(err)
		}
		*pair[1] = target
	}
	appendBytes(t, copied.archive, []byte("different"))
	archive, err := os.ReadFile(copied.archive)
	if err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), filepath.Base(copied.archive))
	if err := os.WriteFile(copied.sha, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return copied
}

func digestOfFile(t *testing.T, filePath string) string {
	t.Helper()
	body, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(body))
}

// rewriteIndexDigest makes the channel index agree with a tampered archive.
func rewriteIndexDigest(t *testing.T, channelRoot, digest string) {
	t.Helper()
	index, err := loadIndex(channelRoot)
	if err != nil {
		t.Fatal(err)
	}
	for position := range index.Releases {
		index.Releases[position].SHA256 = digest
	}
	if err := writeIndex(channelRoot, index); err != nil {
		t.Fatal(err)
	}
}

func appendBytes(t *testing.T, filePath string, extra []byte) {
	t.Helper()
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(extra); err != nil {
		t.Fatal(err)
	}
}

func copyFixture(t *testing.T, source, destination string) {
	t.Helper()
	err := filepath.WalkDir(source, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%04o", info.Mode().Type(), info.Mode().Perm())
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(current)
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
	for current, value := range after {
		previous, existed := before[current]
		switch {
		case !existed:
			lines = append(lines, "created: "+current)
		case previous != value:
			lines = append(lines, "modified: "+current)
		}
	}
	for current := range before {
		if _, still := after[current]; !still {
			lines = append(lines, "removed: "+current)
		}
	}
	return strings.Join(lines, "\n")
}
