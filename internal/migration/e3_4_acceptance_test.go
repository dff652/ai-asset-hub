package migration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/channel"
)

func TestE34TwoDeviceTransferIdempotencyAndExplicitOldVersionRestore(t *testing.T) {
	sourceWorkspace := t.TempDir()
	if err := os.CopyFS(
		sourceWorkspace,
		os.DirFS(filepath.Join("..", "..", "testdata", "workspace-valid")),
	); err != nil {
		t.Fatal(err)
	}
	sourceChannel := t.TempDir()

	v1 := buildMigrationRelease(t, sourceWorkspace, t.TempDir())
	publishedV1, err := channel.Publish(channel.PublishOptions{
		Package: v1.archive,
		Channel: sourceChannel,
	})
	if err != nil || !publishedV1.Ok || publishedV1.Unchanged {
		t.Fatalf("publish v1: err=%v report=%#v", err, publishedV1)
	}

	skillSource := filepath.Join(
		sourceWorkspace, "assets", "skills", "shared-review", "SKILL.md",
	)
	body, err := os.ReadFile(skillSource)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillSource, append(body, []byte("\nVersion two marker.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	replaceMigrationFixture(t, filepath.Join(sourceWorkspace, "manifest.yaml"),
		`version: "2026.07.1"`, `version: "2026.07.2"`)
	v2 := buildMigrationRelease(t, sourceWorkspace, t.TempDir())
	publishedV2, err := channel.Publish(channel.PublishOptions{
		Package: v2.archive,
		Channel: sourceChannel,
	})
	if err != nil || !publishedV2.Ok || publishedV2.Unchanged {
		t.Fatalf("publish v2: err=%v report=%#v", err, publishedV2)
	}

	republished, err := channel.Publish(channel.PublishOptions{
		Package: v1.archive,
		Channel: sourceChannel,
	})
	if err != nil || !republished.Ok || !republished.Unchanged {
		t.Fatalf("idempotent publish v1: err=%v report=%#v", err, republished)
	}

	replaceMigrationFixture(t, filepath.Join(sourceWorkspace, "manifest.yaml"),
		`version: "2026.07.2"`, `version: "2026.07.1"`)
	conflict := buildMigrationRelease(t, sourceWorkspace, t.TempDir())
	if _, err := channel.Publish(channel.PublishOptions{
		Package: conflict.archive,
		Channel: sourceChannel,
	}); !errors.Is(err, channel.ErrChannelBlocked) {
		t.Fatalf("different content at the v1 coordinate was accepted: %v", err)
	}

	transportParent := t.TempDir()
	targetChannel := filepath.Join(transportParent, "received-channel")
	if err := os.CopyFS(targetChannel, os.DirFS(sourceChannel)); err != nil {
		t.Fatalf("transport channel: %v", err)
	}
	targetHome := t.TempDir()
	incoming := t.TempDir()

	pulledV1 := pullMigrationRelease(t, targetChannel, incoming, publishedV1)
	incomingBefore := treeDigest(t, incoming)
	pulledV1Again := pullMigrationRelease(t, targetChannel, incoming, publishedV1)
	if pulledV1Again.Package != pulledV1.Package ||
		pulledV1Again.SHA256 != pulledV1.SHA256 ||
		treeDigest(t, incoming) != incomingBefore {
		t.Fatal("an identical target-device pull was not an idempotent no-op")
	}
	assertPackagePreflight(t, targetHome, pulledV1)

	homeBefore := treeDigest(t, targetHome)
	diffV1, err := apply.Diff(apply.Options{
		Package: pulledV1.Package, ExpectedSHA256: pulledV1.SHA256, Home: targetHome,
	})
	if err != nil || !diffV1.Ok || diffV1.Summary.Create == 0 {
		t.Fatalf("diff v1: err=%v report=%#v", err, diffV1)
	}
	if treeDigest(t, targetHome) != homeBefore {
		t.Fatal("target-device preflight or diff wrote HOME")
	}
	appliedV1, err := apply.Apply(apply.Options{
		Package: pulledV1.Package, ExpectedSHA256: pulledV1.SHA256, Home: targetHome,
	})
	if err != nil || !appliedV1.Ok || appliedV1.Summary.Written == 0 {
		t.Fatalf("apply v1: err=%v report=%#v", err, appliedV1)
	}

	pulledV2 := pullMigrationRelease(t, targetChannel, incoming, publishedV2)
	assertPackagePreflight(t, targetHome, pulledV2)
	appliedV2, err := apply.Apply(apply.Options{
		Package: pulledV2.Package, ExpectedSHA256: pulledV2.SHA256, Home: targetHome,
	})
	if err != nil || !appliedV2.Ok || appliedV2.Summary.Update == 0 {
		t.Fatalf("apply v2: err=%v report=%#v", err, appliedV2)
	}
	installed := filepath.Join(targetHome, ".claude", "skills", "shared-review", "SKILL.md")
	assertMigrationBody(t, installed, "Version two marker.", true)

	// Restoring an old version is always explicit: select v1, preflight it,
	// review its diff, then apply. Nothing infers a version order.
	assertPackagePreflight(t, targetHome, pulledV1)
	oldDiff, err := apply.Diff(apply.Options{
		Package: pulledV1.Package, ExpectedSHA256: pulledV1.SHA256, Home: targetHome,
	})
	if err != nil || !oldDiff.Ok || oldDiff.Summary.Update == 0 {
		t.Fatalf("old-version diff: err=%v report=%#v", err, oldDiff)
	}
	restored, err := apply.Apply(apply.Options{
		Package: pulledV1.Package, ExpectedSHA256: pulledV1.SHA256, Home: targetHome,
	})
	if err != nil || !restored.Ok || restored.Summary.Update == 0 {
		t.Fatalf("old-version apply: err=%v report=%#v", err, restored)
	}
	assertMigrationBody(t, installed, "Version two marker.", false)

	doctor, err := apply.Doctor(apply.DoctorOptions{Home: targetHome})
	if err != nil || !doctor.Ok || doctor.Deployment == nil ||
		doctor.Deployment.Version != publishedV1.Version ||
		doctor.Summary.LocallyModified != 0 ||
		doctor.Summary.Missing != 0 {
		t.Fatalf("doctor after restore: err=%v report=%#v", err, doctor)
	}
}

type migrationRelease struct {
	archive string
}

func buildMigrationRelease(t *testing.T, root, out string) migrationRelease {
	t.Helper()
	report, err := build.Build(build.Options{
		Manifest: filepath.Join(root, "manifest.yaml"),
		Root:     root,
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil || !report.Ok || report.Package == nil {
		t.Fatalf("build release: err=%v report=%#v", err, report)
	}
	return migrationRelease{
		archive: filepath.Join(out, report.Package.Archive),
	}
}

func pullMigrationRelease(
	t *testing.T,
	channelRoot string,
	out string,
	release channel.PublishReport,
) channel.PullReport {
	t.Helper()
	report, err := channel.Pull(channel.PullOptions{
		Channel: channelRoot,
		Name:    release.Name,
		Version: release.Version,
		Profile: release.Profile,
		Out:     out,
	})
	if err != nil || !report.Ok || report.ResolvedLatest {
		t.Fatalf("pull %s: err=%v report=%#v", release.Version, err, report)
	}
	return report
}

func assertPackagePreflight(t *testing.T, home string, release channel.PullReport) {
	t.Helper()
	before := treeDigest(t, home)
	report, err := InspectPackagePreflight(PackagePreflightOptions{
		Package: release.Package,
		Home:    home,
		Expected: ReleaseIdentity{
			Name: release.Name, Version: release.Version,
			Profile: release.Profile, SHA256: release.SHA256,
		},
	})
	if err != nil || !report.Ok ||
		report.Subject.Source != "package" ||
		report.Subject.Version != release.Version ||
		report.Subject.SHA256 != release.SHA256 {
		t.Fatalf("preflight %s: err=%v report=%#v", release.Version, err, report)
	}
	if treeDigest(t, home) != before {
		t.Fatalf("preflight %s wrote target HOME", release.Version)
	}
}

func replaceMigrationFixture(t *testing.T, path, old, replacement string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), old, replacement, 1)
	if updated == string(body) {
		t.Fatalf("%q was not found in %s", old, path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertMigrationBody(t *testing.T, path, marker string, want bool) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	actual := strings.Contains(string(body), marker)
	if actual != want {
		t.Fatalf("%s marker presence=%v, want %v", path, actual, want)
	}
}
