package inventory

import (
	"encoding/json"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

func inspectConfig(entry *Entry, content []byte) []FindingCode {
	if entry.Type != TypeConfig && entry.Type != TypeMCPConfig {
		return nil
	}
	var value map[string]any
	switch {
	case strings.HasSuffix(strings.ToLower(entry.LogicalPath), ".json"):
		if err := json.Unmarshal(content, &value); err != nil {
			entry.ConfigState = ConfigInvalid
			return []FindingCode{FindingInvalidConfig}
		}
	case strings.HasSuffix(strings.ToLower(entry.LogicalPath), ".toml"):
		if err := toml.Unmarshal(content, &value); err != nil {
			entry.ConfigState = ConfigInvalid
			return []FindingCode{FindingInvalidConfig}
		}
	default:
		return nil
	}
	entry.ConfigState = ConfigValid
	entry.Features = explicitConfigFeatures(*entry, value)
	return nil
}

func explicitConfigFeatures(entry Entry, value map[string]any) []Feature {
	found := make(map[Feature]bool)
	if entry.Type == TypeMCPConfig {
		found[FeatureMCP] = true
	}
	if entry.Source == SourceClaude {
		if _, ok := value["hooks"]; ok {
			found[FeatureHooks] = true
		}
		if _, ok := value["enabledPlugins"]; ok {
			found[FeaturePlugins] = true
		}
		if _, ok := value["mcpServers"]; ok {
			found[FeatureMCP] = true
		}
		if projects, ok := value["projects"].(map[string]any); ok {
			for _, rawProject := range projects {
				project, ok := rawProject.(map[string]any)
				if !ok {
					continue
				}
				if _, ok := project["mcpServers"]; ok {
					found[FeatureMCP] = true
					break
				}
			}
		}
	}
	if entry.Source == SourceCodex || entry.Source == SourceGrok {
		if _, ok := value["mcp_servers"]; ok {
			found[FeatureMCP] = true
		}
	}
	if entry.Source == SourceCodex {
		if hasString(value["project_doc_fallback_filenames"], "CLAUDE.md") {
			found[FeatureCodexProjectDocFallback] = true
		}
	}

	features := make([]Feature, 0, len(found))
	for feature := range found {
		features = append(features, feature)
	}
	sort.Slice(features, func(i, j int) bool { return features[i] < features[j] })
	return features
}

func hasString(value any, wanted string) bool {
	switch values := value.(type) {
	case []any:
		for _, value := range values {
			if text, ok := value.(string); ok && text == wanted {
				return true
			}
		}
	case []string:
		for _, value := range values {
			if value == wanted {
				return true
			}
		}
	}
	return false
}
