package pkgload

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
)

func TestOpenDirectoryPackage(t *testing.T) {
	dir := writeValidPackage(t)
	pkg, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(pkg.Files["assets/rules/common.md"]) != "# Common\n" {
		t.Fatalf("files = %#v", pkg.Files)
	}
	if pkg.ArchiveSHA256 != "" {
		t.Fatalf("directory package has archive digest %q", pkg.ArchiveSHA256)
	}
}

func TestOpenDirRejectsLockPathEscape(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	body := []byte("outside\n")
	if err := os.WriteFile(outside, body, 0644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	sum := sha256.Sum256(body)
	writeJSON(t, filepath.Join(dir, "manifest.json"), build.PackageManifest{
		SchemaVersion: 1, Name: "pkg", Version: "1", Profile: "p",
		Targets: []string{"claude"},
		Assets: []build.PackageAsset{{
			ID: "rules.common", Type: "rules", Scope: "global",
			Portability: "portable", Sensitivity: "public",
			Targets: []string{"claude"},
			Files: []build.PackageFile{{
				Path: "../outside.txt", SHA256: hex.EncodeToString(sum[:]),
			}},
		}},
	})
	writeJSON(t, filepath.Join(dir, "lock.json"), build.LockFile{
		SchemaVersion: 1, Name: "pkg", Version: "1", Profile: "p",
		Files: []build.LockEntry{{
			Path: "../outside.txt", SHA256: hex.EncodeToString(sum[:]),
		}},
	})
	if _, err := Open(dir); err == nil {
		t.Fatal("expected path escape rejection")
	}
}

func TestOpenDirRejectsSymlinkPayload(t *testing.T) {
	dir := writeValidPackage(t)
	payload := filepath.Join(dir, "assets", "rules", "common.md")
	if err := os.Remove(payload); err != nil {
		t.Fatalf("remove payload: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Common\n"), 0644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, payload); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("expected symlink payload rejection")
	}
}

func TestOpenRejectsManifestLockMismatch(t *testing.T) {
	dir := writeValidPackage(t)
	lockPath := filepath.Join(dir, "lock.json")
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	var lock build.LockFile
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("decode lock: %v", err)
	}
	lock.Version = "other"
	writeJSON(t, lockPath, lock)
	if _, err := Open(dir); err == nil {
		t.Fatal("expected manifest/lock mismatch rejection")
	}
}

func TestOpenRejectsUnsafePackageName(t *testing.T) {
	dir := writeValidPackage(t)
	for _, name := range []string{"manifest.json", "lock.json"} {
		filePath := filepath.Join(dir, name)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		value["name"] = "../outside"
		writeJSON(t, filePath, value)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("expected unsafe package name rejection")
	}
}

func TestOpenRejectsInvalidAssetMetadata(t *testing.T) {
	dir := writeValidPackage(t)
	manifestPath := filepath.Join(dir, "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest build.PackageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest.Assets[0].Scope = "elsewhere"
	writeJSON(t, manifestPath, manifest)
	if _, err := Open(dir); err == nil {
		t.Fatal("expected invalid asset metadata rejection")
	}
}

func TestOpenTarRejectsDuplicateMember(t *testing.T) {
	dir := writeValidPackage(t)
	manifest, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	lock, _ := os.ReadFile(filepath.Join(dir, "lock.json"))
	payload, _ := os.ReadFile(filepath.Join(dir, "assets", "rules", "common.md"))
	tarPath := filepath.Join(t.TempDir(), "duplicate.tar")
	file, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	tw := tar.NewWriter(file)
	for _, item := range []struct {
		name string
		body []byte
	}{
		{"pkg/manifest.json", manifest},
		{"pkg/manifest.json", manifest},
		{"pkg/lock.json", lock},
		{"pkg/assets/rules/common.md", payload},
	} {
		if err := tw.WriteHeader(&tar.Header{
			Name: item.name, Mode: 0644, Size: int64(len(item.body)),
		}); err != nil {
			t.Fatalf("header: %v", err)
		}
		if _, err := tw.Write(item.body); err != nil {
			t.Fatalf("body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	if _, err := Open(tarPath); err == nil {
		t.Fatal("expected duplicate member rejection")
	}
}

func TestOpenTarAcceptsConsistentRootDirectoryEntries(t *testing.T) {
	dir := writeValidPackage(t)
	manifest, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	lock, _ := os.ReadFile(filepath.Join(dir, "lock.json"))
	payload, _ := os.ReadFile(filepath.Join(dir, "assets", "rules", "common.md"))
	tarPath := writeTestTar(t, []tarMember{
		{name: "pkg/", directory: true},
		{name: "pkg/assets/", directory: true},
		{name: "pkg/assets/rules/", directory: true},
		{name: "pkg/manifest.json", body: manifest},
		{name: "pkg/lock.json", body: lock},
		{name: "pkg/assets/rules/common.md", body: payload},
	})
	pkg, err := Open(tarPath)
	if err != nil {
		t.Fatalf("open tar with directory entries: %v", err)
	}
	body, err := os.ReadFile(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if pkg.ArchiveSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("archive digest = %q", pkg.ArchiveSHA256)
	}
}

func TestOpenTarRejectsMixedRootLayouts(t *testing.T) {
	dir := writeValidPackage(t)
	manifest, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	lock, _ := os.ReadFile(filepath.Join(dir, "lock.json"))
	payload, _ := os.ReadFile(filepath.Join(dir, "assets", "rules", "common.md"))
	tarPath := writeTestTar(t, []tarMember{
		{name: "manifest.json", body: manifest},
		{name: "pkg/lock.json", body: lock},
		{name: "pkg/assets/rules/common.md", body: payload},
	})
	if _, err := Open(tarPath); err == nil {
		t.Fatal("expected mixed root layout rejection")
	}
}

// P4 test anchors: the tar defenses below were correct but unpinned, so a
// refactor could have removed them without a red test.

func TestOpenTarRejectsParentTraversalMembers(t *testing.T) {
	dir := writeValidPackage(t)
	manifest, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	lock, _ := os.ReadFile(filepath.Join(dir, "lock.json"))
	payload, _ := os.ReadFile(filepath.Join(dir, "assets", "rules", "common.md"))

	for _, name := range []string{
		"pkg/assets/../../escape.md",
		"../escape.md",
		"pkg/assets/rules/../../../escape.md",
		"/etc/escape.md",
	} {
		t.Run(name, func(t *testing.T) {
			tarPath := writeTestTar(t, []tarMember{
				{name: "pkg/manifest.json", body: manifest},
				{name: "pkg/lock.json", body: lock},
				{name: "pkg/assets/rules/common.md", body: payload},
				{name: name, body: []byte("payload\n")},
			})
			if _, err := Open(tarPath); err == nil {
				t.Fatalf("member %q was accepted", name)
			}
		})
	}
}

// The hostile member impersonates the one asset the metadata declares, and the
// metadata records the empty digest a link member actually carries. Without the
// typeflag allowlist the package would load, so this pins that check alone.
func TestOpenTarRejectsLinkMembers(t *testing.T) {
	manifest, lock := packageMetadata(t, map[string][]byte{"assets/rules/common.md": {}})

	for _, link := range []struct {
		name     string
		typeflag byte
		linkname string
	}{
		{"symlink to absolute path", tar.TypeSymlink, "/etc/passwd"},
		{"symlink escaping the package", tar.TypeSymlink, "../../../../etc/passwd"},
		{"hard link to another member", tar.TypeLink, "pkg/manifest.json"},
	} {
		t.Run(link.name, func(t *testing.T) {
			tarPath := writeTestTar(t, []tarMember{
				{name: "pkg/manifest.json", body: manifest},
				{name: "pkg/lock.json", body: lock},
				{name: "pkg/assets/rules/common.md", typeflag: link.typeflag, linkname: link.linkname},
			})
			if _, err := Open(tarPath); err == nil {
				t.Fatalf("%s member was accepted", link.name)
			}
		})
	}
}

// The member is genuinely one byte over the cap and the metadata matches its
// digest, so only the size check can reject it. That costs a 64MiB fixture,
// hence the -short skip; a header-only forgery would be rejected as a truncated
// archive instead and would pin nothing.
func TestOpenTarRejectsOversizedMember(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 64MiB fixture")
	}
	huge := make([]byte, maxMemberSize+1)
	manifest, lock := packageMetadata(t, map[string][]byte{"assets/rules/huge.md": huge})
	tarPath := writeTestTar(t, []tarMember{
		{name: "pkg/manifest.json", body: manifest},
		{name: "pkg/lock.json", body: lock},
		{name: "pkg/assets/rules/huge.md", body: huge},
	})
	if _, err := Open(tarPath); err == nil {
		t.Fatal("member above the size cap was accepted")
	}
}

// Every filler is declared in the metadata, so the package is internally
// consistent and only the member-count cap can reject it.
func TestOpenTarRejectsTooManyMembers(t *testing.T) {
	assets := make(map[string][]byte, maxMembers)
	for i := 0; i < maxMembers; i++ {
		assets[fmt.Sprintf("assets/rules/filler-%d.md", i)] = []byte(fmt.Sprintf("filler %d\n", i))
	}
	manifest, lock := packageMetadata(t, assets)

	members := []tarMember{
		{name: "pkg/manifest.json", body: manifest},
		{name: "pkg/lock.json", body: lock},
	}
	for path, body := range assets {
		members = append(members, tarMember{name: "pkg/" + path, body: body})
	}
	if len(members) <= maxMembers {
		t.Fatalf("fixture has %d members, want more than %d", len(members), maxMembers)
	}
	if _, err := Open(writeTestTar(t, members)); err == nil {
		t.Fatal("member count above the limit was accepted")
	}
}

type tarMember struct {
	name      string
	body      []byte
	directory bool
	typeflag  byte
	linkname  string
}

// packageMetadata renders manifest.json and lock.json that agree with the given
// asset bodies. Hostile-tar tests need this: a package that fails the lock
// check would be rejected no matter what the defense under test does.
func packageMetadata(t *testing.T, assets map[string][]byte) (manifestJSON, lockJSON []byte) {
	t.Helper()
	paths := make([]string, 0, len(assets))
	for path := range assets {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	manifest := build.PackageManifest{
		SchemaVersion: 1, Name: "pkg", Version: "1", Profile: "p",
		Targets: []string{"claude"},
	}
	lock := build.LockFile{SchemaVersion: 1, Name: "pkg", Version: "1", Profile: "p"}
	for index, path := range paths {
		sum := sha256.Sum256(assets[path])
		digest := hex.EncodeToString(sum[:])
		manifest.Assets = append(manifest.Assets, build.PackageAsset{
			ID: fmt.Sprintf("rules.asset-%d", index), Type: "rules", Path: path,
			Scope: "global", Portability: "portable", Sensitivity: "public",
			Targets: []string{"claude"},
			Files:   []build.PackageFile{{Path: path, SHA256: digest}},
		})
		lock.Files = append(lock.Files, build.LockEntry{Path: path, SHA256: digest})
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	lockJSON, err = json.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	return manifestJSON, lockJSON
}

func writeTestTar(t *testing.T, members []tarMember) string {
	t.Helper()
	tarPath := filepath.Join(t.TempDir(), "package.tar")
	file, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	tw := tar.NewWriter(file)
	for _, member := range members {
		header := &tar.Header{
			Name: member.name,
			Mode: 0644,
			Size: int64(len(member.body)),
		}
		if member.directory {
			header.Typeflag = tar.TypeDir
			header.Mode = 0755
		}
		if member.typeflag != 0 {
			header.Typeflag = member.typeflag
			header.Linkname = member.linkname
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("header %s: %v", member.name, err)
		}
		if len(member.body) > 0 {
			if _, err := tw.Write(member.body); err != nil {
				t.Fatalf("body %s: %v", member.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return tarPath
}

func writeValidPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := []byte("# Common\n")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	payload := filepath.Join(dir, "assets", "rules", "common.md")
	if err := os.MkdirAll(filepath.Dir(payload), 0755); err != nil {
		t.Fatalf("mkdir payload: %v", err)
	}
	if err := os.WriteFile(payload, body, 0644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	writeJSON(t, filepath.Join(dir, "manifest.json"), build.PackageManifest{
		SchemaVersion: 1, Name: "pkg", Version: "1", Profile: "p",
		Targets: []string{"claude"},
		Assets: []build.PackageAsset{{
			ID: "rules.common", Type: "rules", Path: "assets/rules/common.md",
			Scope: "global", Portability: "portable", Sensitivity: "public",
			Targets: []string{"claude"},
			Files: []build.PackageFile{{
				Path: "assets/rules/common.md", SHA256: digest,
			}},
		}},
	})
	writeJSON(t, filepath.Join(dir, "lock.json"), build.LockFile{
		SchemaVersion: 1, Name: "pkg", Version: "1", Profile: "p",
		Files: []build.LockEntry{{
			Path: "assets/rules/common.md", SHA256: digest,
		}},
	})
	return dir
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
