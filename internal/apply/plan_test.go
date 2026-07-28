package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/adapter"
)

func TestCollapsePlansDetectsContentCollision(t *testing.T) {
	planned := []plannedFile{
		{
			logical: "home/.claude/rules/x.md",
			file: adapter.StagedFile{
				AssetID: "a", RelPath: ".claude/rules/x.md",
				SHA256: "111", Body: []byte("one"),
			},
			root: t.TempDir(),
		},
		{
			logical: "home/.claude/rules/x.md",
			file: adapter.StagedFile{
				AssetID: "b", RelPath: ".claude/rules/x.md",
				SHA256: "222", Body: []byte("two"),
			},
			root: t.TempDir(),
		},
	}
	// fix abs for within - not needed for collapse
	_, findings := collapsePlans(planned)
	if len(findings) == 0 {
		t.Fatal("expected path collision")
	}
	if findings[0].Code != codePathCollision {
		t.Fatalf("code = %s", findings[0].Code)
	}
}

func TestCollapsePlansAllowsIdenticalContent(t *testing.T) {
	planned := []plannedFile{
		{
			logical: "home/.claude/rules/x.md",
			file:    adapter.StagedFile{AssetID: "a", SHA256: "aaa", Body: []byte("same")},
		},
		{
			logical: "home/.claude/rules/x.md",
			file:    adapter.StagedFile{AssetID: "b", SHA256: "aaa", Body: []byte("same")},
		},
	}
	out, findings := collapsePlans(planned)
	if len(findings) != 0 {
		t.Fatalf("findings = %#v", findings)
	}
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
}

func TestDeviceScopeBlocked(t *testing.T) {
	home := t.TempDir()
	_, finding := planInstall(adapter.StagedFile{
		AssetID: "dev.cfg",
		RelPath: ".claude/device.txt",
		Scope:   "device",
		Body:    []byte("x"),
		SHA256:  "x",
	}, home, "")
	if finding == nil || finding.Code != codeDeviceScopeBlocked {
		t.Fatalf("finding = %#v", finding)
	}
}

func TestPathEscapeBlocked(t *testing.T) {
	home := t.TempDir()
	_, finding := planInstall(adapter.StagedFile{
		AssetID: "bad",
		RelPath: "../escape.txt",
		Scope:   "global",
		Body:    []byte("x"),
		SHA256:  "x",
	}, home, "")
	if finding == nil || finding.Code != codePathEscape {
		t.Fatalf("finding = %#v", finding)
	}
}

func TestSymlinkAncestorBlocked(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, ".claude")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, finding := planInstall(adapter.StagedFile{
		AssetID: "rules.bad",
		RelPath: ".claude/rules/x.md",
		Scope:   "global",
		Body:    []byte("x"),
		SHA256:  "x",
	}, home, "")
	if finding == nil || finding.Code != codeSymlinkTarget {
		t.Fatalf("finding = %#v", finding)
	}
	if _, err := os.Stat(filepath.Join(outside, "rules", "x.md")); !os.IsNotExist(err) {
		t.Fatalf("outside path changed: %v", err)
	}
}

// P4 anchor: the leaf itself, not just an ancestor. A pre-existing symlink at
// the destination would otherwise let a write follow it out of the install
// root, and the whole apply must fail rather than skip the file.
func TestSymlinkLeafBlocked(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(t.TempDir(), "target.md")
	if err := os.WriteFile(outside, []byte("original\n"), 0644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "rules"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	leaf := filepath.Join(home, ".claude", "rules", "x.md")
	if err := os.Symlink(outside, leaf); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, finding := planInstall(adapter.StagedFile{
		AssetID: "rules.bad",
		RelPath: ".claude/rules/x.md",
		Scope:   "global",
		Body:    []byte("installed\n"),
		SHA256:  "x",
	}, home, "")
	if finding == nil || finding.Code != codeSymlinkTarget {
		t.Fatalf("finding = %#v", finding)
	}
	body, err := os.ReadFile(outside)
	if err != nil || string(body) != "original\n" {
		t.Fatalf("symlink target followed: body=%q err=%v", body, err)
	}
}
