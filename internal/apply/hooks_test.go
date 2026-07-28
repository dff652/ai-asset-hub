package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestExpandHookPoliciesSetsExecutable(t *testing.T) {
	staged := []adapter.StagedFile{
		{
			RelPath: ".claude/hooks/pre-tool.sh",
			Body:    []byte("#!/bin/sh\necho pre\n"),
			SHA256:  "a", AssetID: "hook.pre", Target: "claude", Scope: "global",
		},
		{
			RelPath: ".claude/skills/x/SKILL.md",
			Body:    []byte("# x\n"),
			SHA256:  "b", AssetID: "skill.x", Target: "claude", Scope: "global",
		},
	}
	out, findings := applyHookPolicy(staged, stageContext{})
	if workspace.HasError(findings) {
		t.Fatalf("findings = %#v", findings)
	}
	if out[0].Mode != adapter.ModeExecutable {
		t.Fatalf("hook mode = %o", out[0].Mode)
	}
	if out[1].Mode != 0 {
		t.Fatalf("non-hook mode = %o, want unchanged zero", out[1].Mode)
	}
}

func TestExpandHookPoliciesRejectsMissingShebang(t *testing.T) {
	staged := []adapter.StagedFile{{
		RelPath: ".claude/hooks/pre-tool.sh",
		Body:    []byte("echo bare\n"),
		SHA256:  "x", AssetID: "hook.bad", Target: "claude", Scope: "global",
	}}
	_, findings := applyHookPolicy(staged, stageContext{})
	if !workspace.HasError(findings) {
		t.Fatal("expected error")
	}
	found := false
	for _, f := range findings {
		if f.Code == codeHookInvalid {
			found = true
		}
	}
	if !found {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestApplyInstallsHookExecutable(t *testing.T) {
	// Uses workspace-2b package via build in phase2b; light unit on commitWrites.
	home := t.TempDir()
	hookPath := filepath.Join(home, ".claude", "hooks", "pre-tool.sh")
	plans := []plannedFile{{
		file: adapter.StagedFile{
			RelPath: ".claude/hooks/pre-tool.sh",
			Body:    []byte("#!/bin/sh\necho ok\n"),
			Mode:    adapter.ModeExecutable,
			AssetID: "hook.pre",
		},
		root: home, logical: "home/.claude/hooks/pre-tool.sh", abs: hookPath, action: actionCreate,
	}}
	backupID, _, meta, err := prepareBackup(home, "pkg", "1", plans)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	written, findings := commitWrites(home, backupID, meta, plans)
	if len(findings) != 0 || written != 1 {
		t.Fatalf("commit written=%d findings=%#v", written, findings)
	}
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("hook not executable: %o", info.Mode().Perm())
	}
}

func TestClassifyChangeDetectsModeDrift(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hook.sh")
	body := []byte("#!/bin/sh\ntrue\n")
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	action, err := classifyChange(path, body, adapter.ModeExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if action != actionUpdate {
		t.Fatalf("action = %s want update for mode drift", action)
	}
}
