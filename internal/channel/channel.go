// Package channel publishes and retrieves immutable asset packages through a
// plain directory.
//
// A channel is a directory: on a USB stick, a mounted NAS or netdisk, or inside
// a git checkout. Moving that directory between machines is git's, rsync's or a
// USB stick's job -- this package deliberately implements no network transport
// (ADR-0007 §1). What it does implement is the part none of those provide:
// immutability, a predictable layout, and integrity checks on both ends.
package channel

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/pkgload"
	"github.com/dff652/ai-asset-hub/internal/version"
)

// ErrChannelBlocked means the request was refused and nothing was written.
var ErrChannelBlocked = errors.New("channel operation blocked")

// ErrNotFound means the requested release is not in the channel.
var ErrNotFound = errors.New("release not found in channel")

// indexName is the channel index. Append order is publish order; see
// ADR-0007 §5 for why nothing here compares version strings.
const indexName = "channel.json"

const packagesDir = "packages"

// maxIndexBytes bounds the index read. A channel index holds one short record
// per release, so anything larger is corruption rather than a big channel.
const maxIndexBytes = 8 << 20

// coordinatePattern keeps name/version/profile usable as single path segments.
// A release coordinate becomes a directory name, so anything that could escape
// or collide is refused rather than sanitised.
var coordinatePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Release is one published (name, version, profile) triple.
type Release struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Profile string `json:"profile"`
	Archive string `json:"archive"`
	SHA256  string `json:"sha256"`
	// Path is channel-relative, so the index stays valid when the channel is
	// copied to another machine or mount point.
	Path        string `json:"path"`
	PublishedBy string `json:"publishedBy"`
}

// Index is the channel's list of releases in publish order.
type Index struct {
	SchemaVersion int       `json:"schemaVersion"`
	Kind          string    `json:"kind"`
	Releases      []Release `json:"releases"`
}

// artifactSet is the four files build produces for one package.
type artifactSet struct {
	base     string
	archive  string
	lock     string
	manifest string
	sha      string
}

func artifactsFor(base string) artifactSet {
	return artifactSet{
		base:     base,
		archive:  base + ".tar",
		lock:     base + ".lock.json",
		manifest: base + ".manifest.json",
		sha:      base + ".sha256",
	}
}

func (a artifactSet) names() []string {
	return []string{a.archive, a.lock, a.manifest, a.sha}
}

func loadIndex(channelRoot string) (Index, error) {
	index := Index{SchemaVersion: 1, Kind: "channel", Releases: []Release{}}
	body, err := os.ReadFile(filepath.Join(channelRoot, indexName))
	if errors.Is(err, os.ErrNotExist) {
		return index, nil
	}
	if err != nil {
		return index, fmt.Errorf("%w: cannot read the channel index", ErrChannelBlocked)
	}
	if len(body) > maxIndexBytes {
		return index, fmt.Errorf("%w: the channel index is implausibly large", ErrChannelBlocked)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&index); err != nil {
		return index, fmt.Errorf("%w: the channel index does not parse", ErrChannelBlocked)
	}
	if index.SchemaVersion != 1 || index.Kind != "channel" {
		return index, fmt.Errorf("%w: unrecognised channel index", ErrChannelBlocked)
	}
	if index.Releases == nil {
		index.Releases = []Release{}
	}
	return index, nil
}

func writeIndex(channelRoot string, index Index) error {
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: cannot encode the channel index", ErrChannelBlocked)
	}
	body = append(body, '\n')
	target := filepath.Join(channelRoot, indexName)
	temporary, err := os.CreateTemp(channelRoot, ".aiah-channel-*.json")
	if err != nil {
		return fmt.Errorf("%w: cannot stage the channel index", ErrChannelBlocked)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%w: cannot stage the channel index", ErrChannelBlocked)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%w: cannot stage the channel index", ErrChannelBlocked)
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fmt.Errorf("%w: cannot install the channel index", ErrChannelBlocked)
	}
	return nil
}

// find returns the newest matching release. With no version the answer is the
// last matching entry, which is the most recently published one -- publish
// order, never a comparison of version strings (ADR-0007 §5).
func (i Index) find(name, wantVersion, wantProfile string) (Release, bool) {
	var found Release
	var ok bool
	for _, release := range i.Releases {
		if release.Name != name {
			continue
		}
		if wantVersion != "" && release.Version != wantVersion {
			continue
		}
		if wantProfile != "" && release.Profile != wantProfile {
			continue
		}
		found, ok = release, true
	}
	return found, ok
}

func (i Index) indexOf(name, releaseVersion, profile string) int {
	for position, release := range i.Releases {
		if release.Name == name && release.Version == releaseVersion && release.Profile == profile {
			return position
		}
	}
	return -1
}

func releasePath(name, releaseVersion, profile string) string {
	return path.Join(packagesDir, name, releaseVersion, profile)
}

func producedBy() string { return version.ProducedBy() }

func requireDirectory(candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", fmt.Errorf("%w: a channel directory is required", ErrChannelBlocked)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve the channel directory", ErrChannelBlocked)
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: the channel is not an accessible directory", ErrChannelBlocked)
	}
	return absolute, nil
}

// validCoordinate keeps a name/version/profile safe as one path segment.
func validCoordinate(value string) bool {
	return value != "" && len(value) <= 128 && coordinatePattern.MatchString(value) &&
		!strings.Contains(value, "..")
}

func digestOf(body []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(body))
}

// parseSHAFile reads the "<digest>  <name>" line build writes and returns the
// digest only if it names the expected archive.
func parseSHAFile(body []byte, expectedArchive string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(string(body)))
	if len(fields) != 2 {
		return "", false
	}
	digest, named := fields[0], fields[1]
	if named != expectedArchive || len(digest) != 64 {
		return "", false
	}
	for _, character := range digest {
		isHex := (character >= '0' && character <= '9') ||
			(character >= 'a' && character <= 'f')
		if !isHex {
			return "", false
		}
	}
	return digest, true
}

// verifyPackage checks the archive against its recorded digest and confirms the
// package still loads. A channel that hands out a corrupt package is worse than
// one that refuses to.
func verifyPackage(archivePath, shaPath string) (string, error) {
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot read %s", ErrChannelBlocked, filepath.Base(archivePath))
	}
	shaBody, err := os.ReadFile(shaPath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot read %s", ErrChannelBlocked, filepath.Base(shaPath))
	}
	recorded, ok := parseSHAFile(shaBody, filepath.Base(archivePath))
	if !ok {
		return "", fmt.Errorf("%w: %s is not a usable checksum for %s",
			ErrChannelBlocked, filepath.Base(shaPath), filepath.Base(archivePath))
	}
	actual := digestOf(archive)
	if actual != recorded {
		return "", fmt.Errorf("%w: checksum mismatch for %s",
			ErrChannelBlocked, filepath.Base(archivePath))
	}
	if _, err := pkgload.Open(archivePath); err != nil {
		return "", fmt.Errorf("%w: %s does not load as a package",
			ErrChannelBlocked, filepath.Base(archivePath))
	}
	return actual, nil
}
