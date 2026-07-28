package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestMCPPolicyResolvesEnvironmentAndPassRefsOnlyInNativeConfig(t *testing.T) {
	t.Setenv("AIAH_SECRET_TEST_TOKEN", "resolved-from-environment")

	binDir := t.TempDir()
	passArgs := filepath.Join(t.TempDir(), "pass-args")
	passPath := filepath.Join(binDir, "pass")
	passScript := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"$AIAH_PASS_ARGS\"\n" +
		"printf '%s\\n' 'resolved-from-pass' 'metadata-not-part-of-secret'\n"
	if err := os.WriteFile(passPath, []byte(passScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("AIAH_PASS_ARGS", passArgs)

	sidecar := adapter.StagedFile{
		RelPath: ".claude/mcp/example.json",
		Body: []byte(`{
  "name":"example",
  "command":"example-mcp",
  "env":{
    "ENV_TOKEN":"${ENV:AIAH_SECRET_TEST_TOKEN}",
    "ENV_ALIAS":"${env:AIAH_SECRET_TEST_TOKEN}",
    "PASS_TOKEN":"${secret:personal/example}",
    "PLAIN":"not-sensitive"
  }
}`),
		SHA256: "sidecar-sha", AssetID: "mcp.example", Target: "claude", Scope: "global",
	}

	out, findings := applyMCPPolicy([]adapter.StagedFile{sidecar}, stageContext{home: t.TempDir()})
	if workspace.HasError(findings) {
		t.Fatalf("findings = %#v", findings)
	}

	var native []byte
	for _, file := range out {
		switch file.RelPath {
		case sidecar.RelPath:
			if string(file.Body) != string(sidecar.Body) {
				t.Fatalf("sidecar reference was changed:\n%s", file.Body)
			}
		case ".claude.json":
			native = file.Body
		}
	}
	if len(native) == 0 {
		t.Fatal("missing native MCP config")
	}
	for _, want := range []string{
		"resolved-from-environment",
		"resolved-from-pass",
		"not-sensitive",
	} {
		if !strings.Contains(string(native), want) {
			t.Fatalf("native config missing %q:\n%s", want, native)
		}
	}
	if strings.Contains(string(native), "metadata-not-part-of-secret") ||
		strings.Contains(string(native), "${ENV:") ||
		strings.Contains(string(native), "${env:") ||
		strings.Contains(string(native), "${secret:") {
		t.Fatalf("native config contains an unresolved or multiline provider value:\n%s", native)
	}
	args, err := os.ReadFile(passArgs)
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != "show\n--\npersonal/example\n" {
		t.Fatalf("pass args = %q", args)
	}
}

func TestMCPPolicyFailsClosedWhenSecretRefCannotResolve(t *testing.T) {
	const missingEnv = "AIAH_TEST_ENV_THAT_DOES_NOT_EXIST"
	previous, existed := os.LookupEnv(missingEnv)
	if err := os.Unsetenv(missingEnv); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(missingEnv, previous)
		} else {
			_ = os.Unsetenv(missingEnv)
		}
	})

	tests := []struct {
		name  string
		value string
		setup func(*testing.T)
	}{
		{name: "environment", value: "${ENV:" + missingEnv + "}"},
		{
			name:  "pass",
			value: "${secret:personal/missing}",
			setup: func(t *testing.T) { t.Setenv("PATH", t.TempDir()) },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setup != nil {
				test.setup(t)
			}
			staged := []adapter.StagedFile{{
				RelPath: ".claude/mcp/example.json",
				Body: []byte(`{"name":"example","command":"example-mcp","env":{"TOKEN":"` +
					test.value + `"}}`),
				Target: "claude", Scope: "global",
			}}
			native, findings := applyScopeMCPPolicy(staged, t.TempDir(), "global")
			if len(native) != 0 || !workspace.HasError(findings) ||
				!hasFindingCode(findings, codeMCPNativeFailed) {
				t.Fatalf("native=%#v findings=%#v", native, findings)
			}
		})
	}
}

func TestMCPPolicyDoesNotExposePassOutputOnFailure(t *testing.T) {
	const providerOutput = "provider-output-must-not-enter-finding"
	binDir := t.TempDir()
	passPath := filepath.Join(binDir, "pass")
	passScript := "#!/bin/sh\nprintf '%s\\n' '" + providerOutput + "' >&2\nexit 1\n"
	if err := os.WriteFile(passPath, []byte(passScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	staged := []adapter.StagedFile{{
		RelPath: ".claude/mcp/example.json",
		Body: []byte(
			`{"name":"example","command":"example-mcp","env":{"TOKEN":"${secret:personal/failing}"}}`,
		),
		Target: "claude", Scope: "global",
	}}
	_, findings := applyScopeMCPPolicy(staged, t.TempDir(), "global")
	body, err := json.Marshal(findings)
	if err != nil {
		t.Fatal(err)
	}
	if !workspace.HasError(findings) || strings.Contains(string(body), providerOutput) {
		t.Fatalf("findings exposed provider output: %s", body)
	}
}

func TestApplyFailsBeforeWritingWhenSecretRefCannotResolve(t *testing.T) {
	t.Setenv("EXAMPLE_SERVICE_TOKEN", "")
	pkg := buildSecretFixturePackage(t)
	home := t.TempDir()
	project := t.TempDir()

	report, err := Apply(Options{
		Package: pkg,
		Home:    home,
		Project: project,
		Targets: []string{"claude"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Ok || !hasFindingCode(report.Findings, codeMCPNativeFailed) {
		t.Fatalf("report = %#v", report)
	}
	for _, root := range []string{home, project} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("secret resolution failure wrote under %s: %#v", root, entries)
		}
	}
}

func TestResolvedSecretStaysOutOfReportAndMetadata(t *testing.T) {
	const secretValue = "resolved-secret-must-stay-out-of-metadata"
	t.Setenv("EXAMPLE_SERVICE_TOKEN", secretValue)
	pkg := buildSecretFixturePackage(t)
	home := t.TempDir()
	project := t.TempDir()

	report, err := Apply(Options{
		Package: pkg,
		Home:    home,
		Project: project,
		Targets: []string{"claude"},
	})
	if err != nil || !report.Ok {
		t.Fatalf("apply: err=%v report=%#v", err, report)
	}
	reportBody, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(reportBody), secretValue) {
		t.Fatalf("apply report leaked resolved secret: %s", reportBody)
	}
	assertTreeExcludesValue(t, filepath.Join(home, ".aiah"), secretValue)

	native, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(native), secretValue) {
		t.Fatalf("native config did not receive resolved secret:\n%s", native)
	}
	sidecar, err := os.ReadFile(filepath.Join(home, ".claude", "mcp", "example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sidecar), secretValue) ||
		!strings.Contains(string(sidecar), "${ENV:EXAMPLE_SERVICE_TOKEN}") {
		t.Fatalf("sidecar did not preserve the reference:\n%s", sidecar)
	}
}

func TestResolvedSecretStaysOutOfBackupAndJournalMetadata(t *testing.T) {
	const secretValue = "resolved-secret-must-not-enter-transaction-metadata"
	home := t.TempDir()
	item := plannedFile{
		file: adapter.StagedFile{
			RelPath: ".claude.json",
			Body:    []byte(secretValue),
			Target:  "claude",
			Scope:   "global",
		},
		root:    home,
		logical: "home/.claude.json",
		abs:     filepath.Join(home, ".claude.json"),
		action:  actionCreate,
	}
	backupID, _, _, err := prepareBackup(home, "fixture", "1", []plannedFile{item})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJournal(home, backupID, []plannedFile{item}); err != nil {
		t.Fatal(err)
	}
	assertTreeExcludesValue(t, filepath.Join(home, ".aiah"), secretValue)
}

func buildSecretFixturePackage(t *testing.T) string {
	t.Helper()
	workspace := filepath.Join("..", "..", "testdata", "workspace-2b")
	out := t.TempDir()
	report, err := build.Build(build.Options{
		Manifest: filepath.Join(workspace, "manifest.yaml"),
		Profile:  "personal",
		OutDir:   out,
	})
	if err != nil || !report.Ok {
		t.Fatalf("build: err=%v report=%#v", err, report)
	}
	return filepath.Join(out, report.Package.Archive)
}

func assertTreeExcludesValue(t *testing.T, root, value string) {
	t.Helper()
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), value) {
			t.Errorf("%s contains resolved secret", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
