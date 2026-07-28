package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/tui"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestRunBootstrapRejectsNonTTYBeforePull(t *testing.T) {
	resetBootstrapDependencies(t)
	bootstrapInteractive = func(io.Reader, io.Writer) bool { return false }
	pullCalled := false
	bootstrapPull = func(channel.PullOptions) (channel.PullReport, error) {
		pullCalled = true
		return channel.PullReport{}, nil
	}

	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"bootstrap", "--channel", t.TempDir(), "--name", "personal",
		"--out", out, "--home", t.TempDir(),
	}, &stdout, &stderr)
	if code != 1 || pullCalled {
		t.Fatalf("exit=%d pullCalled=%v stdout=%q stderr=%q",
			code, pullCalled, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "interactive TTY") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunBootstrapPullsThenOpensDeploymentReview(t *testing.T) {
	resetBootstrapDependencies(t)
	bootstrapInteractive = func(io.Reader, io.Writer) bool { return true }
	channelDir, out, home, project := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	pkg := buildBootstrapPackage(t)

	bootstrapPull = func(options channel.PullOptions) (channel.PullReport, error) {
		want := channel.PullOptions{
			Channel: channelDir, Name: "personal", Version: "1", Profile: "work", Out: out,
		}
		if !reflect.DeepEqual(options, want) {
			t.Fatalf("pull options = %#v, want %#v", options, want)
		}
		return channel.PullReport{
			Ok: true, Name: "personal", Version: "1", Profile: "work", Package: pkg,
		}, nil
	}
	bootstrapDeployment = func(options tui.Options) (tui.DeploymentResult, error) {
		if options.Package != pkg || options.Home != home || options.Project != project ||
			!reflect.DeepEqual(options.Targets, []string{"claude", "codex"}) {
			t.Fatalf("deployment options = %#v", options)
		}
		for _, relative := range []string{".aiah", ".claude", ".codex", ".agents"} {
			if _, err := os.Stat(filepath.Join(home, relative)); !os.IsNotExist(err) {
				t.Fatalf("bootstrap wrote %s before opening confirmation: %v", relative, err)
			}
		}
		report := apply.Report{
			Ok: true, Targets: []string{"claude", "codex"},
			Summary: apply.Summary{Written: 2}, BackupID: "backup-123",
		}
		return tui.DeploymentResult{
			Diff:  apply.Report{Ok: true, DryRun: true},
			Apply: &report,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"bootstrap", "--channel", channelDir, "--name", "personal",
		"--version", "1", "--profile", "work", "--out", out,
		"--home", home, "--project", project, "--targets", "claude,codex",
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"bootstrap complete", "backupId  backup-123",
		"--home " + home, "--project " + project, "--backup backup-123",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout omits %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunBootstrapStopsWhenDeploymentWasNotConfirmed(t *testing.T) {
	resetBootstrapDependencies(t)
	bootstrapInteractive = func(io.Reader, io.Writer) bool { return true }
	bootstrapPull = successfulBootstrapPull
	bootstrapDeployment = func(tui.Options) (tui.DeploymentResult, error) {
		return tui.DeploymentResult{Diff: apply.Report{Ok: true, DryRun: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run(bootstrapArgs(t), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "not confirmed") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "bootstrap complete") {
		t.Fatalf("cancelled bootstrap reported success: %q", stdout.String())
	}
}

func TestRunBootstrapRepeatsCoreFindingsAfterFailedDiff(t *testing.T) {
	resetBootstrapDependencies(t)
	bootstrapInteractive = func(io.Reader, io.Writer) bool { return true }
	bootstrapPull = successfulBootstrapPull
	finding := workspace.Finding{
		Code:     "symlink_target",
		Severity: workspace.SeverityError,
		Message:  "Core message must stay verbatim.",
		Paths:    []string{"home/.claude"},
	}
	bootstrapDeployment = func(tui.Options) (tui.DeploymentResult, error) {
		return tui.DeploymentResult{
			Diff: apply.Report{Ok: false, DryRun: true, Findings: []workspace.Finding{finding}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run(bootstrapArgs(t), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		string(finding.Code), string(finding.Severity), finding.Message, finding.Paths[0],
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr omits %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunBootstrapNoOpNeedsNoRollback(t *testing.T) {
	resetBootstrapDependencies(t)
	bootstrapInteractive = func(io.Reader, io.Writer) bool { return true }
	bootstrapPull = successfulBootstrapPull
	bootstrapDeployment = func(tui.Options) (tui.DeploymentResult, error) {
		report := apply.Report{Ok: true, Summary: apply.Summary{Unchanged: 1}}
		return tui.DeploymentResult{
			Diff: apply.Report{Ok: true, DryRun: true}, Apply: &report,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	if code := run(bootstrapArgs(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "backupId  —") ||
		!strings.Contains(stdout.String(), "no rollback needed") {
		t.Fatalf("no-op output is ambiguous:\n%s", stdout.String())
	}
}

func resetBootstrapDependencies(t *testing.T) {
	t.Helper()
	originalInteractive := bootstrapInteractive
	originalPull := bootstrapPull
	originalDeployment := bootstrapDeployment
	t.Cleanup(func() {
		bootstrapInteractive = originalInteractive
		bootstrapPull = originalPull
		bootstrapDeployment = originalDeployment
	})
}

func successfulBootstrapPull(channel.PullOptions) (channel.PullReport, error) {
	return channel.PullReport{
		Ok: true, Name: "personal", Version: "1", Profile: "personal",
		Package: "/tmp/personal-1-personal.tar",
	}, nil
}

func bootstrapArgs(t *testing.T) []string {
	t.Helper()
	return []string{
		"bootstrap", "--channel", t.TempDir(), "--name", "personal",
		"--out", t.TempDir(), "--home", t.TempDir(),
	}
}

func buildBootstrapPackage(t *testing.T) string {
	t.Helper()
	out := t.TempDir()
	report, err := build.Build(build.Options{
		Manifest: filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"),
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil || !report.Ok {
		t.Fatalf("build: err=%v report=%#v", err, report)
	}
	return filepath.Join(out, report.Package.Archive)
}
