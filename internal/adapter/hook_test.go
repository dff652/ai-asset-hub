package adapter

import (
	"strings"
	"testing"
)

func TestEvaluateHookPolicyScriptExecutable(t *testing.T) {
	policy, err := EvaluateHookPolicy(".claude/hooks/pre-tool.sh", []byte("#!/bin/sh\necho ok\n"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if policy.Mode != ModeExecutable {
		t.Fatalf("mode = %o", policy.Mode)
	}
	if policy.Lifecycle != "PreToolUse" {
		t.Fatalf("lifecycle = %q", policy.Lifecycle)
	}
}

func TestEvaluateHookPolicyRejectsMissingShebang(t *testing.T) {
	_, err := EvaluateHookPolicy(".codex/hooks/pre-tool.sh", []byte("echo no shebang\n"))
	if err == nil {
		t.Fatal("expected missing shebang error")
	}
}

func TestEvaluateHookPolicyRejectsSecret(t *testing.T) {
	_, err := EvaluateHookPolicy(".claude/hooks/x.sh", []byte("#!/bin/sh\necho sk-test-raw-secret-value\n"))
	if err == nil {
		t.Fatal("expected secret rejection")
	}
}

func TestEvaluateHookPolicyJSONNonExecutable(t *testing.T) {
	policy, err := EvaluateHookPolicy(".grok/hooks/session-start.json", []byte(`{"hooks":{"SessionStart":[]}}`))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if policy.Mode != ModeFile {
		t.Fatalf("mode = %o", policy.Mode)
	}
	if policy.Lifecycle != "SessionStart" {
		t.Fatalf("lifecycle = %q", policy.Lifecycle)
	}
}

func TestAnnotateHookPolicies(t *testing.T) {
	staged := []StagedFile{
		{RelPath: ".claude/hooks/pre-tool.sh", Body: []byte("#!/bin/sh\ntrue\n"), Target: "claude"},
		{RelPath: ".claude/skills/x/SKILL.md", Body: []byte("# x\n"), Target: "claude"},
	}
	out, notes, err := AnnotateHookPolicies(staged)
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}
	if out[0].Mode != ModeExecutable {
		t.Fatalf("hook mode = %o", out[0].Mode)
	}
	if out[1].Mode != 0 {
		t.Fatalf("non-hook mode = %o, want unchanged zero", out[1].Mode)
	}
	if len(notes) == 0 {
		t.Fatal("expected policy notes")
	}
	if !strings.Contains(notes[0], "0755") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestInferHookLifecycleDoesNotMatchSubstring(t *testing.T) {
	if got := inferHookLifecycle("nonstop-cleanup.sh"); got != "" {
		t.Fatalf("nonstop lifecycle = %q", got)
	}
	if got := inferHookLifecycle("subagent-stop.sh"); got != "SubagentStop" {
		t.Fatalf("subagent-stop lifecycle = %q", got)
	}
}

func TestIsHookInstallPath(t *testing.T) {
	if !IsHookInstallPath(".claude/hooks/pre-tool.sh") {
		t.Fatal("expected hook path")
	}
	if IsHookInstallPath(".claude/skills/x/SKILL.md") {
		t.Fatal("skill is not hook path")
	}
}
