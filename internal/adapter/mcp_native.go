package adapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

type MCPConfigNoteKind string

const (
	MCPConfigUnchanged MCPConfigNoteKind = "unchanged"
	MCPConfigSkipped   MCPConfigNoteKind = "skipped"
)

type MCPConfigNote struct {
	Target  string
	RelPath string
	Kind    MCPConfigNoteKind
}

// NativeMCPConfigPath returns the harness-native path below one install root.
func NativeMCPConfigPath(target, scope string) (string, bool) {
	if scope == "" {
		scope = "global"
	}
	if scope != "global" && scope != "project" {
		return "", false
	}
	switch target {
	case TargetClaude:
		if scope == "project" {
			return ".mcp.json", true
		}
		return ".claude.json", true
	case TargetCodex:
		return ".codex/config.toml", true
	case TargetGrok:
		return ".grok/config.toml", true
	default:
		return "", false
	}
}

// BuildNativeMCPConfigFiles creates missing native configs as a one-time
// bootstrap. Any existing config is inspected but never rewritten.
func BuildNativeMCPConfigFiles(
	byTarget map[string]map[string]MCPServerEntry,
	existingFiles map[string][]byte,
	scope string,
) ([]StagedFile, []MCPConfigNote, error) {
	if scope == "" {
		scope = "global"
	}
	out := make([]StagedFile, 0)
	notes := make([]MCPConfigNote, 0)
	targets := make([]string, 0, len(byTarget))
	for target := range byTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	for _, target := range targets {
		servers := byTarget[target]
		if len(servers) == 0 {
			continue
		}
		rel, ok := NativeMCPConfigPath(target, scope)
		if !ok {
			return nil, nil, fmt.Errorf("unsupported native MCP target or scope")
		}
		if existing, exists := existingFiles[rel]; exists {
			allPresent, err := inspectNativeMCPConfig(target, existing, servers)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", target, err)
			}
			kind := MCPConfigSkipped
			if allPresent {
				kind = MCPConfigUnchanged
			}
			notes = append(notes, MCPConfigNote{
				Target: target, RelPath: rel, Kind: kind,
			})
			continue
		}

		body, err := renderNativeMCPConfig(target, servers)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", target, err)
		}
		sum := sha256.Sum256(body)
		out = append(out, StagedFile{
			RelPath: rel,
			Body:    body,
			SHA256:  hex.EncodeToString(sum[:]),
			AssetID: "mcp.native",
			Target:  target,
			Source:  "mcp-native",
			Scope:   scope,
			Mode:    ModePrivate,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].RelPath != notes[j].RelPath {
			return notes[i].RelPath < notes[j].RelPath
		}
		return notes[i].Target < notes[j].Target
	})
	return out, notes, nil
}

func inspectNativeMCPConfig(
	target string,
	existing []byte,
	servers map[string]MCPServerEntry,
) (bool, error) {
	root := map[string]any{}
	containerKey := "mcp_servers"
	label := "config.toml"
	if target == TargetClaude {
		containerKey = "mcpServers"
		label = "Claude MCP config"
		if err := json.Unmarshal(existing, &root); err != nil {
			return false, fmt.Errorf("existing Claude MCP config is invalid")
		}
	} else if err := toml.Unmarshal(existing, &root); err != nil {
		return false, fmt.Errorf("existing config.toml is invalid")
	}

	current, exists := root[containerKey]
	if !exists {
		return false, nil
	}
	container, ok := current.(map[string]any)
	if !ok {
		return false, fmt.Errorf("existing %s %s has the wrong type", label, containerKey)
	}
	allPresent := true
	for _, name := range sortedServerNames(servers) {
		desired := serverToMap(servers[name])
		currentServer, exists := container[name]
		if !exists {
			allPresent = false
			continue
		}
		if !jsonEquivalent(currentServer, desired) {
			return false, fmt.Errorf("existing mcp server %q conflicts with package", name)
		}
	}
	return allPresent, nil
}

func renderNativeMCPConfig(
	target string,
	servers map[string]MCPServerEntry,
) ([]byte, error) {
	if target == TargetClaude {
		return marshalJSONStable(map[string]any{"mcpServers": serverMap(servers)})
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(map[string]any{"mcp_servers": serverMap(servers)}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func serverMap(servers map[string]MCPServerEntry) map[string]any {
	out := make(map[string]any, len(servers))
	for _, name := range sortedServerNames(servers) {
		out[name] = serverToMap(servers[name])
	}
	return out
}

func serverToMap(entry MCPServerEntry) map[string]any {
	out := map[string]any{}
	if entry.Command != "" {
		out["command"] = entry.Command
	}
	if len(entry.Args) > 0 {
		out["args"] = entry.Args
	}
	if len(entry.Env) > 0 {
		out["env"] = entry.Env
	}
	if entry.URL != "" {
		out["url"] = entry.URL
	}
	return out
}

func sortedServerNames(servers map[string]MCPServerEntry) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func marshalJSONStable(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func jsonEquivalent(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}
