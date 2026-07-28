package build

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readArchiveFile(t *testing.T, archivePath, logicalPath string) []byte {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	wantSuffix := "/" + filepath.ToSlash(logicalPath)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			t.Fatalf("file %q not in archive", logicalPath)
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		name := filepath.ToSlash(hdr.Name)
		if name == filepath.ToSlash(logicalPath) || strings.HasSuffix(name, wantSuffix) {
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read member: %v", err)
			}
			return body
		}
	}
}
