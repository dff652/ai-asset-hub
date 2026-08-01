package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/readiness"
	"github.com/dff652/ai-asset-hub/internal/version"
)

func TestRunReadinessWritesJSONAndFriendlyText(t *testing.T) {
	workspaceRoot := readinessWorkspaceFixture(t)
	home := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"readiness",
		"--workspace", workspaceRoot,
		"--profile", "personal",
		"--home", home,
		"--output", "json",
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report readiness.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not readiness JSON: %v", err)
	}
	if report.Kind != "migration-readiness" || report.Level != readiness.LevelAttention ||
		report.PackageReadiness.Status != readiness.StatusReady ||
		report.BackupEvidence.Status != readiness.StatusMissing {
		t.Fatalf("report = %#v", report)
	}
	if report.ProducedBy != version.ProducedBy() {
		t.Fatalf("producedBy = %q, want %q", report.ProducedBy, version.ProducedBy())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"readiness",
		"--workspace", workspaceRoot,
		"--profile", "personal",
		"--home", home,
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("text exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"Migration readiness: ATTENTION",
		"Package: ready",
		"Migration preflight: ready",
		"External copy evidence: missing",
		"Restore exercise: missing",
		"Next:",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("text output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunReadinessBlockedReportExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"readiness",
		"--workspace", readinessWorkspaceFixture(t),
		"--profile", "missing-profile",
		"--home", t.TempDir(),
		"--output", "json",
	}, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report readiness.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if report.Ok || report.Level != readiness.LevelBlocked ||
		report.PackageReadiness.Status != readiness.StatusBlocked {
		t.Fatalf("blocked report = %#v", report)
	}
}

func TestRunReadinessRejectsArbitraryEvidencePathWithoutLeak(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "private-evidence-name.yaml")
	secret := "sk-private-readiness-cli-must-not-leak"
	if err := os.WriteFile(outside, []byte(secret), 0o600); err != nil {
		t.Fatalf("write outside evidence: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"readiness",
		"--workspace", readinessWorkspaceFixture(t),
		"--profile", "personal",
		"--home", t.TempDir(),
		"--backup-evidence", outside,
		"--output", "json",
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report readiness.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if report.BackupEvidence.Status != readiness.StatusInvalid {
		t.Fatalf("backup evidence = %#v", report.BackupEvidence)
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, outside) || strings.Contains(combined, secret) ||
		strings.Contains(combined, "private-evidence-name") {
		t.Fatalf("command leaked private evidence path or content: %q", combined)
	}
}

func TestRunReadinessRequiresWorkspaceProfileAndKnownOutput(t *testing.T) {
	for _, args := range [][]string{
		{"readiness"},
		{"readiness", "--workspace", "fixture"},
		{"readiness", "--workspace", "fixture", "--profile", "personal", "--output", "yaml"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func readinessWorkspaceFixture(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid"))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	return path
}
