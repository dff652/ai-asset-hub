package adapter

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseMCPTemplateAllowsSecretRefs(t *testing.T) {
	body := []byte(`{
  "name": "safe",
  "command": "server",
  "env": {"API_KEY": "${ENV:SAFE_API_KEY}"}
}`)
	tmpl, err := ParseMCPTemplate(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tmpl.Name != "safe" || tmpl.Env["API_KEY"] != "${ENV:SAFE_API_KEY}" {
		t.Fatalf("tmpl = %#v", tmpl)
	}
}

func TestParseMCPTemplateRejectsRawSecret(t *testing.T) {
	body := []byte(`{
  "name": "bad",
  "command": "server",
  "env": {"API_KEY": "sk-test-raw-secret-must-fail-validation"}
}`)
	if _, err := ParseMCPTemplate(body); err == nil {
		t.Fatal("expected raw secret rejection")
	}
}

func TestParseMCPTemplateRequiresReferenceForSensitiveEnvKey(t *testing.T) {
	for _, value := range []string{"literal-value-not-shaped-like-a-token", "${ENV:}"} {
		body := []byte(`{
  "name": "bad",
  "command": "server",
  "env": {"API_KEY": "` + value + `"}
}`)
		if _, err := ParseMCPTemplate(body); err == nil {
			t.Fatalf("expected sensitive literal/reference rejection for %q", value)
		}
	}
}

func TestParseSecretReferenceProviders(t *testing.T) {
	tests := []struct {
		value    string
		provider SecretProvider
		name     string
	}{
		{"${ENV:OPENAI_API_KEY}", SecretProviderEnvironment, "OPENAI_API_KEY"},
		{"${env:OPENAI_API_KEY}", SecretProviderEnvironment, "OPENAI_API_KEY"},
		{"${secret:personal/openai}", SecretProviderPass, "personal/openai"},
	}
	for _, test := range tests {
		ref, ok := ParseSecretReference(test.value)
		if !ok || ref.Provider != test.provider || ref.Name != test.name {
			t.Fatalf("ParseSecretReference(%q) = %#v, %v", test.value, ref, ok)
		}
	}
	for _, value := range []string{
		"${ENV:}", "${secret:}", "${secret:two words}", "prefix-${ENV:TOKEN}", "${future:value}",
	} {
		if ref, ok := ParseSecretReference(value); ok {
			t.Fatalf("ParseSecretReference(%q) = %#v, true", value, ref)
		}
	}
}

func TestNativeMCPConfigPath(t *testing.T) {
	tests := []struct {
		target string
		scope  string
		want   string
	}{
		{TargetClaude, "global", ".claude.json"},
		{TargetClaude, "project", ".mcp.json"},
		{TargetCodex, "global", ".codex/config.toml"},
		{TargetCodex, "project", ".codex/config.toml"},
		{TargetGrok, "global", ".grok/config.toml"},
		{TargetGrok, "project", ".grok/config.toml"},
	}
	for _, test := range tests {
		got, ok := NativeMCPConfigPath(test.target, test.scope)
		if !ok || got != test.want {
			t.Fatalf("path(%s, %s) = %q, %v; want %q, true",
				test.target, test.scope, got, ok, test.want)
		}
	}
	if _, ok := NativeMCPConfigPath("future", "global"); ok {
		t.Fatal("unsupported target returned a native path")
	}
	if _, ok := NativeMCPConfigPath(TargetClaude, "device"); ok {
		t.Fatal("unsupported scope returned a native path")
	}
}

func TestBuildNativeMCPConfigFilesCreatesMissingByScope(t *testing.T) {
	byTarget := map[string]map[string]MCPServerEntry{
		TargetClaude: {"example": {Command: "example-mcp"}},
		TargetCodex:  {"example": {Command: "example-mcp"}},
		TargetGrok:   {"example": {Command: "example-mcp"}},
	}
	for _, test := range []struct {
		scope string
		want  []string
	}{
		{"global", []string{".claude.json", ".codex/config.toml", ".grok/config.toml"}},
		{"project", []string{".codex/config.toml", ".grok/config.toml", ".mcp.json"}},
	} {
		files, notes, err := BuildNativeMCPConfigFiles(byTarget, nil, test.scope)
		if err != nil {
			t.Fatalf("%s: %v", test.scope, err)
		}
		if len(notes) != 0 || len(files) != len(test.want) {
			t.Fatalf("%s: files=%#v notes=%#v", test.scope, files, notes)
		}
		for index, file := range files {
			if file.RelPath != test.want[index] {
				t.Fatalf("%s file[%d] = %q, want %q", test.scope, index, file.RelPath, test.want[index])
			}
			if file.Mode != ModePrivate || file.Scope != test.scope {
				t.Fatalf("%s file = %#v", test.scope, file)
			}
			if !strings.Contains(string(file.Body), "example-mcp") {
				t.Fatalf("%s missing server: %s", file.RelPath, file.Body)
			}
		}
	}
}

func TestBuildNativeMCPConfigFilesIsDeterministic(t *testing.T) {
	byTarget := map[string]map[string]MCPServerEntry{
		TargetCodex: {
			"zeta":  {Command: "z", Env: map[string]string{"B": "2", "A": "1"}},
			"alpha": {Command: "a", Args: []string{"2", "1"}},
		},
	}
	first, _, err := BuildNativeMCPConfigFiles(byTarget, nil, "global")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := BuildNativeMCPConfigFiles(byTarget, nil, "global")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Body, second[0].Body) {
		t.Fatalf("native config is not deterministic:\n%s\n---\n%s", first[0].Body, second[0].Body)
	}
}

func TestBuildNativeMCPConfigFilesIdenticalExistingIsNotStaged(t *testing.T) {
	existing := []byte("{\n  \"theme\": \"dark\",\n  \"mcpServers\": {\"example\":{\"command\":\"same\",\"args\":[\"x\"]}}\n}\n")
	byTarget := map[string]map[string]MCPServerEntry{
		TargetClaude: {"example": {Command: "same", Args: []string{"x"}}},
	}
	files, notes, err := BuildNativeMCPConfigFiles(
		byTarget,
		map[string][]byte{".claude.json": existing},
		"global",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || len(notes) != 1 || notes[0].Kind != MCPConfigUnchanged {
		t.Fatalf("files=%#v notes=%#v", files, notes)
	}
}

func TestBuildNativeMCPConfigFilesIdenticalTOMLIsNotStaged(t *testing.T) {
	existing := []byte("# keep comment\nmodel = \"gpt\"\n\n[mcp_servers.example]\ncommand = \"same\"\nargs = [\"x\"]\n")
	byTarget := map[string]map[string]MCPServerEntry{
		TargetCodex: {"example": {Command: "same", Args: []string{"x"}}},
	}
	files, notes, err := BuildNativeMCPConfigFiles(
		byTarget,
		map[string][]byte{".codex/config.toml": existing},
		"global",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || len(notes) != 1 || notes[0].Kind != MCPConfigUnchanged {
		t.Fatalf("files=%#v notes=%#v", files, notes)
	}
}

func TestBuildNativeMCPConfigFilesLeavesExistingWhenServerMissing(t *testing.T) {
	existing := []byte("# user comment\nmodel = \"gpt\"\n\n[mcp_servers.keep]\ncommand = \"keep\"\n")
	byTarget := map[string]map[string]MCPServerEntry{
		TargetCodex: {"example": {Command: "example-mcp"}},
	}
	files, notes, err := BuildNativeMCPConfigFiles(
		byTarget,
		map[string][]byte{".codex/config.toml": existing},
		"global",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || len(notes) != 1 || notes[0].Kind != MCPConfigSkipped {
		t.Fatalf("files=%#v notes=%#v", files, notes)
	}
}

func TestBuildNativeMCPConfigFilesRejectsConflictAndInvalidContainer(t *testing.T) {
	byTarget := map[string]map[string]MCPServerEntry{
		TargetCodex: {"example": {Command: "new-command"}},
	}
	for _, existing := range [][]byte{
		[]byte("[mcp_servers.example]\ncommand = \"old-command\"\n"),
		[]byte("mcp_servers = \"invalid\"\n"),
	} {
		if _, _, err := BuildNativeMCPConfigFiles(
			byTarget,
			map[string][]byte{".codex/config.toml": existing},
			"global",
		); err == nil {
			t.Fatalf("expected native policy rejection for %s", existing)
		}
	}
}
