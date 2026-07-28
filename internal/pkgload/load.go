package pkgload

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/build"
)

var (
	// ErrInvalidPackage means the package path is missing or not a valid asset package.
	ErrInvalidPackage = errors.New("invalid package")
)

const (
	maxArchiveSize = 512 << 20
	maxMemberSize  = 64 << 20
	maxMembers     = 10000
)

// Package is an opened asset package (from .tar or an extracted directory).
type Package struct {
	Manifest build.PackageManifest
	Lock     build.LockFile
	// Files maps package-relative paths (forward slashes) to content.
	Files map[string][]byte
	// Source is the user-supplied package path (tar or directory).
	Source string
}

// Open loads and verifies a package from a .tar archive or extracted directory.
func Open(input string) (Package, error) {
	if input == "" {
		return Package{}, ErrInvalidPackage
	}
	absolute, err := filepath.Abs(input)
	if err != nil {
		return Package{}, ErrInvalidPackage
	}
	info, err := os.Lstat(absolute)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return Package{}, ErrInvalidPackage
	}
	if info.IsDir() {
		return openDir(absolute)
	}
	if !info.Mode().IsRegular() || info.Size() > maxArchiveSize {
		return Package{}, ErrInvalidPackage
	}
	return openTar(absolute)
}

func openDir(input string) (Package, error) {
	dir, err := packageDirectory(input)
	if err != nil {
		return Package{}, err
	}
	manifest, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Package{}, err
	}
	lock, err := readLock(filepath.Join(dir, "lock.json"))
	if err != nil {
		return Package{}, err
	}
	if err := validateMetadata(manifest, lock); err != nil {
		return Package{}, err
	}

	files := make(map[string][]byte, len(lock.Files))
	for _, entry := range lock.Files {
		full, err := securePackageFile(dir, entry.Path)
		if err != nil {
			return Package{}, err
		}
		body, err := readLimitedFile(full)
		if err != nil || fileSHA256(body) != entry.SHA256 {
			return Package{}, ErrInvalidPackage
		}
		files[entry.Path] = body
	}
	if err := validateFiles(lock, files); err != nil {
		return Package{}, err
	}
	return Package{Manifest: manifest, Lock: lock, Files: files, Source: dir}, nil
}

func packageDirectory(input string) (string, error) {
	if regularFile(filepath.Join(input, "manifest.json")) {
		return input, nil
	}
	entries, err := os.ReadDir(input)
	if err != nil {
		return "", ErrInvalidPackage
	}
	var found string
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		candidate := filepath.Join(input, entry.Name())
		if regularFile(filepath.Join(candidate, "manifest.json")) {
			if found != "" {
				return "", ErrInvalidPackage
			}
			found = candidate
		}
	}
	if found == "" {
		return "", ErrInvalidPackage
	}
	return found, nil
}

func openTar(filePath string) (Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Package{}, ErrInvalidPackage
	}
	defer file.Close()

	reader := tar.NewReader(file)
	members := make(map[string][]byte)
	prefix := ""
	layoutSet := false
	count := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Package{}, ErrInvalidPackage
		}
		count++
		if count > maxMembers {
			return Package{}, ErrInvalidPackage
		}
		isDirectory := header.Typeflag == tar.TypeDir
		// The reader normalises the legacy '\x00' type flag to TypeReg, so only
		// regular files and directories can reach here legitimately.
		if !isDirectory && header.Typeflag != tar.TypeReg {
			return Package{}, ErrInvalidPackage
		}
		name, memberPrefix, err := canonicalMemberName(header.Name, isDirectory)
		if err != nil {
			return Package{}, ErrInvalidPackage
		}
		if layoutSet && prefix != memberPrefix {
			return Package{}, ErrInvalidPackage
		}
		layoutSet = true
		prefix = memberPrefix
		if isDirectory {
			continue
		}
		if name == "assets" {
			return Package{}, ErrInvalidPackage
		}
		if header.Size < 0 || header.Size > maxMemberSize {
			return Package{}, ErrInvalidPackage
		}
		if _, duplicate := members[name]; duplicate {
			return Package{}, ErrInvalidPackage
		}
		body, err := io.ReadAll(io.LimitReader(reader, maxMemberSize+1))
		if err != nil || int64(len(body)) != header.Size {
			return Package{}, ErrInvalidPackage
		}
		members[name] = body
	}

	var manifest build.PackageManifest
	if err := decodeJSON(members["manifest.json"], &manifest); err != nil {
		return Package{}, ErrInvalidPackage
	}
	var lock build.LockFile
	if err := decodeJSON(members["lock.json"], &lock); err != nil {
		return Package{}, ErrInvalidPackage
	}
	if err := validateMetadata(manifest, lock); err != nil {
		return Package{}, err
	}
	files := make(map[string][]byte, len(lock.Files))
	lockPaths := make(map[string]bool, len(lock.Files))
	for _, entry := range lock.Files {
		lockPaths[entry.Path] = true
		body, ok := members[entry.Path]
		if !ok || fileSHA256(body) != entry.SHA256 {
			return Package{}, ErrInvalidPackage
		}
		files[entry.Path] = body
	}
	for name := range members {
		if strings.HasPrefix(name, "assets/") && !lockPaths[name] {
			return Package{}, ErrInvalidPackage
		}
	}
	if err := validateFiles(lock, files); err != nil {
		return Package{}, err
	}
	return Package{Manifest: manifest, Lock: lock, Files: files, Source: filePath}, nil
}

func canonicalMemberName(value string, isDirectory bool) (string, string, error) {
	value = strings.TrimPrefix(filepath.ToSlash(value), "./")
	if isDirectory {
		value = strings.TrimSuffix(value, "/")
		if value == "" || value == "." {
			return "", "", nil
		}
	}
	if !safePackagePath(value, false) {
		return "", "", ErrInvalidPackage
	}
	if recognizedMember(value) {
		return value, "", nil
	}
	prefix, rest, ok := strings.Cut(value, "/")
	if isDirectory && !ok {
		return "", prefix, nil
	}
	if !ok || prefix == "" || !recognizedMember(rest) {
		return "", "", ErrInvalidPackage
	}
	return rest, prefix, nil
}

func recognizedMember(value string) bool {
	return value == "manifest.json" || value == "lock.json" ||
		value == "assets" || strings.HasPrefix(value, "assets/")
}
