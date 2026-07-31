package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/tui"
	updater "github.com/dff652/ai-asset-hub/internal/update"
	"github.com/dff652/ai-asset-hub/internal/version"
)

func TestRunHelpExitsZero(t *testing.T) {
	tests := [][]string{
		{"--help"},
		{"scan", "--help"},
		{"validate", "--help"},
		{"build", "--help"},
		{"diff", "--help"},
		{"apply", "--help"},
		{"rollback", "--help"},
		{"doctor", "--help"},
		{"ui", "--help"},
		{"mcp", "--help"},
		{"publish", "--help"},
		{"pull", "--help"},
		{"versions", "--help"},
		{"bootstrap", "--help"},
		{"update", "--help"},
		{"version", "--help"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
			}
			if stdout.Len()+stderr.Len() == 0 {
				t.Fatal("help output is empty")
			}
		})
	}
}

func TestRunWithoutArgsLaunchesTUIOnlyWhenInteractive(t *testing.T) {
	previousInteractive := interactiveUI
	previousLaunch := launchUI
	defer func() {
		interactiveUI = previousInteractive
		launchUI = previousLaunch
	}()

	interactiveUI = func(_ io.Reader, _ io.Writer) bool { return true }
	called := false
	launchUI = func(options tui.Options) error {
		called = true
		if options.Home == "" || options.Input != stdin {
			t.Fatalf("default TUI options = %#v", options)
		}
		return nil
	}

	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !called {
		t.Fatal("interactive no-argument invocation did not launch TUI")
	}
}

func TestRunWithoutArgsDoesNotLaunchTUIInPipes(t *testing.T) {
	previousInteractive := interactiveUI
	previousLaunch := launchUI
	defer func() {
		interactiveUI = previousInteractive
		launchUI = previousLaunch
	}()

	interactiveUI = func(_ io.Reader, _ io.Writer) bool { return false }
	launchUI = func(tui.Options) error {
		t.Fatal("non-interactive no-argument invocation launched TUI")
		return nil
	}

	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunScanWritesJSONOnly(t *testing.T) {
	home, err := filepath.Abs(filepath.Join("..", "..", "testdata", "home-basic"))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"scan", "--home", home, "--output", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report inventory.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not inventory JSON: %v", err)
	}
	if report.SchemaVersion != 1 || report.Kind != "inventory" {
		t.Fatalf("unexpected report header: %#v", report)
	}
	if strings.Contains(stdout.String(), home) {
		t.Fatal("stdout leaked absolute home path")
	}
}

func TestRunSensitiveScanDoesNotLeakValues(t *testing.T) {
	home, err := filepath.Abs(filepath.Join("..", "..", "testdata", "home-sensitive"))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"scan", "--home", home}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	combined := stdout.String() + stderr.String()
	for _, secret := range []string{
		"sk-test-credential-must-never-appear",
		"github_pat_test_auth_must_never_appear",
		"sk-test-inline-secret-must-never-appear",
		"fixture-token-that-must-never-appear",
	} {
		if strings.Contains(combined, secret) {
			t.Fatalf("command output leaked %q", secret)
		}
	}
}

func TestRunInvalidRootDoesNotEchoPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "private-home-name")
	var stdout, stderr bytes.Buffer
	code := run([]string{"scan", "--home", missing}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), missing) || strings.Contains(stderr.String(), "private-home-name") {
		t.Fatalf("stderr leaked root path: %q", stderr.String())
	}
}

func TestRunUIRejectsNonTTYWithJSONAlternative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"ui", "--home", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "interactive TTY") ||
		!strings.Contains(stderr.String(), "scan --output json") {
		t.Fatalf("stderr = %q, want TTY failure with JSON alternative", stderr.String())
	}
}

func TestRunUIDeploymentRejectsNonTTYWithDiffAlternative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"ui", "--home", t.TempDir(), "--package", filepath.Join(t.TempDir(), "fixture.tar"),
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "diff --output json") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunUIAcceptsTargetsForGuidedBuild(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"ui", "--home", t.TempDir(), "--targets", "claude"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "interactive TTY") {
		t.Fatalf("exit code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunUIPreferenceOverridesAreValidatedAndPassedToTUI(t *testing.T) {
	previousLaunch := launchUI
	defer func() { launchUI = previousLaunch }()

	var received tui.Options
	launchUI = func(options tui.Options) error {
		received = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := run(
		[]string{"ui", "--language", "en", "--density", "detailed"},
		&stdout,
		&stderr,
	); code != 0 {
		t.Fatalf("exit code = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if received.Language != "en" || received.Density != "detailed" {
		t.Fatalf(
			"preference overrides = language %q density %q",
			received.Language,
			received.Density,
		)
	}

	called := false
	launchUI = func(tui.Options) error {
		called = true
		return nil
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ui", "--language", "fr"}, &stdout, &stderr); code != 2 {
		t.Fatalf("invalid language exit code = %d, want 2", code)
	}
	if called || !strings.Contains(stderr.String(), "unsupported interface language") {
		t.Fatalf("invalid language called TUI=%v stderr=%q", called, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ui", "--density", "compact"}, &stdout, &stderr); code != 2 {
		t.Fatalf("invalid density exit code = %d, want 2", code)
	}
	if called || !strings.Contains(stderr.String(), "unsupported information density") {
		t.Fatalf("invalid density called TUI=%v stderr=%q", called, stderr.String())
	}
}

func TestRunValidateWritesJSONReport(t *testing.T) {
	manifest, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"validate", "--manifest", manifest, "--output", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout json: %v", err)
	}
	if report["kind"] != "validation" || report["ok"] != true {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunValidateFailsOnSecretWithoutLeak(t *testing.T) {
	root := t.TempDir()
	secret := "sk-cli-validate-must-never-appear"
	mustWriteCLI(t, filepath.Join(root, "assets", "rules", "x.md"), "token: "+secret+"\n")
	mustWriteCLI(t, filepath.Join(root, "manifest.yaml"), `
schemaVersion: 1
name: secret-cli
version: "1"
assets:
  - id: rules.x
    type: rules
    path: assets/rules/x.md
    targets: [claude]
    scope: global
    portability: portable
    sensitivity: private
profiles:
  personal:
    include: [rules.x]
`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"validate", "--manifest", filepath.Join(root, "manifest.yaml")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, secret) {
		t.Fatal("cli validate leaked secret")
	}
	if !strings.Contains(stdout.String(), "suspected_secret") {
		t.Fatalf("stdout missing finding: %s", stdout.String())
	}
}

func TestRunBuildWritesPackageArtifacts(t *testing.T) {
	manifest, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"build",
		"--manifest", manifest,
		"--profile", "personal",
		"--out", out,
		"--output", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json: %v", err)
	}
	if report["ok"] != true || report["kind"] != "build" {
		t.Fatalf("report=%#v", report)
	}
	pkg := report["package"].(map[string]any)
	for _, key := range []string{"archive", "manifest", "lock", "sha256File"} {
		name, _ := pkg[key].(string)
		if name == "" {
			t.Fatalf("missing package.%s", key)
		}
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("artifact %s: %v", name, err)
		}
	}
}

func TestRunApplyAndRollback(t *testing.T) {
	manifest, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	out := t.TempDir()
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"build", "--manifest", manifest, "--profile", "personal", "--out", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("build exit=%d stderr=%q", code, stderr.String())
	}
	var buildReport map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &buildReport); err != nil {
		t.Fatalf("build json: %v", err)
	}
	pkg := buildReport["package"].(map[string]any)
	archive := filepath.Join(out, pkg["archive"].(string))

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"apply", "--package", archive, "--home", home, "--targets", "claude,codex",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "shared-review", "SKILL.md")); err != nil {
		t.Fatalf("installed skill: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"rollback", "--home", home}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rollback exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "shared-review", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("skill should be removed: %v", err)
	}
}

func TestRunDoctorWritesJSONReport(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"doctor", "--home", home, "--output", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var report struct {
		SchemaVersion int    `json:"schemaVersion"`
		Kind          string `json:"kind"`
		ProducedBy    string `json:"producedBy"`
		Ok            bool   `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("doctor json: %v", err)
	}
	if report.SchemaVersion != 1 || report.Kind != "doctor" ||
		report.ProducedBy != version.ProducedBy() || !report.Ok {
		t.Fatalf("doctor report = %#v", report)
	}
}

func TestRunPublishPullRoundTripAcrossMachines(t *testing.T) {
	// Stand in for two machines sharing a directory: a USB stick, a mounted
	// NAS, or a git checkout. aiah never moves the bytes itself (ADR-0007 §1).
	manifest, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dist, channelDir, pulled := t.TempDir(), t.TempDir(), t.TempDir()

	var stdout, stderr bytes.Buffer
	if code := run([]string{"build", "--manifest", manifest, "--profile", "personal",
		"--out", dist, "--output", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("build exit=%d stderr=%q", code, stderr.String())
	}
	archives, err := filepath.Glob(filepath.Join(dist, "*.tar"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("want one archive, got %v (%v)", archives, err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"publish", "--package", archives[0],
		"--channel", channelDir, "--output", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("publish exit=%d stderr=%q", code, stderr.String())
	}
	var published struct {
		Ok         bool   `json:"ok"`
		Name       string `json:"name"`
		Version    string `json:"version"`
		ProducedBy string `json:"producedBy"`
		Unchanged  bool   `json:"unchanged"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &published); err != nil {
		t.Fatalf("publish json: %v", err)
	}
	if !published.Ok || published.Unchanged || published.ProducedBy != version.ProducedBy() {
		t.Fatalf("publish report = %#v", published)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"versions", "--channel", channelDir, "--output", "json"},
		&stdout, &stderr); code != 0 {
		t.Fatalf("versions exit=%d stderr=%q", code, stderr.String())
	}
	var listed struct {
		Releases []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("versions json: %v", err)
	}
	if len(listed.Releases) != 1 || listed.Releases[0].Name != published.Name {
		t.Fatalf("versions = %#v", listed.Releases)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"pull", "--channel", channelDir, "--name", published.Name,
		"--out", pulled, "--output", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("pull exit=%d stderr=%q", code, stderr.String())
	}
	var retrieved struct {
		Ok             bool   `json:"ok"`
		Version        string `json:"version"`
		Package        string `json:"package"`
		ResolvedLatest bool   `json:"resolvedLatest"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &retrieved); err != nil {
		t.Fatalf("pull json: %v", err)
	}
	if !retrieved.Ok || retrieved.Version != published.Version || !retrieved.ResolvedLatest {
		t.Fatalf("pull report = %#v", retrieved)
	}

	// The retrieved package must be installable, which is the whole point.
	home := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"apply", "--package", retrieved.Package,
		"--home", home, "--output", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit=%d stderr=%q", code, stderr.String())
	}
}

func TestRunChannelCommandsRequireTheirFlags(t *testing.T) {
	cases := [][]string{
		{"publish"},
		{"publish", "--package", "x.tar"},
		{"pull", "--channel", "/tmp"},
		{"pull", "--channel", "/tmp", "--name", "n"},
		{"versions"},
		{"versions", "--channel", "/tmp", "extra"},
		{"bootstrap"},
		{"bootstrap", "--channel", "/tmp", "--name", "n", "--out", "/tmp"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%v) exit=%d, want 2", args, code)
		}
	}
}

func TestRunMCPServesReadOnlyToolsOverStdio(t *testing.T) {
	original := stdin
	t.Cleanup(func() { stdin = original })
	stdin = strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"mcp"}, &stdout, &stderr); code != 0 {
		t.Fatalf("mcp exit=%d stderr=%q", code, stderr.String())
	}
	var response struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("mcp response: %v (%q)", err, stdout.String())
	}
	names := make([]string, 0, len(response.Result.Tools))
	for _, tool := range response.Result.Tools {
		names = append(names, tool.Name)
	}
	// The CLI must not widen the surface the server package defines.
	want := "aiah_asset_status,aiah_diff,aiah_doctor,aiah_migration_status," +
		"aiah_scan,aiah_validate,aiah_version"
	if strings.Join(names, ",") != want {
		t.Fatalf("tools = %v, want %s", names, want)
	}
}

func TestRunMCPCallsAssetStatusOverStdio(t *testing.T) {
	original := stdin
	t.Cleanup(func() { stdin = original })
	workspaceRoot, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid"))
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.Abs(filepath.Join("..", "..", "testdata", "home-basic"))
	if err != nil {
		t.Fatal(err)
	}
	stdin = strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"aiah_asset_status","arguments":{"workspace":%q,"home":%q}}}`,
		workspaceRoot, home,
	) + "\n")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"mcp"}, &stdout, &stderr); code != 0 {
		t.Fatalf("mcp exit=%d stderr=%q", code, stderr.String())
	}
	var response struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("mcp response: %v (%q)", err, stdout.String())
	}
	if response.Result.IsError || len(response.Result.Content) != 1 {
		t.Fatalf("asset status response = %#v", response.Result)
	}
	var report struct {
		Kind string `json:"kind"`
		Ok   bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &report); err != nil {
		t.Fatalf("asset status report: %v", err)
	}
	if report.Kind != "asset-catalog" || !report.Ok {
		t.Fatalf("asset status report = %#v", report)
	}
}

func TestRunMCPRejectsArguments(t *testing.T) {
	// No server-level switch may widen what the surface can reach, so the
	// subcommand takes neither flags nor operands. The operand case has to be a
	// bare word: a flag would be rejected by the parser before reaching the
	// operand guard, which would leave that guard untested.
	cases := [][]string{
		{"mcp", "serve"},
		{"mcp", "--home", "/tmp"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%v) exit=%d, want 2", args, code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("run(%v) wrote to stdout: %q", args, stdout.String())
		}
	}
}

func TestRunApplyDryRunDoesNotWrite(t *testing.T) {
	manifest, err := filepath.Abs(filepath.Join("..", "..", "testdata", "workspace-valid", "manifest.yaml"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	out := t.TempDir()
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"build", "--manifest", manifest, "--profile", "personal", "--out", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("build exit=%d stderr=%q", code, stderr.String())
	}
	var buildReport map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &buildReport); err != nil {
		t.Fatalf("build json: %v", err)
	}
	pkg := buildReport["package"].(map[string]any)
	archive := filepath.Join(out, pkg["archive"].(string))

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"apply", "--dry-run", "--package", archive, "--home", home, "--targets", "claude",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("dry-run json: %v", err)
	}
	if report["dryRun"] != true || report["ok"] != true {
		t.Fatalf("dry-run report = %#v", report)
	}
	for _, path := range []string{".claude", ".agents", ".aiah"} {
		if _, err := os.Stat(filepath.Join(home, path)); !os.IsNotExist(err) {
			t.Fatalf("dry-run created %s: %v", path, err)
		}
	}
}

func mustWriteCLI(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestRunVersionReportsBuildIdentity(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "aiah ") {
		t.Fatalf("text output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"version", "--output", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("json exit code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int    `json:"schemaVersion"`
		Kind          string `json:"kind"`
		ProducedBy    string `json:"producedBy"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	// schemaVersion is a number here exactly as in every other report.
	if payload.SchemaVersion != 1 || payload.Kind != "version" ||
		payload.ProducedBy != version.ProducedBy() {
		t.Fatalf("payload = %#v", payload)
	}

	// --version is the flag spelling users reach for first.
	stdout.Reset()
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--version exit code = %d", code)
	}
	if !strings.HasPrefix(stdout.String(), "aiah ") {
		t.Fatalf("--version output = %q", stdout.String())
	}
}

func TestRunUpdateCheckUsesReadOnlyCore(t *testing.T) {
	original := updateCheck
	t.Cleanup(func() { updateCheck = original })
	updateCheck = func(options updater.Options) (updater.Report, error) {
		if options.CurrentVersion != "" {
			t.Fatalf("CLI overrode Core current version: %#v", options)
		}
		return updater.Report{
			SchemaVersion:   1,
			Kind:            "update-check",
			ProducedBy:      version.ProducedBy(),
			Ok:              true,
			CurrentVersion:  "0.1.2",
			LatestVersion:   "0.1.3",
			Status:          updater.StatusUpdateAvailable,
			UpdateAvailable: true,
			UpgradeCommand:  "curl -fsSL https://example.invalid/v0.1.3/install.sh | AIAH_VERSION=0.1.3 sh",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"update", "--check", "--output", "json"},
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	var report updater.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Ok || !report.UpdateAvailable ||
		report.UpgradeCommand !=
			"curl -fsSL https://example.invalid/v0.1.3/install.sh | AIAH_VERSION=0.1.3 sh" {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunUpdateRequiresExplicitCheck(t *testing.T) {
	for _, args := range [][]string{{"update"}, {"--update"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q",
				args, code, stdout.String(), stderr.String())
		}
	}
}

// Every report kind must name the binary that produced it: without it, output
// pasted into an issue cannot be tied to the rules that generated it.
func TestReportsCarryProducedBy(t *testing.T) {
	home := t.TempDir()
	for _, args := range [][]string{
		{"scan", "--home", home, "--output", "json"},
		{"doctor", "--home", home, "--output", "json"},
		{"rollback", "--home", home, "--output", "json"},
	} {
		var stdout, stderr bytes.Buffer
		run(args, &stdout, &stderr)
		var report struct {
			ProducedBy string `json:"producedBy"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("%v: stdout is not JSON: %v", args[0], err)
		}
		if report.ProducedBy != version.ProducedBy() {
			t.Fatalf("%v: producedBy = %q, want %q", args[0], report.ProducedBy, version.ProducedBy())
		}
	}
}
