package build

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type artifactSet struct {
	ArchiveName  string
	ManifestName string
	LockName     string
	SHAName      string
	Archive      []byte
	Manifest     []byte
	Lock         []byte
	SHABody      []byte
	Digest       string
}

func buildArtifacts(
	document workspace.Document,
	profileName string,
	pkgAssets []PackageAsset,
	lockFiles []LockEntry,
	filePayloads map[string][]byte,
	targets []string,
) (artifactSet, error) {
	sort.Slice(pkgAssets, func(i, j int) bool { return pkgAssets[i].ID < pkgAssets[j].ID })
	sort.Slice(lockFiles, func(i, j int) bool { return lockFiles[i].Path < lockFiles[j].Path })

	packageManifest := PackageManifest{
		SchemaVersion: 1,
		Name:          document.Name,
		Version:       document.Version,
		Profile:       profileName,
		Targets:       targets,
		Assets:        pkgAssets,
	}
	lock := LockFile{
		SchemaVersion: 1,
		Name:          document.Name,
		Version:       document.Version,
		Profile:       profileName,
		Files:         lockFiles,
	}

	manifestJSON, err := marshalStable(packageManifest)
	if err != nil {
		return artifactSet{}, err
	}
	lockJSON, err := marshalStable(lock)
	if err != nil {
		return artifactSet{}, err
	}

	// The profile is part of a package's identity: a personal build and a
	// minimal build of the same name and version are different packages. Leaving
	// it out of the artifact name let the second build silently overwrite the
	// first, and the .sha256, manifest and lock left beside it still read as one
	// consistent set.
	baseName := sanitizeFileName(document.Name) + "-" + sanitizeFileName(document.Version) +
		"-" + sanitizeFileName(profileName)
	archiveData, err := writeTar(packageRootName(document.Name, document.Version), manifestJSON, lockJSON, filePayloads)
	if err != nil {
		return artifactSet{}, err
	}
	sum := sha256.Sum256(archiveData)
	digest := hex.EncodeToString(sum[:])
	archiveName := baseName + ".tar"
	return artifactSet{
		ArchiveName:  archiveName,
		ManifestName: baseName + ".manifest.json",
		LockName:     baseName + ".lock.json",
		SHAName:      baseName + ".sha256",
		Archive:      archiveData,
		Manifest:     manifestJSON,
		Lock:         lockJSON,
		SHABody:      []byte(digest + "  " + archiveName + "\n"),
		Digest:       digest,
	}, nil
}

// publishArtifacts writes all package files atomically via a temp directory.
func publishArtifacts(outDir string, artifacts artifactSet) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(outDir, ".aiah-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	writes := []struct {
		name string
		body []byte
	}{
		{artifacts.ArchiveName, artifacts.Archive},
		{artifacts.ManifestName, artifacts.Manifest},
		{artifacts.LockName, artifacts.Lock},
		{artifacts.SHAName, artifacts.SHABody},
	}
	for _, item := range writes {
		if err := os.WriteFile(filepath.Join(tmp, item.name), item.body, 0644); err != nil {
			return err
		}
	}

	// Atomic publish: Write all files to staging, then rename as group.
	// Rename files in stable order to minimize race window.
	// If process dies during this loop, marker absence signals incomplete build.
	for _, item := range writes {
		src := filepath.Join(tmp, item.name)
		dst := filepath.Join(outDir, item.name)
		if err := os.Rename(src, dst); err != nil {
			// Cross-device fallback: write to temporary suffix, then rename.
			data, readErr := os.ReadFile(src)
			if readErr != nil {
				return err
			}
			tmpDst := dst + ".tmp"
			if writeErr := os.WriteFile(tmpDst, data, 0644); writeErr != nil {
				return writeErr
			}
			if renameErr := os.Rename(tmpDst, dst); renameErr != nil {
				return renameErr
			}
		}
	}

	// Write completion marker last (after all artifacts exist).
	// Absence of marker signals incomplete/interrupted build.
	marker := filepath.Join(outDir, "."+artifacts.ArchiveName+".complete")
	return os.WriteFile(marker, []byte{}, 0644)
}

func writeTar(packageRoot string, manifestJSON, lockJSON []byte, files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	modTime := time.Unix(0, 0).UTC()

	writeBytes := func(name string, mode int64, body []byte) error {
		hdr := &tar.Header{
			Name:    filepath.ToSlash(filepath.Join(packageRoot, name)),
			Mode:    mode,
			Size:    int64(len(body)),
			ModTime: modTime,
			Uid:     0,
			Gid:     0,
			Uname:   "",
			Gname:   "",
			Format:  tar.FormatUSTAR,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(body)
		return err
	}

	if err := writeBytes("manifest.json", 0644, manifestJSON); err != nil {
		return nil, err
	}
	if err := writeBytes("lock.json", 0644, lockJSON); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := writeBytes(path, 0644, files[path]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func marshalStable(value any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func packageRootName(name, version string) string {
	return sanitizeFileName(name) + "-" + sanitizeFileName(version)
}

func sanitizeFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "package"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-")
	return replacer.Replace(value)
}
