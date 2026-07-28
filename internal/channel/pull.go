package channel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PullOptions describes one retrieval.
type PullOptions struct {
	Channel string
	Name    string
	// Version is optional. Empty resolves to the most recently published
	// release for Name -- publish order, not version comparison (ADR-0007 §5).
	Version string
	// Profile is optional; empty accepts whichever profile matched.
	Profile string
	// Out is the directory the artifacts are written to. It is never a tool
	// directory: pull retrieves, apply installs.
	Out string
}

// PullReport says what was retrieved and from where.
type PullReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	ProducedBy    string `json:"producedBy"`
	Ok            bool   `json:"ok"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	SHA256        string `json:"sha256"`
	// ResolvedLatest is true when no version was requested, so the caller can
	// see that publish order picked this one.
	ResolvedLatest bool     `json:"resolvedLatest"`
	Package        string   `json:"package"`
	Files          []string `json:"files"`
}

// Pull copies a published release out of the channel and verifies it.
//
// The archive is checked against its recorded digest after the copy; a mismatch
// removes what was written and fails closed. Pull never touches HOME: it hands
// back a package path for the user to inspect with diff and then apply.
func Pull(options PullOptions) (PullReport, error) {
	report := PullReport{SchemaVersion: 1, Kind: "pull", ProducedBy: producedBy(), Files: []string{}}

	channelRoot, err := requireDirectory(options.Channel)
	if err != nil {
		return report, err
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return report, fmt.Errorf("%w: a package name is required", ErrChannelBlocked)
	}
	outRoot, err := requireDirectory(options.Out)
	if err != nil {
		return report, fmt.Errorf("%w: the output directory is not accessible", ErrChannelBlocked)
	}

	index, err := loadIndex(channelRoot)
	if err != nil {
		return report, err
	}
	release, found := index.find(name, strings.TrimSpace(options.Version), strings.TrimSpace(options.Profile))
	if !found {
		return report, fmt.Errorf("%w: %s", ErrNotFound, describeRequest(options))
	}
	report.Name, report.Version, report.Profile = release.Name, release.Version, release.Profile
	report.ResolvedLatest = strings.TrimSpace(options.Version) == ""

	source := filepath.Join(channelRoot, filepath.FromSlash(release.Path))
	artifacts := artifactsFor(strings.TrimSuffix(release.Archive, ".tar"))
	for _, member := range artifacts.names() {
		if !regularFile(filepath.Join(source, member)) {
			return report, fmt.Errorf(
				"%w: the channel index lists %s %s but %s is missing from the tree",
				ErrChannelBlocked, release.Name, release.Version, member)
		}
	}

	written, err := copyArtifacts(source, outRoot, artifacts)
	if err != nil {
		removeAll(written)
		return report, err
	}

	// Verify what actually landed, not what we believed we copied.
	digest, err := verifyPackage(
		filepath.Join(outRoot, artifacts.archive),
		filepath.Join(outRoot, artifacts.sha),
	)
	if err != nil {
		removeAll(written)
		return report, err
	}
	if release.SHA256 != "" && digest != release.SHA256 {
		removeAll(written)
		return report, fmt.Errorf(
			"%w: %s does not match the digest the channel index recorded",
			ErrChannelBlocked, artifacts.archive)
	}

	report.SHA256 = digest
	report.Package = filepath.Join(outRoot, artifacts.archive)
	report.Files = append(report.Files, artifacts.names()...)
	report.Ok = true
	return report, nil
}

// ListOptions selects which releases to report.
type ListOptions struct {
	Channel string
	// Name is optional; empty lists every release in the channel.
	Name string
}

// ListReport enumerates a channel in publish order.
type ListReport struct {
	SchemaVersion int       `json:"schemaVersion"`
	Kind          string    `json:"kind"`
	ProducedBy    string    `json:"producedBy"`
	Ok            bool      `json:"ok"`
	Channel       string    `json:"channel"`
	Releases      []Release `json:"releases"`
}

// List reports the channel contents in publish order. The order is meaningful:
// it is what an omitted --version resolves against.
func List(options ListOptions) (ListReport, error) {
	report := ListReport{
		SchemaVersion: 1, Kind: "channel", ProducedBy: producedBy(), Releases: []Release{},
	}
	channelRoot, err := requireDirectory(options.Channel)
	if err != nil {
		return report, err
	}
	report.Channel = channelRoot

	index, err := loadIndex(channelRoot)
	if err != nil {
		return report, err
	}
	wanted := strings.TrimSpace(options.Name)
	for _, release := range index.Releases {
		if wanted != "" && release.Name != wanted {
			continue
		}
		report.Releases = append(report.Releases, release)
	}
	report.Ok = true
	return report, nil
}

func copyArtifacts(source, destination string, artifacts artifactSet) ([]string, error) {
	written := make([]string, 0, len(artifacts.names()))
	for _, member := range artifacts.names() {
		body, err := os.ReadFile(filepath.Join(source, member))
		if err != nil {
			return written, fmt.Errorf("%w: cannot read %s from the channel", ErrChannelBlocked, member)
		}
		target := filepath.Join(destination, member)
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return written, fmt.Errorf("%w: cannot write %s", ErrChannelBlocked, member)
		}
		written = append(written, target)
	}
	return written, nil
}

func removeAll(paths []string) {
	for index := len(paths) - 1; index >= 0; index-- {
		_ = os.Remove(paths[index])
	}
}

func describeRequest(options PullOptions) string {
	parts := []string{options.Name}
	if strings.TrimSpace(options.Version) != "" {
		parts = append(parts, options.Version)
	}
	if strings.TrimSpace(options.Profile) != "" {
		parts = append(parts, "profile "+options.Profile)
	}
	return strings.Join(parts, " ")
}
