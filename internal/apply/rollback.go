package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

// RollbackOptions restores files from a backup created by Apply.
type RollbackOptions struct {
	Home     string
	Project  string
	BackupID string // empty means latest current.json backup
}

// RollbackReport is emitted by rollback.
type RollbackReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Kind          string              `json:"kind"`
	ProducedBy    string              `json:"producedBy"`
	Ok            bool                `json:"ok"`
	BackupID      string              `json:"backupId,omitempty"`
	Restored      []string            `json:"restored"`
	Removed       []string            `json:"removed"`
	Findings      []workspace.Finding `json:"findings"`
}

type rollbackFile struct {
	meta backupFile
	abs  string
	src  string
}

// Rollback restores previous content for files recorded in a backup.
func Rollback(options RollbackOptions) (RollbackReport, error) {
	if options.Home == "" && options.Project == "" {
		return RollbackReport{}, ErrInvalidOptions
	}
	home, project, err := rollbackRoots(options)
	if err != nil {
		return RollbackReport{}, ErrInvalidOptions
	}
	metaRoot := home
	if metaRoot == "" {
		metaRoot = project
	}

	report := RollbackReport{
		SchemaVersion: 1,
		Kind:          "rollback",
		ProducedBy:    version.ProducedBy(),
		Restored:      make([]string, 0),
		Removed:       make([]string, 0),
		Findings:      make([]workspace.Finding, 0),
	}

	backupID := options.BackupID
	if backupID == "" {
		deploy, err := readCurrentDeploy(metaRoot)
		if err != nil {
			report.Findings = append(report.Findings, workspace.Finding{
				Code:     codeMissingDeployment,
				Severity: workspace.SeverityError,
				Message:  "No current deployment record found.",
				Paths:    []string{".aiah/deployments/current.json"},
			})
			return finishRollback(report), nil
		}
		backupID = deploy.BackupID
	}
	report.BackupID = backupID
	if !safeIdentifier(backupID) {
		report.Findings = append(report.Findings, invalidBackupFinding())
		return finishRollback(report), nil
	}

	backupRel := filepath.ToSlash(filepath.Join(".aiah", "backups", backupID))
	backupRoot, err := securePath(metaRoot, backupRel)
	if err != nil {
		report.Findings = append(report.Findings, invalidBackupFinding())
		return finishRollback(report), nil
	}
	metaRel := filepath.ToSlash(filepath.Join(backupRel, "backup.json"))
	metaPath, err := secureRegularFile(metaRoot, metaRel)
	if err != nil {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codeMissingBackup,
			Severity: workspace.SeverityError,
			Message:  "Backup metadata was not found.",
			Paths:    []string{metaRel},
		})
		return finishRollback(report), nil
	}
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		report.Findings = append(report.Findings, invalidBackupFinding())
		return finishRollback(report), nil
	}
	var meta backupRecord
	if err := json.Unmarshal(raw, &meta); err != nil || meta.ID != backupID || len(meta.Files) == 0 {
		report.Findings = append(report.Findings, invalidBackupFinding())
		return finishRollback(report), nil
	}

	planned, findings := planRollbackFiles(backupRoot, meta, home, project)
	report.Findings = append(report.Findings, findings...)
	if workspace.HasError(report.Findings) {
		return finishRollback(report), nil
	}

	for _, file := range planned {
		if file.meta.BackedUp {
			if err := copyFile(file.src, file.abs, backupMode(file.meta.Mode)); err != nil {
				report.Findings = append(report.Findings, workspace.Finding{
					Code:     codeApplyIO,
					Severity: workspace.SeverityError,
					Message:  "Cannot restore backed up file.",
					Paths:    []string{file.meta.Path},
				})
				continue
			}
			report.Restored = append(report.Restored, file.meta.Path)
			continue
		}
		if err := os.Remove(file.abs); err != nil && !os.IsNotExist(err) {
			report.Findings = append(report.Findings, workspace.Finding{
				Code:     codeApplyIO,
				Severity: workspace.SeverityError,
				Message:  "Cannot remove file created by apply.",
				Paths:    []string{file.meta.Path},
			})
			continue
		}
		report.Removed = append(report.Removed, file.meta.Path)
	}

	sort.Strings(report.Restored)
	sort.Strings(report.Removed)
	if !workspace.HasError(report.Findings) {
		if current, err := readCurrentDeploy(metaRoot); err == nil && current.BackupID == backupID {
			_ = os.Remove(filepath.Join(metaRoot, ".aiah", "deployments", "current.json"))
		}
	}
	return finishRollback(report), nil
}

func rollbackRoots(options RollbackOptions) (string, string, error) {
	var home, project string
	var err error
	if options.Home != "" {
		home, err = absDir(options.Home)
		if err != nil {
			return "", "", err
		}
	}
	if options.Project != "" {
		project, err = absDir(options.Project)
		if err != nil {
			return "", "", err
		}
	}
	return home, project, nil
}

func planRollbackFiles(
	backupRoot string,
	meta backupRecord,
	home, project string,
) ([]rollbackFile, []workspace.Finding) {
	planned := make([]rollbackFile, 0, len(meta.Files))
	findings := make([]workspace.Finding, 0)
	seen := make(map[string]bool)
	for _, file := range meta.Files {
		var root string
		switch file.Root {
		case "home":
			root = home
		case "project":
			root = project
		default:
			findings = append(findings, invalidBackupFinding())
			continue
		}
		if root == "" {
			findings = append(findings, workspace.Finding{
				Code:     codeMissingInstallRoot,
				Severity: workspace.SeverityError,
				Message:  "Backup entry root is not available on this rollback.",
				Paths:    []string{file.Path},
			})
			continue
		}
		if (file.Action != actionCreate && file.Action != actionUpdate) ||
			(file.Action == actionUpdate) != file.BackedUp ||
			file.Mode&^uint32(0777) != 0 {
			findings = append(findings, invalidBackupFinding())
			continue
		}
		expected := file.Root + "/" + filepath.ToSlash(file.RelPath)
		if file.Path != expected || seen[file.Path] {
			findings = append(findings, invalidBackupFinding())
			continue
		}
		seen[file.Path] = true
		absolute, err := securePath(root, file.RelPath)
		if err != nil {
			findings = append(findings, invalidBackupFinding())
			continue
		}
		item := rollbackFile{meta: file, abs: absolute}
		if file.BackedUp {
			source, err := secureRegularFile(backupRoot, file.Path)
			if err != nil {
				findings = append(findings, invalidBackupFinding())
				continue
			}
			item.src = source
		}
		planned = append(planned, item)
	}
	return planned, findings
}

func invalidBackupFinding() workspace.Finding {
	return workspace.Finding{
		Code:     codeInvalidBackup,
		Severity: workspace.SeverityError,
		Message:  "Backup metadata is invalid or contains unsafe paths.",
		Paths:    []string{"backup.json"},
	}
}

func finishRollback(report RollbackReport) RollbackReport {
	sort.SliceStable(report.Findings, func(i, j int) bool {
		return report.Findings[i].Code < report.Findings[j].Code
	})
	report.Ok = !workspace.HasError(report.Findings)
	return report
}

func readCurrentDeploy(metaRoot string) (deployRecord, error) {
	path, err := secureRegularFile(metaRoot, ".aiah/deployments/current.json")
	if err != nil {
		return deployRecord{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return deployRecord{}, err
	}
	var deploy deployRecord
	if err := json.Unmarshal(raw, &deploy); err != nil {
		return deployRecord{}, err
	}
	if strings.TrimSpace(deploy.BackupID) == "" || !safeIdentifier(deploy.BackupID) {
		return deployRecord{}, os.ErrNotExist
	}
	return deploy, nil
}
