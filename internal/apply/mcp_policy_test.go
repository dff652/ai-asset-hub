package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestMCPPolicyCreatesMissingNativeFiles(t *testing.T) {
	t.Setenv("T", "resolved-test-token")
	home := t.TempDir()
	staged := []adapter.StagedFile{
		{
			RelPath: ".claude/mcp/example.json",
			Body:    []byte(`{"name":"example","command":"example-mcp","env":{"T":"${ENV:T}"}}`),
			SHA256:  "a", AssetID: "mcp.example", Target: "claude", Scope: "global", Source: "assets/mcp/example.json",
		},
		{
			RelPath: ".codex/mcp/example.json",
			Body:    []byte(`{"name":"example","command":"example-mcp","env":{"T":"${ENV:T}"}}`),
			SHA256:  "b", AssetID: "mcp.example", Target: "codex", Scope: "global", Source: "assets/mcp/example.json",
		},
		{
			RelPath: ".grok/mcp/example.json",
			Body:    []byte(`{"name":"example","command":"example-mcp","env":{"T":"${ENV:T}"}}`),
			SHA256:  "g", AssetID: "mcp.example", Target: "grok", Scope: "global", Source: "assets/mcp/example.json",
		},
		{
			RelPath: ".claude/skills/x/SKILL.md",
			Body:    []byte("# x\n"),
			SHA256:  "c", AssetID: "skill.x", Target: "claude", Scope: "global", Source: "assets/skills/x/SKILL.md",
		},
	}

	out, findings := applyMCPPolicy(staged, stageContext{home: home})
	if workspace.HasError(findings) {
		t.Fatalf("findings = %#v", findings)
	}
	var settings, codexCfg, grokCfg, skill bool
	for _, file := range out {
		switch file.RelPath {
		case ".claude.json":
			settings = true
			if file.Mode != adapter.ModePrivate {
				t.Fatalf("settings mode = %o, want 600", file.Mode)
			}
			if !strings.Contains(string(file.Body), "example-mcp") {
				t.Fatalf("settings = %s", file.Body)
			}
			if !strings.Contains(string(file.Body), "resolved-test-token") ||
				strings.Contains(string(file.Body), "${ENV:T}") {
				t.Fatalf("settings did not resolve secret ref: %s", file.Body)
			}
		case ".codex/config.toml":
			codexCfg = true
			if file.Mode != adapter.ModePrivate {
				t.Fatalf("codex mode = %o, want 600", file.Mode)
			}
			if !strings.Contains(string(file.Body), "example-mcp") {
				t.Fatalf("codex = %s", file.Body)
			}
		case ".grok/config.toml":
			grokCfg = true
			if file.Mode != adapter.ModePrivate {
				t.Fatalf("grok mode = %o, want 600", file.Mode)
			}
			if !strings.Contains(string(file.Body), "example-mcp") {
				t.Fatalf("grok = %s", file.Body)
			}
		case ".claude/skills/x/SKILL.md":
			skill = true
		case ".claude/mcp/example.json", ".codex/mcp/example.json", ".grok/mcp/example.json":
			// sidecars retained
		}
	}
	if !settings || !codexCfg || !grokCfg || !skill {
		t.Fatalf("missing native outputs settings=%v codex=%v grok=%v skill=%v",
			settings, codexCfg, grokCfg, skill)
	}
	for index, want := range staged {
		if out[index].AssetID != want.AssetID || out[index].RelPath != want.RelPath {
			t.Fatalf("staged order changed at %d: got %#v want %#v", index, out[index], want)
		}
	}
}

func TestMCPPolicyUsesProjectNativePaths(t *testing.T) {
	project := t.TempDir()
	staged := []adapter.StagedFile{
		{RelPath: ".claude/mcp/example.json", Body: []byte(`{"name":"example","command":"example-mcp"}`), Target: "claude", Scope: "project"},
		{RelPath: ".codex/mcp/example.json", Body: []byte(`{"name":"example","command":"example-mcp"}`), Target: "codex", Scope: "project"},
		{RelPath: ".grok/mcp/example.json", Body: []byte(`{"name":"example","command":"example-mcp"}`), Target: "grok", Scope: "project"},
	}
	out, findings := applyMCPPolicy(staged, stageContext{project: project})
	if workspace.HasError(findings) {
		t.Fatalf("findings = %#v", findings)
	}
	paths := map[string]bool{}
	for _, file := range out {
		if file.Source == "mcp-native" {
			paths[file.RelPath] = true
		}
	}
	for _, want := range []string{".mcp.json", ".codex/config.toml", ".grok/config.toml"} {
		if !paths[want] {
			t.Fatalf("missing project native path %q in %#v", want, paths)
		}
	}
}

func TestMCPPolicyLeavesExistingNativeConfigUnchanged(t *testing.T) {
	home := t.TempDir()
	nativePath := filepath.Join(home, ".codex", "config.toml")
	original := []byte("# preserve this comment\napi_token = \"existing-user-secret\"\n")
	mustWrite(t, nativePath, original)
	staged := []adapter.StagedFile{{
		RelPath: ".codex/mcp/example.json",
		Body:    []byte(`{"name":"example","command":"example-mcp"}`),
		Target:  "codex", Scope: "global",
	}}

	out, findings := applyMCPPolicy(staged, stageContext{home: home})
	if workspace.HasError(findings) {
		t.Fatalf("findings = %#v", findings)
	}
	for _, file := range out {
		if file.RelPath == ".codex/config.toml" {
			t.Fatalf("existing native config was staged: %#v", file)
		}
	}
	if !hasFindingCode(findings, codeMCPNativeSkipped) {
		t.Fatalf("missing native skip finding: %#v", findings)
	}
	after, err := os.ReadFile(nativePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("native config changed:\n%s", after)
	}
}

func TestMCPPolicyIdenticalExistingIsNotStaged(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".claude.json"),
		[]byte("{\n \"mcpServers\":{\"example\":{\"command\":\"example-mcp\"}},\n \"theme\":\"dark\"\n}\n"))
	staged := []adapter.StagedFile{{
		RelPath: ".claude/mcp/example.json",
		Body:    []byte(`{"name":"example","command":"example-mcp"}`),
		Target:  "claude", Scope: "global",
	}}
	out, findings := applyMCPPolicy(staged, stageContext{home: home})
	if workspace.HasError(findings) {
		t.Fatalf("findings = %#v", findings)
	}
	for _, file := range out {
		if file.RelPath == ".claude.json" {
			t.Fatalf("identical native config was staged: %#v", file)
		}
	}
}

func TestMCPPolicyRejectsRawSecret(t *testing.T) {
	home := t.TempDir()
	staged := []adapter.StagedFile{{
		RelPath: ".claude/mcp/bad.json",
		Body:    []byte(`{"name":"bad","command":"c","env":{"K":"sk-test-raw-secret-must-fail"}}`),
		SHA256:  "x", AssetID: "mcp.bad", Target: "claude", Scope: "global",
	}}
	_, findings := applyMCPPolicy(staged, stageContext{home: home})
	if !workspace.HasError(findings) {
		t.Fatal("expected secret rejection")
	}
	found := false
	for _, f := range findings {
		if f.Code == codeMCPNativeFailed {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mcp_native_failed finding, got %#v", findings)
	}
}

func TestMCPPolicyDefersMissingRootToPlan(t *testing.T) {
	staged := []adapter.StagedFile{{
		RelPath: ".claude/mcp/example.json",
		Body:    []byte(`{"name":"example","command":"example-mcp"}`),
		AssetID: "mcp.example", Target: "claude", Scope: "global",
	}}
	out, findings := applyMCPPolicy(staged, stageContext{})
	if workspace.HasError(findings) || len(findings) != 0 {
		t.Fatalf("policy claimed missing-root ownership: %#v", findings)
	}
	if len(out) != 1 || out[0].RelPath != staged[0].RelPath {
		t.Fatalf("staged files changed without an install root: %#v", out)
	}
}

func TestMCPPolicyCreatedNativeIsOneTimeBootstrap(t *testing.T) {
	home := t.TempDir()
	firstSidecar := adapter.StagedFile{
		RelPath: ".claude/mcp/example.json",
		Body:    []byte(`{"name":"example","command":"example-mcp"}`),
		AssetID: "mcp.example", Target: "claude", Scope: "global",
	}
	first, findings := applyMCPPolicy([]adapter.StagedFile{firstSidecar}, stageContext{home: home})
	if workspace.HasError(findings) {
		t.Fatalf("first policy: %#v", findings)
	}
	var created []byte
	for _, file := range first {
		if file.RelPath == ".claude.json" {
			created = append([]byte(nil), file.Body...)
		}
	}
	if len(created) == 0 {
		t.Fatal("missing one-time native bootstrap output")
	}
	nativePath := filepath.Join(home, ".claude.json")
	mustWrite(t, nativePath, created)

	secondSidecar := adapter.StagedFile{
		RelPath: ".claude/mcp/extra.json",
		Body:    []byte(`{"name":"extra","command":"extra-mcp"}`),
		AssetID: "mcp.extra", Target: "claude", Scope: "global",
	}
	second, findings := applyMCPPolicy(
		[]adapter.StagedFile{firstSidecar, secondSidecar},
		stageContext{home: home},
	)
	if workspace.HasError(findings) || !hasFindingCode(findings, codeMCPNativeSkipped) {
		t.Fatalf("second policy: %#v", findings)
	}
	if len(second) != 2 {
		t.Fatalf("existing native config was staged again: %#v", second)
	}
	after, err := os.ReadFile(nativePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(created) {
		t.Fatal("one-time native bootstrap file changed")
	}
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
