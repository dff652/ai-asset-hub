package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/readiness"
)

// runReadiness aggregates existing read-only build and migration preflight
// Core reports with optional evidence stored inside the asset library. It does
// not build, publish, apply, or create evidence.
func runReadiness(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("readiness", flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultHome, err := os.UserHomeDir()
	if err != nil {
		_, _ = io.WriteString(stderr, "aiah: cannot determine home directory\n")
		return 1
	}
	workspaceRoot := flags.String("workspace", "", "asset library directory containing manifest.yaml")
	manifest := flags.String("manifest", "", "optional manifest path inside the asset library")
	profile := flags.String("profile", "", "profile name from the manifest")
	home := flags.String("home", defaultHome, "home directory to inspect for migration prerequisites")
	project := flags.String("project", "", "optional project directory to inspect")
	backupEvidence := flags.String(
		"backup-evidence", "", "optional evidence file under <workspace>/.aiah/evidence",
	)
	restoreExercise := flags.String(
		"restore-exercise", "", "optional exercise file under <workspace>/.aiah/evidence",
	)
	output := flags.String("output", "text", "output format (text|json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *workspaceRoot == "" || *profile == "" {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "text" && *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: unsupported output format\n")
		return 2
	}

	report, err := readiness.Inspect(readiness.Options{
		WorkspaceRoot:       *workspaceRoot,
		ManifestPath:        *manifest,
		Profile:             *profile,
		Home:                *home,
		Project:             *project,
		BackupEvidencePath:  *backupEvidence,
		RestoreExercisePath: *restoreExercise,
	})
	if err != nil {
		if errors.Is(err, readiness.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: readiness options are invalid\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: readiness check failed\n")
		}
		return 1
	}
	if *output == "json" {
		if code := writeJSON(stdout, stderr, report); code != 0 {
			return code
		}
	} else {
		writeReadinessText(stdout, report)
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func writeReadinessText(out io.Writer, report readiness.Report) {
	_, _ = fmt.Fprintf(out, "Migration readiness: %s\n", strings.ToUpper(report.Level))
	_, _ = fmt.Fprintf(
		out,
		"Asset library: %s %s (profile %s)\n",
		displayCLIValue(report.Subject.Name),
		displayCLIValue(report.Subject.Version),
		displayCLIValue(report.Subject.Profile),
	)
	_, _ = fmt.Fprintf(
		out,
		"Package: %s (%d assets, %d files)\n",
		report.PackageReadiness.Status,
		report.PackageReadiness.AssetCount,
		report.PackageReadiness.FileCount,
	)
	_, _ = fmt.Fprintf(
		out,
		"Migration preflight: %s (targets: %s, missing secrets: %d, degraded: %d)\n",
		report.MigrationPreflight.Status,
		displayCLIList(report.MigrationPreflight.Targets),
		report.MigrationPreflight.MissingSecrets,
		report.MigrationPreflight.DegradedItems,
	)
	_, _ = fmt.Fprintf(out, "External copy evidence: %s\n", report.BackupEvidence.Status)
	_, _ = fmt.Fprintf(out, "Restore exercise: %s\n", report.RestoreExercise.Status)
	for _, finding := range report.Findings {
		_, _ = fmt.Fprintf(out, "- %s [%s]: %s\n", finding.Severity, finding.Code, finding.Message)
	}
	switch {
	case report.Level == readiness.LevelBlocked:
		_, _ = io.WriteString(out, "Next: fix the package or migration preflight findings, then check again.\n")
	case report.BackupEvidence.Status != readiness.StatusRecorded:
		_, _ = io.WriteString(out, "Next: store the package outside this device and provide an external-copy evidence file.\n")
	case report.RestoreExercise.Status != readiness.StatusPassed:
		_, _ = io.WriteString(out, "Next: complete and provide an isolated restore-exercise record.\n")
	default:
		_, _ = io.WriteString(out, "Next: rerun this check after changing the selected assets.\n")
	}
}

func displayCLIValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func displayCLIList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ",")
}
