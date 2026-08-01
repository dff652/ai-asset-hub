package readiness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/migration"
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

var ErrInvalidOptions = errors.New("invalid readiness options")

// Inspect evaluates build and migration prerequisites, then reads optional
// evidence. It never builds a package, creates evidence, or writes any input
// directory.
func Inspect(options Options) (Report, error) {
	options = normalizeOptions(options)
	report := newReport(options.Profile)
	if options.WorkspaceRoot == "" || options.Profile == "" || options.Home == "" {
		return report, ErrInvalidOptions
	}

	root, err := workspace.ValidateExistingRoot(
		options.WorkspaceRoot,
		options.Home,
		options.Project,
	)
	if err != nil {
		return report, ErrInvalidOptions
	}
	manifestPath := options.ManifestPath
	if manifestPath == "" {
		manifestPath = filepath.Join(root, "manifest.yaml")
	} else if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(root, manifestPath)
	}
	manifestPath, err = secureManifestPath(root, manifestPath)
	if err != nil {
		return report, ErrInvalidOptions
	}

	prepared, buildReport, err := build.Prepare(build.PrepareOptions{
		Manifest: manifestPath,
		Root:     root,
		Profile:  options.Profile,
	})
	if err != nil {
		if errors.Is(err, build.ErrInvalidOptions) {
			return report, ErrInvalidOptions
		}
		return report, err
	}
	report.Findings = append(report.Findings, buildReport.Findings...)
	if !buildReport.Ok {
		report.PackageReadiness.Status = StatusBlocked
		report.MigrationPreflight.Status = StatusBlocked
		markUncheckedEvidence(&report, options)
		return finish(report), nil
	}

	digest, err := selectionDigest(prepared.Manifest)
	if err != nil {
		return report, err
	}
	report.Subject = Subject{
		Name:            prepared.Manifest.Name,
		Version:         prepared.Manifest.Version,
		Profile:         prepared.Manifest.Profile,
		SelectionSHA256: digest,
	}
	report.PackageReadiness = PackageReadiness{
		Status:     StatusReady,
		AssetCount: buildReport.Summary.AssetCount,
		FileCount:  buildReport.Summary.FileCount,
	}

	preflight, err := migration.InspectPreflight(migration.PreflightOptions{
		WorkspaceRoot: root,
		ManifestPath:  manifestPath,
		Profile:       options.Profile,
		Home:          options.Home,
		Project:       options.Project,
	})
	if err != nil {
		if errors.Is(err, build.ErrInvalidOptions) {
			return report, ErrInvalidOptions
		}
		return report, err
	}
	report.Findings = append(report.Findings, preflight.Findings...)
	targets := make([]string, 0, len(preflight.Targets))
	for _, target := range preflight.Targets {
		targets = append(targets, target.Target)
	}
	report.MigrationPreflight = MigrationPreflight{
		Status:             StatusReady,
		Targets:            targets,
		UnsupportedTargets: preflight.Summary.UnsupportedTargets,
		DroppedItems:       preflight.Summary.DroppedItems,
		DegradedItems:      preflight.Summary.DegradedItems,
		SecretReferences:   preflight.Summary.SecretReferences,
		MissingSecrets:     preflight.Summary.MissingSecrets,
		DevicePrivateItems: preflight.Summary.DevicePrivateItems,
	}
	if !preflight.Ok {
		report.MigrationPreflight.Status = StatusBlocked
	} else if preflight.Summary.DegradedItems > 0 {
		report.MigrationPreflight.Status = StatusAttention
	}

	report.BackupEvidence, err = inspectBackupEvidence(root, options.BackupEvidencePath, report.Subject)
	if err != nil {
		return report, err
	}
	report.RestoreExercise, err = inspectRestoreExercise(
		root,
		options.RestoreExercisePath,
		report.Subject,
		report.MigrationPreflight.Targets,
	)
	if err != nil {
		return report, err
	}
	appendEvidenceFindings(&report)
	return finish(report), nil
}

func secureManifestPath(root, value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil || !workspace.Within(root, absolute) {
		return "", ErrInvalidOptions
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalidOptions
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || !workspace.Within(root, resolved) {
		return "", ErrInvalidOptions
	}
	return filepath.Clean(resolved), nil
}

func normalizeOptions(options Options) Options {
	options.WorkspaceRoot = strings.TrimSpace(options.WorkspaceRoot)
	options.ManifestPath = strings.TrimSpace(options.ManifestPath)
	options.Profile = strings.TrimSpace(options.Profile)
	options.Home = strings.TrimSpace(options.Home)
	options.Project = strings.TrimSpace(options.Project)
	options.BackupEvidencePath = strings.TrimSpace(options.BackupEvidencePath)
	options.RestoreExercisePath = strings.TrimSpace(options.RestoreExercisePath)
	return options
}

func newReport(profile string) Report {
	return Report{
		SchemaVersion: 1,
		Kind:          "migration-readiness",
		ProducedBy:    version.ProducedBy(),
		Level:         LevelBlocked,
		Subject:       Subject{Profile: strings.TrimSpace(profile)},
		PackageReadiness: PackageReadiness{
			Status: StatusBlocked,
		},
		MigrationPreflight: MigrationPreflight{
			Status:  StatusBlocked,
			Targets: []string{},
		},
		BackupEvidence:  BackupEvidence{Status: StatusMissing},
		RestoreExercise: RestoreExercise{Status: StatusMissing, Targets: []string{}},
		Findings:        []workspace.Finding{},
	}
}

func selectionDigest(manifest build.PackageManifest) (string, error) {
	// Prepare already sorts selected assets and per-asset targets/files. Marshal
	// the resolved, hashed package manifest rather than the source YAML so asset
	// body changes invalidate evidence even when manifest.yaml is unchanged.
	body, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func markUncheckedEvidence(report *Report, options Options) {
	if options.BackupEvidencePath != "" {
		report.BackupEvidence.Status = StatusUnchecked
	}
	if options.RestoreExercisePath != "" {
		report.RestoreExercise.Status = StatusUnchecked
	}
}

func appendEvidenceFindings(report *Report) {
	appendOne := func(status, kind string) {
		var message string
		switch status {
		case StatusInvalid:
			message = "The selected " + kind + " evidence is invalid or cannot be read safely."
		case StatusMismatch:
			message = "The selected " + kind + " evidence does not match the current asset selection."
		case StatusFailed:
			message = "The recorded restore exercise did not complete every required step."
		default:
			return
		}
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     "readiness_" + strings.ReplaceAll(kind, "-", "_") + "_" + status,
			Severity: workspace.SeverityWarning,
			Message:  message,
			Paths:    []string{"evidence/" + kind},
		})
	}
	appendOne(report.BackupEvidence.Status, "backup")
	appendOne(report.RestoreExercise.Status, "restore")
}

func finish(report Report) Report {
	sort.Strings(report.MigrationPreflight.Targets)
	sort.Strings(report.RestoreExercise.Targets)
	sort.SliceStable(report.Findings, func(i, j int) bool {
		left, right := report.Findings[i], report.Findings[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return strings.Join(left.Paths, "\x00") < strings.Join(right.Paths, "\x00")
	})
	if report.PackageReadiness.Status == StatusBlocked ||
		report.MigrationPreflight.Status == StatusBlocked {
		report.Ok = false
		report.Level = LevelBlocked
		return report
	}
	report.Ok = true
	if report.MigrationPreflight.Status == StatusReady &&
		report.BackupEvidence.Status == StatusRecorded &&
		report.RestoreExercise.Status == StatusPassed {
		report.Level = LevelReady
	} else {
		report.Level = LevelAttention
	}
	return report
}
