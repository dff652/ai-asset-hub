package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestBackupIDsAreUnique(t *testing.T) {
	home := t.TempDir()
	first, _, _, err := prepareBackup(home, "pkg", "1", nil)
	if err != nil {
		t.Fatalf("first backup: %v", err)
	}
	second, _, _, err := prepareBackup(home, "pkg", "1", nil)
	if err != nil {
		t.Fatalf("second backup: %v", err)
	}
	if first == second {
		t.Fatalf("backup ids collided: %q", first)
	}
}

func TestApplyJournalsAreVersionedByBackupID(t *testing.T) {
	home := t.TempDir()
	plans := []plannedFile{{logical: "home/first.txt"}}
	for _, backupID := range []string{
		"20260726T120000.000000000Z",
		"20260726T120001.000000000Z",
	} {
		if err := writeJournal(home, backupID, plans); err != nil {
			t.Fatalf("write journal %s: %v", backupID, err)
		}
	}
	for _, backupID := range []string{
		"20260726T120000.000000000Z",
		"20260726T120001.000000000Z",
	} {
		expected := filepath.Join(home, ".aiah", "apply-journal-"+backupID+".json")
		if _, err := os.Stat(expected); err != nil {
			t.Fatalf("journal %s was overwritten: %v", backupID, err)
		}
	}
}

func TestCommitFailureRestoresCommittedFiles(t *testing.T) {
	home := t.TempDir()
	firstPath := filepath.Join(home, "first.txt")
	if err := os.WriteFile(firstPath, []byte("old\n"), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	plans := []plannedFile{
		{
			file: adapter.StagedFile{
				RelPath: "first.txt", Body: []byte("new\n"), AssetID: "first",
			},
			root: home, logical: "home/first.txt", abs: firstPath, action: actionUpdate,
		},
		{
			file: adapter.StagedFile{
				RelPath: "blocked/second.txt", Body: []byte("second\n"), AssetID: "second",
			},
			root: home, logical: "home/blocked/second.txt",
			abs: filepath.Join(home, "blocked", "second.txt"), action: actionCreate,
		},
	}
	backupID, _, meta, err := prepareBackup(home, "pkg", "1", plans)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "blocked"), []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	written, findings := commitWrites(home, backupID, meta, plans)
	if written != 0 ||
		!hasFindingCode(findings, codePathEscape) ||
		!hasFindingCode(findings, codeApplyFailedRollback) {
		t.Fatalf("commit result written=%d findings=%#v", written, findings)
	}
	body, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if string(body) != "old\n" {
		t.Fatalf("restored body = %q", body)
	}
	if _, err := os.Stat(journalPath(home, backupID)); !os.IsNotExist(err) {
		t.Fatalf("journal should be cleared after successful recovery: %v", err)
	}
}

func TestFailedRecoveryKeepsJournal(t *testing.T) {
	home := t.TempDir()
	firstPath := filepath.Join(home, "first.txt")
	if err := os.WriteFile(firstPath, []byte("old\n"), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	plans := []plannedFile{
		{
			file: adapter.StagedFile{
				RelPath: "first.txt", Body: []byte("new\n"), AssetID: "first",
			},
			root: home, logical: "home/first.txt", abs: firstPath, action: actionUpdate,
		},
		{
			file: adapter.StagedFile{
				RelPath: "blocked/second.txt", Body: []byte("second\n"), AssetID: "second",
			},
			root: home, logical: "home/blocked/second.txt",
			abs: filepath.Join(home, "blocked", "second.txt"), action: actionCreate,
		},
	}
	backupID, backupRoot, meta, err := prepareBackup(home, "pkg", "1", plans)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if err := os.Remove(filepath.Join(backupRoot, "home", "first.txt")); err != nil {
		t.Fatalf("remove backup payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "blocked"), []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	written, findings := commitWrites(home, backupID, meta, plans)
	if written != 1 ||
		!hasFindingCode(findings, codePathEscape) ||
		!hasFindingCode(findings, codeRollbackIncomplete) {
		t.Fatalf("commit result written=%d findings=%#v", written, findings)
	}
	if _, err := os.Stat(journalPath(home, backupID)); err != nil {
		t.Fatalf("journal should remain for recovery: %v", err)
	}
}

func findingCodes(findings []workspace.Finding) map[string]bool {
	out := make(map[string]bool, len(findings))
	for _, finding := range findings {
		out[finding.Code] = true
	}
	return out
}
