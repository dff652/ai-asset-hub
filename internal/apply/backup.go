package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type backupRecord struct {
	ID        string       `json:"id"`
	CreatedAt string       `json:"createdAt"`
	Package   string       `json:"package"`
	Files     []backupFile `json:"files"`
}

type backupFile struct {
	Path     string `json:"path"`
	Root     string `json:"root"` // home | project
	RelPath  string `json:"relPath"`
	Action   string `json:"action"`
	BackedUp bool   `json:"backedUp"`
	Mode     uint32 `json:"mode,omitempty"` // original permission bits for updates
}

type deployRecord struct {
	BackupID string `json:"backupId"`
	// ProducedBy records which aiah binary performed the install. Without it a
	// deployment cannot be traced back to the adapter and classification rules
	// that produced it.
	ProducedBy string            `json:"producedBy"`
	Package    string            `json:"package"`
	Version    string            `json:"version"`
	Profile    string            `json:"profile"`
	Targets    []string          `json:"targets"`
	AppliedAt  string            `json:"appliedAt"`
	Files      []string          `json:"files"`
	FileStates []deployFileState `json:"fileStates,omitempty"`
	Home       bool              `json:"home"`
	Project    bool              `json:"project"`
}

type deployFileState struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

func prepareBackup(metaRoot string, pkgName, pkgVersion string, toWrite []plannedFile) (string, string, backupRecord, error) {
	backupID, backupRoot, err := createBackupRoot(metaRoot)
	if err != nil {
		return "", "", backupRecord{}, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(backupRoot)
		}
	}()
	meta := backupRecord{
		ID:        backupID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Package:   pkgName + "@" + pkgVersion,
		Files:     make([]backupFile, 0, len(toWrite)),
	}
	for _, item := range toWrite {
		entry := backupFile{
			Path:     item.logical,
			Root:     rootKind(item.logical),
			RelPath:  item.file.RelPath,
			Action:   item.action,
			BackedUp: false,
		}
		if item.action == actionUpdate {
			original, err := secureRegularFile(item.root, item.file.RelPath)
			if err != nil {
				return "", "", backupRecord{}, err
			}
			info, err := os.Lstat(original)
			if err != nil {
				return "", "", backupRecord{}, err
			}
			entry.Mode = uint32(info.Mode().Perm())
			dest := filepath.Join(backupRoot, filepath.FromSlash(item.logical))
			if err := copyFile(item.abs, dest, 0600); err != nil {
				return "", "", backupRecord{}, err
			}
			entry.BackedUp = true
		}
		meta.Files = append(meta.Files, entry)
	}
	body, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", "", backupRecord{}, err
	}
	if err := atomicWriteFile(filepath.Join(backupRoot, "backup.json"), append(body, '\n'), 0644); err != nil {
		return "", "", backupRecord{}, err
	}
	complete = true
	return backupID, backupRoot, meta, nil
}

func createBackupRoot(metaRoot string) (string, string, error) {
	parent, err := securePath(metaRoot, ".aiah/backups")
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", "", err
	}
	if _, err := securePath(metaRoot, ".aiah/backups"); err != nil {
		return "", "", err
	}

	base := time.Now().UTC().Format("20060102T150405.000000000Z")
	for attempt := 0; attempt < 1000; attempt++ {
		backupID := base
		if attempt > 0 {
			backupID = fmt.Sprintf("%s-%d", base, attempt)
		}
		backupRoot := filepath.Join(parent, backupID)
		if err := os.Mkdir(backupRoot, 0700); err == nil {
			return backupID, backupRoot, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", "", err
		}
	}
	return "", "", errors.New("cannot allocate backup id")
}

func writeDeployRecord(metaRoot string, deploy deployRecord) error {
	dir, err := securePath(metaRoot, ".aiah/deployments")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(deploy, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	name := fmt.Sprintf("%s-%s.json", deploy.BackupID, deploy.Package)
	if err := atomicWriteFile(filepath.Join(dir, name), body, 0644); err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(dir, "current.json"), body, 0644)
}
