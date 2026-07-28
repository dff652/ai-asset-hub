package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsDeploymentDrift(t *testing.T) {
	home := t.TempDir()
	backupID := "20260726T120000.000000000Z"
	target := filepath.Join(home, ".claude", "hooks", "managed.sh")
	mustWriteDoctorFile(t, target, "#!/bin/sh\n", 0755)
	writeDoctorBackup(t, home, backupID, backupRecord{
		ID:        backupID,
		CreatedAt: "2026-07-26T12:00:00Z",
		Package:   "fixture@1",
		Files: []backupFile{{
			Path: "home/.claude/hooks/managed.sh", Root: "home",
			RelPath: ".claude/hooks/managed.sh", Action: actionCreate,
		}},
	})
	deploy := deployRecord{
		BackupID: backupID, ProducedBy: "aiah test", Package: "fixture",
		Version: "1", Profile: "personal", AppliedAt: "2026-07-26T12:00:00Z",
		Files: []string{"home/.claude/hooks/managed.sh"},
		FileStates: []deployFileState{{
			Path:   "home/.claude/hooks/managed.sh",
			SHA256: doctorSHA256([]byte("#!/bin/sh\n")),
			Mode:   0755,
		}},
		Home: true,
	}
	if err := writeDeployRecord(home, deploy); err != nil {
		t.Fatalf("write deploy: %v", err)
	}

	report, err := Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor unchanged: %v", err)
	}
	if !report.Ok || report.Summary.Unchanged != 1 || len(report.Drift) != 1 ||
		report.Drift[0].Status != driftUnchanged {
		t.Fatalf("unchanged report = %#v", report)
	}

	mustWriteDoctorFile(t, target, "#!/bin/sh\necho changed\n", 0755)
	report, err = Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor modified: %v", err)
	}
	if !report.Ok || report.Summary.LocallyModified != 1 ||
		report.Drift[0].Status != driftLocallyModified ||
		!hasFindingCode(report.Findings, codeDeploymentDrift) {
		t.Fatalf("modified report = %#v", report)
	}

	mustWriteDoctorFile(t, target, "#!/bin/sh\n", 0644)
	report, err = Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor mode drift: %v", err)
	}
	if report.Summary.LocallyModified != 1 ||
		report.Drift[0].Status != driftLocallyModified {
		t.Fatalf("mode drift report = %#v", report)
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("remove target: %v", err)
	}
	report, err = Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor missing: %v", err)
	}
	if !report.Ok || report.Summary.Missing != 1 ||
		report.Drift[0].Status != driftMissing {
		t.Fatalf("missing report = %#v", report)
	}
}

func TestDoctorAcceptsChangedFilesAsSubsetOfManagedState(t *testing.T) {
	deploy := deployRecord{
		Files: []string{"home/changed.txt"},
		FileStates: []deployFileState{
			{Path: "home/changed.txt", SHA256: doctorSHA256([]byte("changed")), Mode: 0644},
			{Path: "home/unchanged.txt", SHA256: doctorSHA256([]byte("unchanged")), Mode: 0644},
		},
	}
	if !validDeploymentStates(deploy) {
		t.Fatalf("changed files should be a subset of managed file states: %#v", deploy)
	}
}

func TestDoctorReportsRecoveryArtifactsAndMCPRisks(t *testing.T) {
	home := t.TempDir()
	backupID := "20260726T120000.000000000Z"
	if err := writeJournal(home, backupID, []plannedFile{{logical: "home/a.txt"}}); err != nil {
		t.Fatalf("write journal: %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, ".aiah-stage-leftover"), 0700); err != nil {
		t.Fatalf("mkdir stage: %v", err)
	}
	mustWriteDoctorFile(
		t,
		filepath.Join(home, ".claude.json"),
		"{\"mcpServers\":{\"demo\":{\"command\":\"demo\",\"args\":[]}}}\n",
		0600,
	)

	report, err := Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	codes := findingCodes(report.Findings)
	for _, code := range []string{
		codeApplyJournalPresent,
		codeStaleStage,
		codeMCPEmptyArgs,
	} {
		if !codes[code] {
			t.Fatalf("missing %s in %#v", code, report.Findings)
		}
	}
	if report.Ok {
		t.Fatalf("preserved journal must fail doctor: %#v", report)
	}
	for _, path := range []string{
		journalPath(home, backupID),
		filepath.Join(home, ".aiah-stage-leftover"),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("doctor changed diagnostic artifact %s: %v", path, err)
		}
	}
}

func TestDoctorRejectsMetadataAndMCPConfigSymlinks(t *testing.T) {
	t.Run("metadata root", func(t *testing.T) {
		home := t.TempDir()
		outside := t.TempDir()
		mustWriteDoctorFile(
			t,
			filepath.Join(outside, "apply-journal-secret-name.json"),
			"{}\n",
			0644,
		)
		if err := os.Symlink(outside, filepath.Join(home, ".aiah")); err != nil {
			t.Fatalf("symlink metadata: %v", err)
		}
		report, err := Doctor(DoctorOptions{Home: home})
		if err != nil {
			t.Fatalf("doctor: %v", err)
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		if report.Ok || !hasFindingCode(report.Findings, codeDeploymentInvalid) ||
			strings.Contains(string(encoded), "secret-name") {
			t.Fatalf("doctor followed metadata symlink: %s", encoded)
		}
	})

	t.Run("native MCP config", func(t *testing.T) {
		home := t.TempDir()
		outside := filepath.Join(t.TempDir(), "claude.json")
		mustWriteDoctorFile(t, outside, "{}\n", 0600)
		if err := os.Symlink(outside, filepath.Join(home, ".claude.json")); err != nil {
			t.Fatalf("symlink mcp config: %v", err)
		}
		report, err := Doctor(DoctorOptions{Home: home})
		if err != nil {
			t.Fatalf("doctor: %v", err)
		}
		if !hasFindingCode(report.Findings, codeMCPConfigSymlink) {
			t.Fatalf("report = %#v", report)
		}
	})
}

func TestDoctorReportsMissingCurrentBackup(t *testing.T) {
	home := t.TempDir()
	deploy := deployRecord{
		BackupID:   "20260726T120000.000000000Z",
		ProducedBy: "aiah test", Package: "fixture", Version: "1",
		Profile: "personal", AppliedAt: "2026-07-26T12:00:00Z",
		Home: true,
	}
	if err := writeDeployRecord(home, deploy); err != nil {
		t.Fatalf("write deploy: %v", err)
	}
	report, err := Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if report.Ok || !hasFindingCode(report.Findings, codeMissingBackup) {
		t.Fatalf("report = %#v", report)
	}
}

func TestDoctorRejectsBackupWithMissingPayload(t *testing.T) {
	home := t.TempDir()
	backupID := "20260726T120000.000000000Z"
	writeDoctorBackup(t, home, backupID, backupRecord{
		ID: backupID,
		Files: []backupFile{{
			Path: "home/managed.txt", Root: "home", RelPath: "managed.txt",
			Action: actionUpdate, BackedUp: true, Mode: 0644,
		}},
	})
	if err := writeDeployRecord(home, deployRecord{
		BackupID: backupID, ProducedBy: "aiah test", Package: "fixture",
		Version: "1", Profile: "personal", Home: true,
	}); err != nil {
		t.Fatalf("write deploy: %v", err)
	}
	report, err := Doctor(DoctorOptions{Home: home})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if report.Ok || !hasFindingCode(report.Findings, codeInvalidBackup) {
		t.Fatalf("report = %#v", report)
	}
}

func writeDoctorBackup(t *testing.T, home, backupID string, record backupRecord) {
	t.Helper()
	root := filepath.Join(home, ".aiah", "backups", backupID)
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal backup: %v", err)
	}
	mustWriteDoctorFile(t, filepath.Join(root, "backup.json"), string(body)+"\n", 0644)
}

func mustWriteDoctorFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func doctorSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
