package channel

import (
	"bytes"
	"errors"
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
// removes only artifacts created by this call and fails closed. Pull never
// touches HOME: it hands back a package path for the user to inspect with diff
// and then apply.
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

	created, err := copyArtifacts(source, outRoot, artifacts)
	if err != nil {
		removeAll(created)
		return report, err
	}

	// Verify what actually landed, not what we believed we copied.
	digest, err := verifyPackage(
		filepath.Join(outRoot, artifacts.archive),
		filepath.Join(outRoot, artifacts.sha),
	)
	if err != nil {
		removeAll(created)
		return report, err
	}
	if release.SHA256 != "" && digest != release.SHA256 {
		removeAll(created)
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

// copyArtifacts never overwrites an output artifact. A complete, byte-identical
// set is an idempotent no-op; a partial or different set is refused before any
// write. O_EXCL closes the preflight-to-write race for a concurrently created
// target.
type artifactCopy struct {
	name   string
	target string
	body   []byte
}

func copyArtifacts(source, destination string, artifacts artifactSet) ([]string, error) {
	copies := make([]artifactCopy, 0, len(artifacts.names()))
	existing := 0
	for _, member := range artifacts.names() {
		sourcePath := filepath.Join(source, member)
		target := filepath.Join(destination, member)

		want, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: cannot read %s from the channel", ErrChannelBlocked, member,
			)
		}
		copies = append(copies, artifactCopy{name: member, target: target, body: want})
		info, err := os.Lstat(target)
		switch {
		case err == nil:
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf(
					"%w: output artifact %s exists but is not a regular file",
					ErrChannelBlocked, member,
				)
			}
			got, err := os.ReadFile(target)
			if err != nil {
				return nil, fmt.Errorf(
					"%w: cannot inspect output artifact %s", ErrChannelBlocked, member,
				)
			}
			existing++
			if !bytes.Equal(got, want) {
				return nil, fmt.Errorf(
					"%w: output artifact %s already exists with different content",
					ErrChannelBlocked, member,
				)
			}
		case !errors.Is(err, os.ErrNotExist):
			return nil, fmt.Errorf(
				"%w: cannot inspect output artifact %s", ErrChannelBlocked, member,
			)
		}
	}
	if existing != 0 {
		if existing == len(copies) {
			return nil, nil
		}
		return nil, fmt.Errorf(
			"%w: output contains only part of the requested artifact set",
			ErrChannelBlocked,
		)
	}

	created := make([]string, 0, len(copies))
	for _, copy := range copies {
		output, err := os.OpenFile(copy.target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return created, fmt.Errorf("%w: cannot create %s", ErrChannelBlocked, copy.name)
		}
		written, err := output.Write(copy.body)
		if err != nil || written != len(copy.body) {
			_ = output.Close()
			_ = os.Remove(copy.target)
			return created, fmt.Errorf("%w: cannot write %s", ErrChannelBlocked, copy.name)
		}
		if err := output.Close(); err != nil {
			_ = os.Remove(copy.target)
			return created, fmt.Errorf("%w: cannot close %s", ErrChannelBlocked, copy.name)
		}
		created = append(created, copy.target)
	}
	return created, nil
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
