package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

const (
	driftUnchanged       = "unchanged"
	driftLocallyModified = "locally-modified"
	driftMissing         = "missing"
)

// DoctorOptions identifies the roots whose aiah deployment state is inspected.
type DoctorOptions struct {
	Home    string
	Project string
}

// DoctorReport is the deterministic, read-only deployment health report.
type DoctorReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Kind          string              `json:"kind"`
	ProducedBy    string              `json:"producedBy"`
	Ok            bool                `json:"ok"`
	Summary       DoctorSummary       `json:"summary"`
	Deployment    *DoctorDeployment   `json:"deployment,omitempty"`
	Drift         []DriftEntry        `json:"drift"`
	Findings      []workspace.Finding `json:"findings"`
}

type DoctorSummary struct {
	Deployment      bool `json:"deployment"`
	Journals        int  `json:"journals"`
	Stages          int  `json:"stages"`
	Backups         int  `json:"backups"`
	InvalidBackups  int  `json:"invalidBackups"`
	Unchanged       int  `json:"unchanged"`
	LocallyModified int  `json:"locallyModified"`
	Missing         int  `json:"missing"`
	Unchecked       int  `json:"unchecked"`
}

type DoctorDeployment struct {
	BackupID   string   `json:"backupId"`
	ProducedBy string   `json:"producedBy"`
	Package    string   `json:"package"`
	Version    string   `json:"version"`
	Profile    string   `json:"profile"`
	Targets    []string `json:"targets,omitempty"`
	AppliedAt  string   `json:"appliedAt"`
}

type DriftEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// Doctor inspects journals, stages, backups, deployment drift, and native MCP
// config shapes without changing either install root.
func Doctor(options DoctorOptions) (DoctorReport, error) {
	if options.Home == "" && options.Project == "" {
		return DoctorReport{}, ErrInvalidOptions
	}
	home, project, err := rollbackRoots(RollbackOptions{
		Home: options.Home, Project: options.Project,
	})
	if err != nil {
		return DoctorReport{}, ErrInvalidOptions
	}
	metaRoot := home
	if metaRoot == "" {
		metaRoot = project
	}
	report := DoctorReport{
		SchemaVersion: 1,
		Kind:          "doctor",
		ProducedBy:    version.ProducedBy(),
		Drift:         make([]DriftEntry, 0),
		Findings:      make([]workspace.Finding, 0),
	}

	deploy, present, finding := inspectDeployment(metaRoot)
	if finding != nil {
		report.Findings = append(report.Findings, *finding)
	}
	if present {
		report.Summary.Deployment = true
		report.Deployment = &DoctorDeployment{
			BackupID: deploy.BackupID, ProducedBy: deploy.ProducedBy,
			Package: deploy.Package, Version: deploy.Version,
			Profile: deploy.Profile, Targets: append([]string(nil), deploy.Targets...),
			AppliedAt: deploy.AppliedAt,
		}
	}

	inspectJournals(metaRoot, &report)
	inspectStages(metaRoot, &report)
	validBackups := inspectBackups(metaRoot, &report)
	if present && !validBackups[deploy.BackupID] {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codeMissingBackup,
			Severity: workspace.SeverityError,
			Message:  "The current deployment backup is missing or invalid.",
			Paths:    []string{backupMetadataLogical(deploy.BackupID)},
		})
	}
	if present {
		inspectDrift(deploy, home, project, &report)
	}
	inspectMCPConfigs(home, project, &report)
	return finishDoctor(report), nil
}

func inspectDeployment(metaRoot string) (deployRecord, bool, *workspace.Finding) {
	const logical = ".aiah/deployments/current.json"
	path, err := securePath(metaRoot, logical)
	if err != nil {
		finding := invalidDeploymentFinding()
		return deployRecord{}, false, &finding
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return deployRecord{}, false, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		finding := invalidDeploymentFinding()
		return deployRecord{}, false, &finding
	}
	body, err := os.ReadFile(path)
	if err != nil {
		finding := invalidDeploymentFinding()
		return deployRecord{}, false, &finding
	}
	var deploy deployRecord
	if json.Unmarshal(body, &deploy) != nil || !safeIdentifier(deploy.BackupID) {
		finding := invalidDeploymentFinding()
		return deployRecord{}, false, &finding
	}
	return deploy, true, nil
}

func invalidDeploymentFinding() workspace.Finding {
	return workspace.Finding{
		Code:     codeDeploymentInvalid,
		Severity: workspace.SeverityError,
		Message:  "The current deployment record is unreadable or invalid.",
		Paths:    []string{".aiah/deployments/current.json"},
	}
}

func inspectJournals(metaRoot string, report *DoctorReport) {
	dir, pathErr := securePath(metaRoot, ".aiah")
	if pathErr != nil {
		report.Findings = append(report.Findings, workspace.Finding{
			Code: codeApplyJournalPresent, Severity: workspace.SeverityError,
			Message: "Apply state cannot be inspected safely.", Paths: []string{".aiah"},
		})
		return
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		report.Findings = append(report.Findings, workspace.Finding{
			Code: codeApplyJournalPresent, Severity: workspace.SeverityError,
			Message: "Apply state cannot be inspected.", Paths: []string{".aiah"},
		})
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "apply-journal.json" &&
			(!strings.HasPrefix(name, "apply-journal-") || !strings.HasSuffix(name, ".json")) {
			continue
		}
		report.Summary.Journals++
		logical := filepath.ToSlash(filepath.Join(".aiah", name))
		valid := entry.Type().IsRegular()
		if valid {
			body, readErr := os.ReadFile(filepath.Join(dir, name))
			var record journalRecord
			valid = readErr == nil && json.Unmarshal(body, &record) == nil &&
				safeIdentifier(record.BackupID) &&
				validJournalPaths(record.Committed) && validJournalPaths(record.Pending)
			if valid && name != "apply-journal.json" {
				valid = name == "apply-journal-"+record.BackupID+".json"
			}
		}
		message := "An unfinished apply journal requires inspection before another apply."
		if !valid {
			message = "An unfinished apply journal is unreadable or invalid."
		}
		report.Findings = append(report.Findings, workspace.Finding{
			Code: codeApplyJournalPresent, Severity: workspace.SeverityError,
			Message: message, Paths: []string{logical},
		})
	}
}

func validJournalPaths(paths []string) bool {
	for _, path := range paths {
		if !validLogicalPath(path) {
			return false
		}
	}
	return true
}

func inspectStages(metaRoot string, report *DoctorReport) {
	entries, err := os.ReadDir(metaRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".aiah-stage-") {
			continue
		}
		report.Summary.Stages++
		report.Findings = append(report.Findings, workspace.Finding{
			Code: codeStaleStage, Severity: workspace.SeverityWarning,
			Message: "A staging path from an interrupted apply remains.",
			Paths:   []string{entry.Name()},
		})
	}
}

func inspectBackups(metaRoot string, report *DoctorReport) map[string]bool {
	valid := make(map[string]bool)
	root, pathErr := securePath(metaRoot, ".aiah/backups")
	if pathErr != nil {
		report.Summary.InvalidBackups++
		report.Findings = append(report.Findings, invalidBackupDoctorFinding(".aiah/backups"))
		return valid
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return valid
	}
	if err != nil {
		report.Summary.InvalidBackups++
		report.Findings = append(report.Findings, invalidBackupDoctorFinding(".aiah/backups"))
		return valid
	}
	for _, entry := range entries {
		report.Summary.Backups++
		backupID := entry.Name()
		ok := entry.IsDir() && safeIdentifier(backupID) &&
			validateBackupForDoctor(metaRoot, backupID)
		if ok {
			valid[backupID] = true
			continue
		}
		report.Summary.InvalidBackups++
		report.Findings = append(
			report.Findings,
			invalidBackupDoctorFinding(backupMetadataLogical(backupID)),
		)
	}
	return valid
}

func validateBackupForDoctor(metaRoot, backupID string) bool {
	backupRel := filepath.ToSlash(filepath.Join(".aiah", "backups", backupID))
	backupRoot, err := securePath(metaRoot, backupRel)
	if err != nil {
		return false
	}
	metaPath, err := secureRegularFile(metaRoot, filepath.ToSlash(filepath.Join(backupRel, "backup.json")))
	if err != nil {
		return false
	}
	body, err := os.ReadFile(metaPath)
	if err != nil {
		return false
	}
	var record backupRecord
	if json.Unmarshal(body, &record) != nil || record.ID != backupID || len(record.Files) == 0 {
		return false
	}
	seen := make(map[string]bool, len(record.Files))
	for _, file := range record.Files {
		if (file.Root != "home" && file.Root != "project") ||
			!workspace.SafeRelativePath(file.RelPath) ||
			file.Path != file.Root+"/"+filepath.ToSlash(file.RelPath) ||
			seen[file.Path] ||
			(file.Action != actionCreate && file.Action != actionUpdate) ||
			(file.Action == actionUpdate) != file.BackedUp ||
			file.Mode&^uint32(0777) != 0 {
			return false
		}
		seen[file.Path] = true
		if file.BackedUp {
			if _, err := secureRegularFile(backupRoot, file.Path); err != nil {
				return false
			}
		}
	}
	return true
}

func invalidBackupDoctorFinding(path string) workspace.Finding {
	return workspace.Finding{
		Code: codeInvalidBackup, Severity: workspace.SeverityError,
		Message: "Backup metadata or payload is missing, invalid, or unsafe.",
		Paths:   []string{path},
	}
}

func backupMetadataLogical(backupID string) string {
	return filepath.ToSlash(filepath.Join(".aiah", "backups", backupID, "backup.json"))
}

func inspectDrift(deploy deployRecord, home, project string, report *DoctorReport) {
	if len(deploy.FileStates) == 0 {
		if len(deploy.Files) != 0 {
			report.Summary.Unchecked = len(deploy.Files)
			report.Findings = append(report.Findings, workspace.Finding{
				Code: codeDriftUnavailable, Severity: workspace.SeverityWarning,
				Message: "This deployment predates stored hashes and modes; drift cannot be determined.",
				Paths:   []string{".aiah/deployments/current.json"},
			})
		}
		return
	}
	if !validDeploymentStates(deploy) {
		report.Findings = append(report.Findings, invalidDeploymentFinding())
		report.Summary.Unchecked = len(deploy.FileStates)
		return
	}

	drifted := make([]string, 0)
	for _, state := range deploy.FileStates {
		root, relative, ok := logicalRoot(state.Path, home, project)
		if !ok {
			report.Findings = append(report.Findings, invalidDeploymentFinding())
			report.Summary.Unchecked++
			continue
		}
		status := driftStatus(root, relative, state)
		report.Drift = append(report.Drift, DriftEntry{Path: state.Path, Status: status})
		switch status {
		case driftUnchanged:
			report.Summary.Unchanged++
		case driftLocallyModified:
			report.Summary.LocallyModified++
			drifted = append(drifted, state.Path)
		case driftMissing:
			report.Summary.Missing++
			drifted = append(drifted, state.Path)
		}
	}
	if len(drifted) != 0 {
		sort.Strings(drifted)
		report.Findings = append(report.Findings, workspace.Finding{
			Code: codeDeploymentDrift, Severity: workspace.SeverityWarning,
			Message: "Files from the current deployment are locally modified or missing.",
			Paths:   drifted,
		})
	}
}

func validDeploymentStates(deploy deployRecord) bool {
	files := make(map[string]bool, len(deploy.Files))
	for _, path := range deploy.Files {
		if !validLogicalPath(path) || files[path] {
			return false
		}
		files[path] = true
	}
	states := make(map[string]bool, len(deploy.FileStates))
	for _, state := range deploy.FileStates {
		decoded, err := hex.DecodeString(state.SHA256)
		if !validLogicalPath(state.Path) || states[state.Path] ||
			err != nil || len(decoded) != sha256.Size ||
			state.Mode&^uint32(0777) != 0 {
			return false
		}
		states[state.Path] = true
	}
	for path := range files {
		if !states[path] {
			return false
		}
	}
	return true
}

func validLogicalPath(path string) bool {
	switch {
	case strings.HasPrefix(path, "home/"):
		return workspace.SafeRelativePath(strings.TrimPrefix(path, "home/"))
	case strings.HasPrefix(path, "project/"):
		return workspace.SafeRelativePath(strings.TrimPrefix(path, "project/"))
	default:
		return false
	}
}

func logicalRoot(path, home, project string) (string, string, bool) {
	if strings.HasPrefix(path, "home/") && home != "" {
		return home, strings.TrimPrefix(path, "home/"), true
	}
	if strings.HasPrefix(path, "project/") && project != "" {
		return project, strings.TrimPrefix(path, "project/"), true
	}
	return "", "", false
}

func driftStatus(root, relative string, state deployFileState) string {
	path, err := securePath(root, relative)
	if errors.Is(err, errSymlinkInPath) {
		return driftLocallyModified
	}
	if err != nil {
		return driftLocallyModified
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return driftMissing
	}
	if err != nil || !info.Mode().IsRegular() {
		return driftLocallyModified
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return driftLocallyModified
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != state.SHA256 ||
		uint32(info.Mode().Perm()) != state.Mode {
		return driftLocallyModified
	}
	return driftUnchanged
}

func inspectMCPConfigs(home, project string, report *DoctorReport) {
	type configPath struct {
		root    string
		logical string
		rel     string
	}
	configs := make([]configPath, 0, 6)
	for _, item := range []struct {
		root, scope, prefix string
	}{
		{home, "global", "home/"},
		{project, "project", "project/"},
	} {
		if item.root == "" {
			continue
		}
		seen := make(map[string]bool)
		for _, target := range []string{adapter.TargetClaude, adapter.TargetCodex, adapter.TargetGrok} {
			rel, ok := adapter.NativeMCPConfigPath(target, item.scope)
			if !ok || seen[rel] {
				continue
			}
			seen[rel] = true
			configs = append(configs, configPath{
				root: item.root, rel: rel, logical: item.prefix + rel,
			})
		}
	}
	for _, config := range configs {
		path, err := securePath(config.root, config.rel)
		if errors.Is(err, errSymlinkInPath) {
			report.Findings = append(report.Findings, workspace.Finding{
				Code: codeMCPConfigSymlink, Severity: workspace.SeverityWarning,
				Message: "Native MCP config is a symlink or has a symlink parent; apply will fail closed.",
				Paths:   []string{config.logical},
			})
			continue
		}
		if err != nil {
			report.Findings = append(report.Findings, invalidMCPConfigFinding(config.logical))
			continue
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			report.Findings = append(report.Findings, invalidMCPConfigFinding(config.logical))
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil || len(strings.TrimSpace(string(body))) == 0 {
			report.Findings = append(report.Findings, invalidMCPConfigFinding(config.logical))
			continue
		}
		root := map[string]any{}
		if strings.HasSuffix(config.rel, ".json") {
			err = json.Unmarshal(body, &root)
		} else {
			err = toml.Unmarshal(body, &root)
		}
		if err != nil {
			report.Findings = append(report.Findings, invalidMCPConfigFinding(config.logical))
			continue
		}
		if containsEmptyMCPArgs(root) {
			report.Findings = append(report.Findings, workspace.Finding{
				Code: codeMCPEmptyArgs, Severity: workspace.SeverityWarning,
				Message: "Native MCP config contains an empty args array that may compare unequal to an omitted args field.",
				Paths:   []string{config.logical},
			})
		}
	}
}

func invalidMCPConfigFinding(path string) workspace.Finding {
	return workspace.Finding{
		Code: codeMCPConfigInvalid, Severity: workspace.SeverityWarning,
		Message: "Native MCP config is empty, invalid, or not a regular file; apply will fail closed.",
		Paths:   []string{path},
	}
}

func containsEmptyMCPArgs(root map[string]any) bool {
	for _, key := range []string{"mcpServers", "mcp_servers"} {
		if containsEmptyArgs(root[key]) {
			return true
		}
	}
	return false
}

func containsEmptyArgs(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "args" {
				if args, ok := child.([]any); ok && len(args) == 0 {
					return true
				}
			}
			if containsEmptyArgs(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsEmptyArgs(child) {
				return true
			}
		}
	}
	return false
}

func finishDoctor(report DoctorReport) DoctorReport {
	sort.Slice(report.Drift, func(i, j int) bool {
		return report.Drift[i].Path < report.Drift[j].Path
	})
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return strings.Join(report.Findings[i].Paths, "\x00") <
			strings.Join(report.Findings[j].Paths, "\x00")
	})
	report.Ok = !workspace.HasError(report.Findings)
	return report
}
