package pkgload

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/build"
)

var (
	assetIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
	targetPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

func validateMetadata(manifest build.PackageManifest, lock build.LockFile) error {
	if manifest.SchemaVersion != 1 || lock.SchemaVersion != 1 ||
		!safePackageName(manifest.Name) || manifest.Version == "" || manifest.Profile == "" ||
		manifest.Name != lock.Name || manifest.Version != lock.Version ||
		manifest.Profile != lock.Profile || len(manifest.Targets) == 0 ||
		len(manifest.Assets) == 0 {
		return ErrInvalidPackage
	}
	lockPaths := make(map[string]string, len(lock.Files))
	for _, entry := range lock.Files {
		if !safePackagePath(entry.Path, true) || !validSHA256(entry.SHA256) {
			return ErrInvalidPackage
		}
		if existing, duplicate := lockPaths[entry.Path]; duplicate {
			if existing != entry.SHA256 {
				return ErrInvalidPackage
			}
			continue
		}
		lockPaths[entry.Path] = entry.SHA256
	}
	assetIDs := make(map[string]bool, len(manifest.Assets))
	referenced := make(map[string]bool, len(lock.Files))
	packageTargets := make(map[string]bool, len(manifest.Targets))
	for _, target := range manifest.Targets {
		if !targetPattern.MatchString(target) || packageTargets[target] {
			return ErrInvalidPackage
		}
		packageTargets[target] = true
	}
	for _, asset := range manifest.Assets {
		if !validAssetMetadata(asset) || assetIDs[asset.ID] || !safePackagePath(asset.Path, true) ||
			len(asset.Targets) == 0 || len(asset.Files) == 0 {
			return ErrInvalidPackage
		}
		assetIDs[asset.ID] = true
		assetTargets := make(map[string]bool, len(asset.Targets))
		for _, target := range asset.Targets {
			if !packageTargets[target] || assetTargets[target] {
				return ErrInvalidPackage
			}
			assetTargets[target] = true
		}
		for _, file := range asset.Files {
			if !safePackagePath(file.Path, true) || !validSHA256(file.SHA256) ||
				lockPaths[file.Path] != file.SHA256 {
				return ErrInvalidPackage
			}
			if file.Path != asset.Path &&
				!strings.HasPrefix(file.Path, strings.TrimSuffix(asset.Path, "/")+"/") {
				return ErrInvalidPackage
			}
			referenced[file.Path] = true
		}
	}
	if len(referenced) != len(lockPaths) {
		return ErrInvalidPackage
	}
	return nil
}

func validAssetMetadata(asset build.PackageAsset) bool {
	validType := asset.Type == "rules" || asset.Type == "skill" || asset.Type == "memory" ||
		asset.Type == "agent" || asset.Type == "hook" || asset.Type == "mcp"
	validScope := asset.Scope == "global" || asset.Scope == "project" || asset.Scope == "device"
	validPortability := asset.Portability == "portable" || asset.Portability == "adapter-required"
	validSensitivity := asset.Sensitivity == "public" || asset.Sensitivity == "private" ||
		asset.Sensitivity == "sensitive"
	return assetIDPattern.MatchString(asset.ID) && validType && validScope &&
		validPortability && validSensitivity
}

func validateFiles(lock build.LockFile, files map[string][]byte) error {
	for _, entry := range lock.Files {
		body, ok := files[entry.Path]
		if !ok || fileSHA256(body) != entry.SHA256 {
			return ErrInvalidPackage
		}
	}
	return nil
}

func securePackageFile(root, relative string) (string, error) {
	if !safePackagePath(relative, true) {
		return "", ErrInvalidPackage
	}
	full := filepath.Join(root, filepath.FromSlash(relative))
	current := root
	for _, part := range strings.Split(relative, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrInvalidPackage
		}
	}
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxMemberSize {
		return "", ErrInvalidPackage
	}
	return full, nil
}

func safePackagePath(value string, requireAssets bool) bool {
	if value == "" || path.IsAbs(value) || path.Clean(value) != value ||
		value == "." || value == ".." || strings.HasPrefix(value, "../") ||
		strings.Contains(value, "\\") {
		return false
	}
	return !requireAssets || strings.HasPrefix(value, "assets/")
}

func safePackageName(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			continue
		}
		if index > 0 && (char == '.' || char == '_' || char == '-') {
			continue
		}
		return false
	}
	return true
}

func readManifest(filePath string) (build.PackageManifest, error) {
	var manifest build.PackageManifest
	body, err := readLimitedFile(filePath)
	if err != nil || decodeJSON(body, &manifest) != nil {
		return build.PackageManifest{}, ErrInvalidPackage
	}
	return manifest, nil
}

func readLock(filePath string) (build.LockFile, error) {
	var lock build.LockFile
	body, err := readLimitedFile(filePath)
	if err != nil || decodeJSON(body, &lock) != nil {
		return build.LockFile{}, ErrInvalidPackage
	}
	return lock, nil
}

func decodeJSON(body []byte, target any) error {
	if len(body) == 0 {
		return ErrInvalidPackage
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidPackage
	}
	return nil
}

func readLimitedFile(filePath string) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxMemberSize {
		return nil, ErrInvalidPackage
	}
	return os.ReadFile(filePath)
}

func regularFile(filePath string) bool {
	info, err := os.Lstat(filePath)
	return err == nil && info.Mode().IsRegular()
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func fileSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
