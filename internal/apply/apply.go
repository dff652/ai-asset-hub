package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/pkgload"
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

var (
	// ErrInvalidOptions means apply flags are incomplete or unusable.
	ErrInvalidOptions = errors.New("invalid apply options")
)

// Options configures package installation into a home and/or project directory.
//
// Missing install roots are strict: a project-scoped asset without --project
// (or global without --home) fails the entire apply. Device-scoped assets never apply.
type Options struct {
	Package string
	// ExpectedSHA256 optionally binds Diff and Apply to immutable archive bytes
	// selected by a channel pull. Extracted package directories cannot satisfy
	// this contract.
	ExpectedSHA256 string
	Home           string
	// Project is optional. Project-scoped assets install here when set.
	Project string
	Targets []string
	// DryRun only computes the plan and diff; no files are written.
	DryRun bool
}

// Report is the deterministic JSON report for apply/diff.
type Report struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Kind          string                  `json:"kind"`
	ProducedBy    string                  `json:"producedBy"`
	Ok            bool                    `json:"ok"`
	DryRun        bool                    `json:"dryRun"`
	Home          string                  `json:"homeLogical,omitempty"`
	Project       string                  `json:"projectLogical,omitempty"`
	Package       string                  `json:"package"`
	Targets       []string                `json:"targets"`
	Summary       Summary                 `json:"summary"`
	Compile       []adapter.CompileReport `json:"compile"`
	Changes       []Change                `json:"changes"`
	BackupID      string                  `json:"backupId,omitempty"`
	Findings      []workspace.Finding     `json:"findings"`
}

type Summary struct {
	Staged    int `json:"staged"`
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Written   int `json:"written"`
	Skipped   int `json:"skipped"`
}

type Change struct {
	Path   string `json:"path"` // logical home/... or project/...
	Action string `json:"action"`
	SHA256 string `json:"sha256"`
}

const (
	actionCreate    = "create"
	actionUpdate    = "update"
	actionUnchanged = "unchanged"
	actionSkipped   = "skipped"
)

// Diff plans installation without writing.
func Diff(options Options) (Report, error) {
	options.DryRun = true
	return Apply(options)
}

// Apply installs a package using staging, backups, and mid-failure rollback.
func Apply(options Options) (Report, error) {
	if options.Package == "" || (options.Home == "" && options.Project == "") {
		return Report{}, ErrInvalidOptions
	}

	var home, project string
	var err error
	if options.Home != "" {
		home, err = absDir(options.Home)
		if err != nil {
			return Report{}, ErrInvalidOptions
		}
	}
	if options.Project != "" {
		project, err = absDir(options.Project)
		if err != nil {
			return Report{}, ErrInvalidOptions
		}
	}

	report := Report{
		SchemaVersion: 1,
		Kind:          "apply",
		ProducedBy:    version.ProducedBy(),
		DryRun:        options.DryRun,
		Package:       filepath.Base(options.Package),
		Findings:      make([]workspace.Finding, 0),
		Changes:       make([]Change, 0),
	}
	if home != "" {
		report.Home = "home"
	}
	if project != "" {
		report.Project = "project"
	}

	pkg, err := pkgload.Open(options.Package)
	if err != nil {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codeInvalidPackage,
			Severity: workspace.SeverityError,
			Message:  "Package could not be opened or failed integrity checks.",
			Paths:    []string{"package"},
		})
		return finish(report), nil
	}
	if options.ExpectedSHA256 != "" && pkg.ArchiveSHA256 != options.ExpectedSHA256 {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codeInvalidPackage,
			Severity: workspace.SeverityError,
			Message:  "Package no longer matches the selected immutable release.",
			Paths:    []string{"package/sha256"},
		})
		return finish(report), nil
	}

	targets, rejectedTargets := adapter.ResolveTargets(options.Targets, pkg.Manifest.Targets)
	for _, target := range rejectedTargets {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codeInvalidTarget,
			Severity: workspace.SeverityError,
			Message:  "Target is unsupported or not present in the package.",
			Paths:    []string{target},
		})
	}
	if len(targets) == 0 {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codeInvalidTarget,
			Severity: workspace.SeverityError,
			Message:  "No supported package target was selected.",
			Paths:    []string{"targets"},
		})
	}
	if workspace.HasError(report.Findings) {
		return finish(report), nil
	}
	staged, compileReports := adapter.CompileTargets(pkg.Manifest, pkg.Files, targets)
	report.Compile = compileReports
	report.Targets = targets

	staged, policyFindings := applyStagePolicies(staged, stageContext{
		home: home, project: project,
	})
	report.Findings = append(report.Findings, policyFindings...)
	if workspace.HasError(report.Findings) {
		return finish(report), nil
	}
	report.Summary.Staged = len(staged)

	planned := make([]plannedFile, 0, len(staged))
	for _, file := range staged {
		item, finding := planInstall(file, home, project)
		if finding != nil {
			report.Findings = append(report.Findings, *finding)
			report.Summary.Skipped++
			report.Changes = append(report.Changes, Change{
				Path:   logicalPath(file),
				Action: actionSkipped,
				SHA256: file.SHA256,
			})
			continue
		}
		planned = append(planned, item)
	}

	collapsed, collisionFindings := collapsePlans(planned)
	report.Findings = append(report.Findings, collisionFindings...)
	if workspace.HasError(report.Findings) {
		return finish(report), nil
	}

	sort.Slice(collapsed, func(i, j int) bool {
		return collapsed[i].logical < collapsed[j].logical
	})

	for i := range collapsed {
		action, err := classifyChange(collapsed[i].abs, collapsed[i].file.Body, collapsed[i].file.Mode)
		if err != nil {
			report.Findings = append(report.Findings, workspace.Finding{
				Code:     codeUnreadable,
				Severity: workspace.SeverityError,
				Message:  "Existing target path is unreadable.",
				Paths:    []string{collapsed[i].logical},
			})
			continue
		}
		collapsed[i].action = action
		report.Changes = append(report.Changes, Change{
			Path:   collapsed[i].logical,
			Action: action,
			SHA256: collapsed[i].file.SHA256,
		})
		switch action {
		case actionCreate:
			report.Summary.Create++
		case actionUpdate:
			report.Summary.Update++
		case actionUnchanged:
			report.Summary.Unchanged++
		}
	}

	if workspace.HasError(report.Findings) {
		return finish(report), nil
	}
	if options.DryRun {
		return finish(report), nil
	}

	toWrite := make([]plannedFile, 0, len(collapsed))
	for _, item := range collapsed {
		if item.action != actionUnchanged {
			toWrite = append(toWrite, item)
		}
	}
	if len(toWrite) == 0 {
		return finish(report), nil
	}

	metaRoot := home
	if metaRoot == "" {
		metaRoot = project
	}

	backupID, _, backupMeta, err := prepareBackup(metaRoot, pkg.Manifest.Name, pkg.Manifest.Version, toWrite)
	if err != nil {
		report.Findings = append(report.Findings, ioFinding("Cannot create backup."))
		return finish(report), nil
	}
	// Always surface backup id once backup exists, including failure paths.
	report.BackupID = backupID

	written, findings := commitWrites(metaRoot, backupID, backupMeta, toWrite)
	if len(findings) != 0 {
		report.Findings = append(report.Findings, findings...)
		report.Summary.Written = written
		return finish(report), nil
	}
	report.Summary.Written = written

	deploy := deployRecord{
		BackupID:   backupID,
		ProducedBy: version.ProducedBy(),
		Package:    pkg.Manifest.Name,
		Version:    pkg.Manifest.Version,
		Profile:    pkg.Manifest.Profile,
		Targets:    report.Targets,
		AppliedAt:  time.Now().UTC().Format(time.RFC3339),
		Files:      sortedLogicals(toWrite),
		FileStates: deploymentFileStates(collapsed),
		Home:       home != "",
		Project:    project != "",
	}
	if err := writeDeployRecord(metaRoot, deploy); err != nil {
		recovered, findings := recoverCommitted(
			metaRoot,
			backupID,
			backupMeta,
			toWrite,
			ioFinding("Files were installed but the deployment record could not be written."),
		)
		report.Summary.Written = recovered
		report.Findings = append(report.Findings, findings...)
		return finish(report), nil
	}
	clearJournal(metaRoot, backupID)
	return finish(report), nil
}

func absDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", ErrInvalidOptions
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", ErrInvalidOptions
	}
	return filepath.Clean(resolved), nil
}

func finish(report Report) Report {
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return report.Findings[i].Message < report.Findings[j].Message
	})
	sort.SliceStable(report.Changes, func(i, j int) bool {
		return report.Changes[i].Path < report.Changes[j].Path
	})
	report.Ok = !workspace.HasError(report.Findings)
	return report
}

func sortedLogicals(files []plannedFile) []string {
	out := logicalsFor(files)
	sort.Strings(out)
	return out
}

func deploymentFileStates(files []plannedFile) []deployFileState {
	out := make([]deployFileState, 0, len(files))
	for _, file := range files {
		sum := sha256.Sum256(file.file.Body)
		out = append(out, deployFileState{
			Path:   file.logical,
			SHA256: hex.EncodeToString(sum[:]),
			Mode:   uint32(fileMode(file.file.Mode).Perm()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
