package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/tui"
)

var (
	bootstrapInteractive = tui.IsInteractive
	bootstrapPull        = channel.Pull
	bootstrapDeployment  = tui.RunDeployment
)

// runBootstrap retrieves one immutable release, then hands the verified
// package to the existing Phase C review. There is intentionally no --yes or
// non-interactive mode: apply still requires typing "apply" in a real TTY.
func runBootstrap(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	channelDir := flags.String("channel", "", "channel directory")
	name := flags.String("name", "", "package name")
	pkgVersion := flags.String("version", "", "package version (default: most recently published)")
	profile := flags.String("profile", "", "profile (default: whichever matched)")
	outDir := flags.String("out", "", "directory to retain the retrieved artifacts")
	home := flags.String("home", "", "target home directory")
	project := flags.String("project", "", "optional project directory for project-scoped assets")
	targets := flags.String("targets", "", "comma-separated targets (claude,codex,grok)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *channelDir == "" || *name == "" || *outDir == "" ||
		(*home == "" && *project == "") {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if !bootstrapInteractive(stdin, stdout) {
		_, _ = io.WriteString(
			stderr,
			"aiah: bootstrap requires an interactive TTY; use pull, diff, and apply separately\n",
		)
		return 1
	}

	pulled, err := bootstrapPull(channel.PullOptions{
		Channel: *channelDir, Name: *name, Version: *pkgVersion,
		Profile: *profile, Out: *outDir,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aiah: %s\n", err.Error())
		return 1
	}
	if _, err := fmt.Fprintf(
		stdout,
		"retrieved %s %s (%s)\npackage  %s\n",
		pulled.Name,
		pulled.Version,
		pulled.Profile,
		pulled.Package,
	); err != nil {
		_, _ = io.WriteString(stderr, "aiah: cannot write bootstrap output\n")
		return 1
	}

	result, err := bootstrapDeployment(tui.Options{
		Home: *home, Project: *project, Package: pulled.Package,
		Targets: splitCSV(*targets), Input: stdin, Output: stdout,
	})
	if err != nil {
		_, _ = io.WriteString(stderr, "aiah: deployment review failed\n")
		return 1
	}
	if result.Apply == nil {
		if !result.Diff.Ok {
			writeBootstrapFindings(stderr, result.Diff)
			_, _ = io.WriteString(stderr, "aiah: bootstrap stopped because diff did not pass\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: bootstrap apply was not confirmed\n")
		}
		return 1
	}
	if !result.Apply.Ok {
		writeBootstrapFindings(stderr, *result.Apply)
		writeBootstrapRecovery(stdout, *home, *project, *result.Apply)
		_, _ = io.WriteString(stderr, "aiah: bootstrap apply failed\n")
		return 1
	}
	if _, err := fmt.Fprintf(
		stdout,
		"bootstrap complete · written %d\n",
		result.Apply.Summary.Written,
	); err != nil {
		_, _ = io.WriteString(stderr, "aiah: cannot write bootstrap output\n")
		return 1
	}
	if !writeBootstrapRecovery(stdout, *home, *project, *result.Apply) {
		_, _ = io.WriteString(stderr, "aiah: cannot write bootstrap output\n")
		return 1
	}
	return 0
}

func writeBootstrapFindings(output io.Writer, report apply.Report) {
	for _, finding := range report.Findings {
		_, _ = fmt.Fprintf(output, "%s · %s\n%s\n", finding.Code, finding.Severity, finding.Message)
		for _, path := range finding.Paths {
			_, _ = fmt.Fprintf(output, "  %s\n", path)
		}
	}
}

func writeBootstrapRecovery(output io.Writer, home, project string, report apply.Report) bool {
	if report.BackupID == "" {
		_, err := io.WriteString(output, "backupId  — · no rollback needed\n")
		return err == nil
	}
	command := apply.RollbackCommand(apply.RollbackOptions{
		Home: home, Project: project, BackupID: report.BackupID,
	})
	_, err := fmt.Fprintf(output, "backupId  %s\nrollback  %s\n", report.BackupID, command)
	return err == nil
}
