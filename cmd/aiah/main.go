package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/mcp"
	"github.com/dff652/ai-asset-hub/internal/tui"
	"github.com/dff652/ai-asset-hub/internal/validate"
	"github.com/dff652/ai-asset-hub/internal/version"
)

const usage = `usage:
  aiah scan [--home PATH] [--project PATH] [--output json]
  aiah validate --manifest PATH [--root PATH] [--output json]
  aiah build --manifest PATH --profile NAME --out DIR [--root PATH] [--output json]
  aiah diff --package PATH [--home PATH] [--project PATH] [--targets LIST] [--output json]
  aiah apply --package PATH [--home PATH] [--project PATH] [--targets LIST] [--dry-run] [--output json]
  aiah rollback [--home PATH] [--project PATH] [--backup ID] [--output json]
  aiah doctor [--home PATH] [--project PATH] [--output json]
  aiah publish --package PATH --channel DIR [--output json]
  aiah pull --channel DIR --name NAME [--version V] [--profile P] --out DIR [--output json]
  aiah versions --channel DIR [--name NAME] [--output json]
  aiah bootstrap --channel DIR --name NAME [--version V] [--profile P] --out DIR [--home PATH] [--project PATH] [--targets LIST]
  aiah ui [--home PATH] [--project PATH] [--workspace PATH] [--package PATH] [--targets LIST]
  aiah mcp
  aiah version [--output json]
`

// stdin is indirected so the stdio-driven subcommands stay testable.
var stdin io.Reader = os.Stdin

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		if len(args) != 1 {
			_, _ = io.WriteString(stderr, usage)
			return 2
		}
		_, _ = io.WriteString(stdout, usage)
		return 0
	case "version", "--version", "-v":
		return runVersion(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "apply":
		return runApply(args[1:], stdout, stderr)
	case "rollback":
		return runRollback(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "publish":
		return runPublish(args[1:], stdout, stderr)
	case "pull":
		return runPull(args[1:], stdout, stderr)
	case "versions":
		return runVersions(args[1:], stdout, stderr)
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
	case "ui":
		return runUI(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdout, stderr)
	default:
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
}

func runUI(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("ui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultHome, err := os.UserHomeDir()
	if err != nil {
		_, _ = io.WriteString(stderr, "aiah: cannot determine home directory\n")
		return 1
	}
	home := flags.String("home", defaultHome, "home directory to scan")
	project := flags.String("project", "", "optional project directory to scan")
	// Without --workspace the UI stays read-only. There is no default: guessing
	// where to write assets is exactly what this tool must not do.
	workspace := flags.String("workspace", "", "asset workspace root; enables composing a manifest")
	pkg := flags.String("package", "", "package .tar or extracted directory; enables diff/apply review")
	targets := flags.String("targets", "", "comma-separated deployment targets (claude,codex,grok)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || (*pkg == "" && *targets != "") {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}

	err = tui.Run(tui.Options{
		Home:      *home,
		Project:   *project,
		Workspace: *workspace,
		Package:   *pkg,
		Targets:   splitCSV(*targets),
		Input:     os.Stdin,
		Output:    stdout,
	})
	if errors.Is(err, tui.ErrNotTTY) {
		_, _ = io.WriteString(
			stderr,
			uiTTYAlternative(*pkg),
		)
		return 1
	}
	if err != nil {
		_, _ = io.WriteString(stderr, "aiah: ui failed\n")
		return 1
	}
	return 0
}

func uiTTYAlternative(pkg string) string {
	if pkg != "" {
		return "aiah: ui requires an interactive TTY; use 'aiah diff --output json' instead\n"
	}
	return "aiah: ui requires an interactive TTY; use 'aiah scan --output json' instead\n"
}

// runMCP serves the read-only MCP surface on stdio. It takes no flags: the
// paths a caller may inspect are tool arguments, so there is no server-level
// switch that could widen what the surface reaches.
func runMCP(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if err := mcp.Run(mcp.Options{In: stdin, Out: stdout, ErrOut: stderr}); err != nil {
		_, _ = io.WriteString(stderr, "aiah: mcp server failed\n")
		return 1
	}
	return 0
}

func runScan(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultHome, err := os.UserHomeDir()
	if err != nil {
		_, _ = io.WriteString(stderr, "aiah: cannot determine home directory\n")
		return 1
	}
	home := flags.String("home", defaultHome, "home directory to scan")
	project := flags.String("project", "", "optional project directory to scan")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}

	report, err := inventory.Scan(inventory.Options{
		Home:    *home,
		Project: *project,
	})
	if err != nil {
		if errors.Is(err, inventory.ErrInvalidRoot) {
			_, _ = io.WriteString(stderr, "aiah: scan root is not an accessible directory\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: scan failed\n")
		}
		return 1
	}
	return writeJSON(stdout, stderr, report)
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifest := flags.String("manifest", "", "path to manifest.yaml or manifest.json")
	root := flags.String("root", "", "workspace root (default: manifest directory)")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *manifest == "" {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}

	report, err := validate.Validate(validate.Options{
		Manifest: *manifest,
		Root:     *root,
	})
	if err != nil {
		if errors.Is(err, validate.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: validate options are invalid\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: validate failed\n")
		}
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifest := flags.String("manifest", "", "path to manifest.yaml or manifest.json")
	profile := flags.String("profile", "", "profile name from the manifest")
	outDir := flags.String("out", "", "directory for package artifacts")
	root := flags.String("root", "", "workspace root (default: manifest directory)")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *manifest == "" || *profile == "" || *outDir == "" {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}

	report, err := build.Build(build.Options{
		Manifest: *manifest,
		Root:     *root,
		Profile:  *profile,
		OutDir:   *outDir,
	})
	if err != nil {
		if errors.Is(err, build.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: build options are invalid\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: build failed\n")
		}
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func runDiff(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pkg := flags.String("package", "", "package .tar or extracted directory")
	home := flags.String("home", "", "target home directory")
	project := flags.String("project", "", "optional project directory for project-scoped assets")
	targets := flags.String("targets", "", "comma-separated targets (claude,codex,grok)")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *pkg == "" || (*home == "" && *project == "") {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := apply.Diff(apply.Options{
		Package: *pkg,
		Home:    *home,
		Project: *project,
		Targets: splitCSV(*targets),
	})
	if err != nil {
		if errors.Is(err, apply.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: diff options are invalid\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: diff failed\n")
		}
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func runApply(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pkg := flags.String("package", "", "package .tar or extracted directory")
	home := flags.String("home", "", "target home directory")
	project := flags.String("project", "", "optional project directory for project-scoped assets")
	targets := flags.String("targets", "", "comma-separated targets (claude,codex,grok)")
	dryRun := flags.Bool("dry-run", false, "plan only; write nothing (same as aiah diff)")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *pkg == "" || (*home == "" && *project == "") {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := apply.Apply(apply.Options{
		Package: *pkg,
		Home:    *home,
		Project: *project,
		Targets: splitCSV(*targets),
		DryRun:  *dryRun,
	})
	if err != nil {
		if errors.Is(err, apply.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: apply options are invalid\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: apply failed\n")
		}
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func runRollback(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	flags.SetOutput(stderr)
	home := flags.String("home", "", "target home directory")
	project := flags.String("project", "", "project directory (if used at apply)")
	backup := flags.String("backup", "", "backup id (default: current deployment)")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || (*home == "" && *project == "") {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := apply.Rollback(apply.RollbackOptions{
		Home:     *home,
		Project:  *project,
		BackupID: *backup,
	})
	if err != nil {
		if errors.Is(err, apply.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: rollback options are invalid\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: rollback failed\n")
		}
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultHome, err := os.UserHomeDir()
	if err != nil {
		_, _ = io.WriteString(stderr, "aiah: cannot determine home directory\n")
		return 1
	}
	home := flags.String("home", defaultHome, "home directory to inspect")
	project := flags.String("project", "", "optional project directory used at apply")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := apply.Doctor(apply.DoctorOptions{
		Home: *home, Project: *project,
	})
	if err != nil {
		if errors.Is(err, apply.ErrInvalidOptions) {
			_, _ = io.WriteString(stderr, "aiah: doctor root is not an accessible directory\n")
		} else {
			_, _ = io.WriteString(stderr, "aiah: doctor failed\n")
		}
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

// runPublish copies a built package into a channel directory. The channel is a
// plain directory -- moving it between machines is git's, rsync's or a USB
// stick's job, not aiah's (ADR-0007 §1).
func runPublish(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pkg := flags.String("package", "", "built package .tar; its three sibling artifacts must be alongside it")
	channelDir := flags.String("channel", "", "channel directory")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *pkg == "" || *channelDir == "" {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := channel.Publish(channel.PublishOptions{Package: *pkg, Channel: *channelDir})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aiah: %s\n", err.Error())
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

// runPull retrieves a published package and verifies it. It writes only --out;
// installing is still a separate, human-confirmed apply.
func runPull(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("pull", flag.ContinueOnError)
	flags.SetOutput(stderr)
	channelDir := flags.String("channel", "", "channel directory")
	name := flags.String("name", "", "package name")
	pkgVersion := flags.String("version", "", "package version (default: most recently published)")
	profile := flags.String("profile", "", "profile (default: whichever matched)")
	outDir := flags.String("out", "", "directory to write the retrieved artifacts to")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *channelDir == "" || *name == "" || *outDir == "" {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := channel.Pull(channel.PullOptions{
		Channel: *channelDir, Name: *name, Version: *pkgVersion,
		Profile: *profile, Out: *outDir,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aiah: %s\n", err.Error())
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

// runVersions lists a channel in publish order, which is the order an omitted
// --version resolves against.
func runVersions(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("versions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	channelDir := flags.String("channel", "", "channel directory")
	name := flags.String("name", "", "optional package name filter")
	output := flags.String("output", "json", "output format (json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *channelDir == "" {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *output != "json" {
		_, _ = io.WriteString(stderr, "aiah: only --output json is supported\n")
		return 2
	}
	report, err := channel.List(channel.ListOptions{Channel: *channelDir, Name: *name})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aiah: %s\n", err.Error())
		return 1
	}
	if code := writeJSON(stdout, stderr, report); code != 0 {
		return code
	}
	if !report.Ok {
		return 1
	}
	return 0
}

func splitCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseFlagSet(flags *flag.FlagSet, args []string) (bool, int) {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return false, 0
		}
		return false, 2
	}
	return true, 0
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_, _ = fmt.Fprintln(stderr, "aiah: cannot write output")
		return 1
	}
	return 0
}

// runVersion reports the build identity. Machine output mirrors the producedBy
// string that every report and deployment record carries.
func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "text", "output format (text|json)")
	if ok, code := parseFlagSet(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	switch *output {
	case "text":
		_, _ = fmt.Fprintln(stdout, version.Full())
	case "json":
		// Same header shape as every other report: schemaVersion is a number,
		// and encoding goes through writeJSON so escaping stays uniform.
		return writeJSON(stdout, stderr, struct {
			SchemaVersion int    `json:"schemaVersion"`
			Kind          string `json:"kind"`
			ProducedBy    string `json:"producedBy"`
			Version       string `json:"version"`
			Commit        string `json:"commit"`
			Date          string `json:"date"`
		}{1, "version", version.ProducedBy(), version.Version, version.Commit, version.Date})
	default:
		_, _ = io.WriteString(stderr, "aiah: unsupported output format\n")
		return 2
	}
	return 0
}
