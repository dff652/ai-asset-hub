package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestRollbackRejectsUnsafeBackupID(t *testing.T) {
	home := t.TempDir()
	report, err := Rollback(RollbackOptions{
		Home:     home,
		BackupID: "../../outside",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if report.Ok || !hasFindingCode(report.Findings, codeInvalidBackup) {
		t.Fatalf("report = %#v", report)
	}
}

func TestRollbackRejectsUnsafeMetadataBeforeMutation(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(filepath.Dir(home), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep\n"), 0644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	backupID := "20260724T120000.000000000Z"
	backupRoot := filepath.Join(home, ".aiah", "backups", backupID)
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}
	meta := backupRecord{
		ID: backupID,
		Files: []backupFile{{
			Path:     "home/../../outside.txt",
			Root:     "home",
			RelPath:  "../../outside.txt",
			Action:   actionCreate,
			BackedUp: false,
		}},
	}
	body, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupRoot, "backup.json"), body, 0644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	report, err := Rollback(RollbackOptions{Home: home, BackupID: backupID})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if report.Ok || !hasFindingCode(report.Findings, codeInvalidBackup) {
		t.Fatalf("report = %#v", report)
	}
	kept, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside: %v", err)
	}
	if string(kept) != "keep\n" {
		t.Fatalf("outside changed: %q", kept)
	}
}

func hasFindingCode(findings []workspace.Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
