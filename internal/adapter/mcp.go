package adapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/content"
)

// MCPTemplate is the portable package format under assets/mcp/*.json.
// Real secret values are rejected; ${ENV:...} / ${secret:...} refs are allowed.
type MCPTemplate struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"` // optional remote transport
}

type SecretProvider string

const (
	SecretProviderEnvironment SecretProvider = "environment"
	SecretProviderPass        SecretProvider = "pass"
)

type SecretReference struct {
	Provider SecretProvider
	Name     string
}

// ParseMCPTemplate decodes and validates a package MCP template.
func ParseMCPTemplate(body []byte) (MCPTemplate, error) {
	var tmpl MCPTemplate
	if err := json.Unmarshal(body, &tmpl); err != nil {
		return MCPTemplate{}, fmt.Errorf("invalid mcp template json")
	}
	tmpl.Name = strings.TrimSpace(tmpl.Name)
	tmpl.Command = strings.TrimSpace(tmpl.Command)
	tmpl.URL = strings.TrimSpace(tmpl.URL)
	if tmpl.Name == "" {
		return MCPTemplate{}, fmt.Errorf("mcp template missing name")
	}
	if tmpl.Command == "" && tmpl.URL == "" {
		return MCPTemplate{}, fmt.Errorf("mcp template missing command or url")
	}
	if looksLikeRawSecret(tmpl.Command) || looksLikeRawSecret(tmpl.URL) {
		return MCPTemplate{}, fmt.Errorf("mcp template command/url looks like a raw secret")
	}
	for key, value := range tmpl.Env {
		if strings.TrimSpace(key) == "" {
			return MCPTemplate{}, fmt.Errorf("mcp template has empty env key")
		}
		if isSensitiveEnvKey(key) && !isSecretRef(value) {
			return MCPTemplate{}, fmt.Errorf("mcp template env %q requires a secret reference", key)
		}
		if looksLikeRawSecret(value) {
			return MCPTemplate{}, fmt.Errorf("mcp template env %q looks like a raw secret", key)
		}
	}
	return tmpl, nil
}

func isSecretRef(value string) bool {
	_, ok := ParseSecretReference(value)
	return ok
}

// ParseSecretReference recognizes whole-value references supported by apply.
// ${secret:...} uses the pass password-store provider.
func ParseSecretReference(value string) (SecretReference, bool) {
	value = strings.TrimSpace(value)
	for _, candidate := range []struct {
		prefix   string
		provider SecretProvider
	}{
		{prefix: "${ENV:", provider: SecretProviderEnvironment},
		{prefix: "${env:", provider: SecretProviderEnvironment},
		{prefix: "${secret:", provider: SecretProviderPass},
	} {
		prefix := candidate.prefix
		if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
		if name == "" || strings.TrimSpace(name) != name ||
			strings.ContainsAny(name, "{} \t\r\n") {
			return SecretReference{}, false
		}
		return SecretReference{Provider: candidate.provider, Name: name}, true
	}
	return SecretReference{}, false
}

func isSensitiveEnvKey(key string) bool {
	normalized := strings.ToUpper(key)
	parts := strings.FieldsFunc(normalized, func(char rune) bool {
		return (char < 'A' || char > 'Z') && (char < '0' || char > '9')
	})
	for _, part := range parts {
		switch part {
		case "TOKEN", "SECRET", "PASSWORD", "PASSWD", "AUTHORIZATION", "BEARER":
			return true
		}
	}
	for _, marker := range []string{"API_KEY", "ACCESS_KEY", "PRIVATE_KEY"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func looksLikeRawSecret(value string) bool {
	if isSecretRef(value) {
		return false
	}
	return content.ContainsSecret([]byte(value))
}

// MCPServerEntry is the tool-agnostic server definition used by native config policy.
type MCPServerEntry struct {
	Command string
	Args    []string
	Env     map[string]string
	URL     string
}

func (t MCPTemplate) entry() MCPServerEntry {
	env := map[string]string{}
	for k, v := range t.Env {
		env[k] = v
	}
	args := append([]string{}, t.Args...)
	return MCPServerEntry{Command: t.Command, Args: args, Env: env, URL: t.URL}
}

// CollectMCPTemplates extracts validated templates from staged mcp/*.json files.
func CollectMCPTemplates(staged []StagedFile) (map[string]map[string]MCPServerEntry, error) {
	// target -> server name -> entry
	byTarget := map[string]map[string]MCPServerEntry{}

	for _, file := range staged {
		if !IsMCPTemplatePath(file.RelPath) {
			continue
		}
		tmpl, err := ParseMCPTemplate(file.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.RelPath, err)
		}
		if byTarget[file.Target] == nil {
			byTarget[file.Target] = map[string]MCPServerEntry{}
		}
		if prev, ok := byTarget[file.Target][tmpl.Name]; ok {
			if !mcpEntriesEqual(prev, tmpl.entry()) {
				return nil, fmt.Errorf("mcp server %q defined twice with different content for %s", tmpl.Name, file.Target)
			}
		}
		byTarget[file.Target][tmpl.Name] = tmpl.entry()
	}
	return byTarget, nil
}

// IsMCPTemplatePath reports whether rel is a harness MCP sidecar destination.
func IsMCPTemplatePath(rel string) bool {
	_, section, ok := toolInstallPath(rel)
	return ok && section == "mcp" && strings.HasSuffix(rel, ".json")
}

func mcpEntriesEqual(a, b MCPServerEntry) bool {
	if a.Command != b.Command || a.URL != b.URL || len(a.Args) != len(b.Args) || len(a.Env) != len(b.Env) {
		return false
	}
	for i := range a.Args {
		if a.Args[i] != b.Args[i] {
			return false
		}
	}
	for k, v := range a.Env {
		if b.Env[k] != v {
			return false
		}
	}
	return true
}
