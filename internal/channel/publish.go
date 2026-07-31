package channel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/build"
)

// PublishOptions describes one publish request.
type PublishOptions struct {
	// Package is the built .tar. Its three sibling artifacts must sit next to
	// it, exactly as build wrote them.
	Package string
	// Channel is the destination directory.
	Channel string
}

// PublishReport says what reached the channel.
type PublishReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	ProducedBy    string `json:"producedBy"`
	Ok            bool   `json:"ok"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	SHA256        string `json:"sha256"`
	Path          string `json:"path"`
	// Unchanged is true when this exact release was already published, so the
	// call wrote nothing.
	Unchanged bool `json:"unchanged"`
}

// Publish copies a built package into the channel under
// packages/<name>/<version>/<profile>/ and records it in the index.
//
// Publishing is immutable (ADR-0007 §3): re-publishing a byte-identical release
// is a no-op, and re-publishing different bytes under the same coordinate is
// refused. There is no --force; change the version instead.
func Publish(options PublishOptions) (PublishReport, error) {
	report := PublishReport{SchemaVersion: 1, Kind: "publish", ProducedBy: producedBy()}

	channelRoot, err := requireDirectory(options.Channel)
	if err != nil {
		return report, err
	}
	archivePath, err := filepath.Abs(strings.TrimSpace(options.Package))
	if err != nil || strings.TrimSpace(options.Package) == "" {
		return report, fmt.Errorf("%w: a package path is required", ErrChannelBlocked)
	}
	if !strings.HasSuffix(archivePath, ".tar") {
		return report, fmt.Errorf("%w: publish takes the built .tar", ErrChannelBlocked)
	}

	sourceDir := filepath.Dir(archivePath)
	artifacts := artifactsFor(strings.TrimSuffix(filepath.Base(archivePath), ".tar"))
	for _, name := range artifacts.names() {
		if !regularFile(filepath.Join(sourceDir, name)) {
			return report, fmt.Errorf("%w: %s is missing next to the package", ErrChannelBlocked, name)
		}
	}

	// Verify before publishing: a channel that hands out a corrupt package is
	// worse than one that refuses to accept it.
	digest, err := verifyPackage(archivePath, filepath.Join(sourceDir, artifacts.sha))
	if err != nil {
		return report, err
	}

	name, releaseVersion, profile, err := coordinatesOf(sourceDir, artifacts)
	if err != nil {
		return report, err
	}
	report.Name, report.Version, report.Profile, report.SHA256 = name, releaseVersion, profile, digest

	relative := releasePath(name, releaseVersion, profile)
	report.Path = relative

	index, err := loadIndex(channelRoot)
	if err != nil {
		return report, err
	}
	parent, err := secureChannelDirectory(channelRoot, path.Dir(relative), true)
	if err != nil {
		return report, err
	}
	destination := filepath.Join(parent, profile)

	if info, err := os.Lstat(destination); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return report, fmt.Errorf(
				"%w: the release destination is not a safe directory",
				ErrChannelBlocked,
			)
		}
		identical, err := releaseMatches(sourceDir, destination, artifacts)
		if err != nil {
			return report, err
		}
		if !identical {
			return report, fmt.Errorf(
				"%w: %s %s (%s) is already published with different content; publish a new version instead",
				ErrChannelBlocked, name, releaseVersion, profile)
		}
		report.Ok, report.Unchanged = true, true
		// The index can lag the tree if an earlier run died between the two
		// writes, so reconcile rather than assume.
		if index.indexOf(name, releaseVersion, profile) < 0 {
			index.Releases = append(index.Releases, releaseRecord(report, artifacts))
			if err := writeIndex(channelRoot, index); err != nil {
				return report, err
			}
		}
		return report, nil
	}

	if err := stageRelease(sourceDir, destination, artifacts); err != nil {
		return report, err
	}

	index.Releases = append(index.Releases, releaseRecord(report, artifacts))
	if err := writeIndex(channelRoot, index); err != nil {
		// The tree is ahead of the index; undo the tree so the channel does not
		// hold a release nothing can find.
		_ = os.RemoveAll(destination)
		return report, err
	}

	report.Ok = true
	return report, nil
}

func releaseRecord(report PublishReport, artifacts artifactSet) Release {
	return Release{
		Name:        report.Name,
		Version:     report.Version,
		Profile:     report.Profile,
		Archive:     artifacts.archive,
		SHA256:      report.SHA256,
		Path:        report.Path,
		PublishedBy: report.ProducedBy,
	}
}

// coordinatesOf reads name and version from the package manifest rather than
// parsing the filename: the manifest is what apply will believe, so the channel
// must be keyed by the same values. Profile has no manifest field, so it comes
// from the filename build itself produced.
func coordinatesOf(sourceDir string, artifacts artifactSet) (string, string, string, error) {
	body, err := os.ReadFile(filepath.Join(sourceDir, artifacts.manifest))
	if err != nil {
		return "", "", "", fmt.Errorf("%w: cannot read %s", ErrChannelBlocked, artifacts.manifest)
	}
	var manifest build.PackageManifest
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&manifest); err != nil {
		return "", "", "", fmt.Errorf("%w: %s does not parse", ErrChannelBlocked, artifacts.manifest)
	}
	name, releaseVersion := manifest.Name, manifest.Version

	prefix := name + "-" + releaseVersion + "-"
	if !strings.HasPrefix(artifacts.base, prefix) {
		return "", "", "", fmt.Errorf(
			"%w: %s does not match the manifest's name and version; rebuild the package",
			ErrChannelBlocked, artifacts.archive)
	}
	profile := strings.TrimPrefix(artifacts.base, prefix)

	for label, value := range map[string]string{
		"name": name, "version": releaseVersion, "profile": profile,
	} {
		if !validCoordinate(value) {
			return "", "", "", fmt.Errorf(
				"%w: %s %q cannot be used as a channel path segment", ErrChannelBlocked, label, value)
		}
	}
	return name, releaseVersion, profile, nil
}

// releaseMatches reports whether the already-published release is byte-identical
// to the one being offered.
func releaseMatches(sourceDir, destination string, artifacts artifactSet) (bool, error) {
	for _, name := range artifacts.names() {
		want, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			return false, fmt.Errorf("%w: cannot read %s", ErrChannelBlocked, name)
		}
		got, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil {
			return false, nil
		}
		if !bytes.Equal(want, got) {
			return false, nil
		}
	}
	return true, nil
}

// stageRelease writes the four artifacts into a temporary directory and renames
// it into place, so a failure never leaves half a version in the channel.
func stageRelease(sourceDir, destination string, artifacts artifactSet) error {
	parent := filepath.Dir(destination)
	staging, err := os.MkdirTemp(parent, ".aiah-publish-*")
	if err != nil {
		return fmt.Errorf("%w: cannot stage the release", ErrChannelBlocked)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()

	for _, name := range artifacts.names() {
		body, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			return fmt.Errorf("%w: cannot read %s", ErrChannelBlocked, name)
		}
		if err := os.WriteFile(filepath.Join(staging, name), body, 0o644); err != nil {
			return fmt.Errorf("%w: cannot stage %s", ErrChannelBlocked, name)
		}
	}
	if err := os.Rename(staging, destination); err != nil {
		return fmt.Errorf("%w: cannot install the release", ErrChannelBlocked)
	}
	cleanup = false
	return nil
}

func regularFile(candidate string) bool {
	info, err := os.Lstat(candidate)
	return err == nil && info.Mode().IsRegular()
}
