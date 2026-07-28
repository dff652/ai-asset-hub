package apply

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type journalRecord struct {
	BackupID  string   `json:"backupId"`
	Committed []string `json:"committed"` // logical paths successfully installed
	Pending   []string `json:"pending"`
}

func writeJournal(metaRoot, backupID string, pending []plannedFile) error {
	dir, err := securePath(metaRoot, ".aiah")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	paths := make([]string, 0, len(pending))
	for _, item := range pending {
		paths = append(paths, item.logical)
	}
	rec := journalRecord{BackupID: backupID, Committed: []string{}, Pending: paths}
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(journalPath(metaRoot, backupID), append(body, '\n'), 0644)
}

func updateJournal(metaRoot, backupID string, committed, pending []string) error {
	rec := journalRecord{BackupID: backupID, Committed: committed, Pending: pending}
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(journalPath(metaRoot, backupID), append(body, '\n'), 0644)
}

func journalPath(metaRoot, backupID string) string {
	return filepath.Join(metaRoot, ".aiah", "apply-journal-"+backupID+".json")
}

func clearJournal(metaRoot, backupID string) {
	_ = os.Remove(journalPath(metaRoot, backupID))
}

// commitWrites stages all files then renames into place. On failure after any
// successful install, restores already-committed files using backup metadata.
func commitWrites(metaRoot string, backupID string, meta backupRecord, toWrite []plannedFile) (written int, findings []workspace.Finding) {
	if err := writeJournal(metaRoot, backupID, toWrite); err != nil {
		f := ioFinding("Cannot write apply journal.")
		return 0, []workspace.Finding{f}
	}

	stageRoot, err := os.MkdirTemp(metaRoot, ".aiah-stage-*")
	if err != nil {
		clearJournal(metaRoot, backupID)
		f := ioFinding("Cannot create staging directory.")
		return 0, []workspace.Finding{f}
	}
	defer os.RemoveAll(stageRoot)

	for _, item := range toWrite {
		stagePath := filepath.Join(stageRoot, filepath.FromSlash(item.logical))
		if err := os.MkdirAll(filepath.Dir(stagePath), 0755); err != nil {
			clearJournal(metaRoot, backupID)
			f := ioFinding("Cannot create staging path.")
			return 0, []workspace.Finding{f}
		}
		mode := fileMode(item.file.Mode)
		if err := os.WriteFile(stagePath, item.file.Body, mode); err != nil {
			clearJournal(metaRoot, backupID)
			f := ioFinding("Cannot write staged file.")
			return 0, []workspace.Finding{f}
		}
		// WriteFile mode is subject to umask; force the intended bits.
		if err := os.Chmod(stagePath, mode); err != nil {
			clearJournal(metaRoot, backupID)
			f := ioFinding("Cannot set staged file mode.")
			return 0, []workspace.Finding{f}
		}
	}

	committed := make([]plannedFile, 0, len(toWrite))
	pendingLogicals := make([]string, 0, len(toWrite))
	for _, item := range toWrite {
		pendingLogicals = append(pendingLogicals, item.logical)
	}

	for _, item := range toWrite {
		stagePath := filepath.Join(stageRoot, filepath.FromSlash(item.logical))
		absolute, err := securePath(item.root, item.file.RelPath)
		if err != nil || absolute != item.abs {
			f := workspace.Finding{
				Code:     codePathEscape,
				Severity: workspace.SeverityError,
				Message:  "Install path escapes the install root.",
				Paths:    []string{item.logical},
			}
			return recoverCommitted(metaRoot, backupID, meta, committed, f)
		}
		if err := os.MkdirAll(filepath.Dir(item.abs), 0755); err != nil {
			f := ioFinding("Cannot create destination path.")
			return recoverCommitted(metaRoot, backupID, meta, committed, f)
		}
		mode := fileMode(item.file.Mode)
		if err := os.Rename(stagePath, item.abs); err != nil {
			if err := copyFile(stagePath, item.abs, mode); err != nil {
				f := workspace.Finding{
					Code:     codeApplyIO,
					Severity: workspace.SeverityError,
					Message:  "Cannot install staged file.",
					Paths:    []string{item.logical},
				}
				return recoverCommitted(metaRoot, backupID, meta, committed, f)
			}
		}
		committed = append(committed, item)
		written++
		pendingLogicals = pendingLogicals[1:]
		if err := updateJournal(metaRoot, backupID, logicalsFor(committed), pendingLogicals); err != nil {
			f := ioFinding("Cannot update apply journal.")
			return recoverCommitted(metaRoot, backupID, meta, committed, f)
		}
	}

	return written, nil
}

func recoverCommitted(
	metaRoot, backupID string,
	meta backupRecord,
	committed []plannedFile,
	original workspace.Finding,
) (int, []workspace.Finding) {
	if len(committed) == 0 {
		clearJournal(metaRoot, backupID)
		return 0, []workspace.Finding{original}
	}
	if err := restoreCommitted(metaRoot, meta, committed); err != nil {
		finding := workspace.Finding{
			Code:     codeRollbackIncomplete,
			Severity: workspace.SeverityError,
			Message:  "Install failed and automatic recovery was incomplete; the apply journal was preserved.",
			Paths:    []string{backupID},
		}
		return len(committed), []workspace.Finding{original, finding}
	}
	clearJournal(metaRoot, backupID)
	finding := workspace.Finding{
		Code:     codeApplyFailedRollback,
		Severity: workspace.SeverityError,
		Message:  "Install failed mid-apply; committed files were rolled back using the backup.",
		Paths:    []string{backupID},
	}
	return 0, []workspace.Finding{original, finding}
}

func restoreCommitted(metaRoot string, meta backupRecord, committed []plannedFile) error {
	byLogical := make(map[string]backupFile, len(meta.Files))
	for _, entry := range meta.Files {
		byLogical[entry.Path] = entry
	}
	backupRoot := filepath.Join(metaRoot, ".aiah", "backups", meta.ID)
	// reverse order
	for i := len(committed) - 1; i >= 0; i-- {
		item := committed[i]
		entry := byLogical[item.logical]
		absolute, err := securePath(item.root, item.file.RelPath)
		if err != nil || absolute != item.abs {
			return errUnsafePath
		}
		if entry.BackedUp {
			src, err := secureRegularFile(backupRoot, item.logical)
			if err != nil {
				return err
			}
			if err := copyFile(src, item.abs, backupMode(entry.Mode)); err != nil {
				return err
			}
			continue
		}
		// create: remove installed file
		if err := os.Remove(item.abs); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func backupMode(mode uint32) os.FileMode {
	if mode == 0 {
		// Backward compatibility for backups created before mode was recorded.
		return 0644
	}
	return os.FileMode(mode)
}

func copyFile(src, dst string, mode os.FileMode) error {
	info, err := os.Lstat(src)
	if err != nil || !info.Mode().IsRegular() {
		return errUnsafePath
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return atomicWriteFile(dst, body, mode)
}

func atomicWriteFile(dst string, body []byte, mode os.FileMode) error {
	if mode == 0 {
		mode = 0644
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".aiah-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, bytes.NewReader(body)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

func fileMode(mode uint32) os.FileMode {
	return os.FileMode(adapter.FileMode(mode))
}

func logicalsFor(files []plannedFile) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.logical)
	}
	return out
}

func ioFinding(message string) workspace.Finding {
	return workspace.Finding{
		Code:     codeApplyIO,
		Severity: workspace.SeverityError,
		Message:  message,
		Paths:    []string{"home"},
	}
}
