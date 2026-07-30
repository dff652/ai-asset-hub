package apply

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/dff652/ai-asset-hub/internal/adapter"
)

// SecretReferenceStatus reports whether one portable secret reference can be
// resolved on this device. Resolved values are deliberately discarded.
type SecretReferenceStatus struct {
	Provider  adapter.SecretProvider `json:"provider"`
	Name      string                 `json:"name"`
	Available bool                   `json:"available"`
	Targets   []string               `json:"targets"`
}

// InspectSecretReferences validates staged MCP templates and checks their
// referenced providers without changing the templates or exposing values.
func InspectSecretReferences(staged []adapter.StagedFile) ([]SecretReferenceStatus, error) {
	byTarget, err := adapter.CollectMCPTemplates(staged)
	if err != nil {
		return nil, err
	}

	targetsByReference := make(map[adapter.SecretReference]map[string]bool)
	for _, target := range sortedKeys(byTarget) {
		for _, serverName := range sortedKeys(byTarget[target]) {
			entry := byTarget[target][serverName]
			for _, envKey := range sortedKeys(entry.Env) {
				ref, ok := adapter.ParseSecretReference(entry.Env[envKey])
				if !ok {
					continue
				}
				if targetsByReference[ref] == nil {
					targetsByReference[ref] = make(map[string]bool)
				}
				targetsByReference[ref][target] = true
			}
		}
	}

	references := make([]adapter.SecretReference, 0, len(targetsByReference))
	for ref := range targetsByReference {
		references = append(references, ref)
	}
	sort.Slice(references, func(i, j int) bool {
		if references[i].Provider != references[j].Provider {
			return references[i].Provider < references[j].Provider
		}
		return references[i].Name < references[j].Name
	})

	statuses := make([]SecretReferenceStatus, 0, len(references))
	for _, ref := range references {
		_, resolveErr := resolveSecretReference(ref)
		statuses = append(statuses, SecretReferenceStatus{
			Provider:  ref.Provider,
			Name:      ref.Name,
			Available: resolveErr == nil,
			Targets:   sortedKeys(targetsByReference[ref]),
		})
	}
	return statuses, nil
}

func resolveMCPSecretReferences(byTarget map[string]map[string]adapter.MCPServerEntry) error {
	resolved := make(map[adapter.SecretReference]string)
	for _, target := range sortedKeys(byTarget) {
		servers := byTarget[target]
		for _, serverName := range sortedKeys(servers) {
			entry := servers[serverName]
			for _, envKey := range sortedKeys(entry.Env) {
				ref, ok := adapter.ParseSecretReference(entry.Env[envKey])
				if !ok {
					continue
				}
				value, exists := resolved[ref]
				if !exists {
					var err error
					value, err = resolveSecretReference(ref)
					if err != nil {
						return fmt.Errorf("%s mcp server %q env %q: %w", target, serverName, envKey, err)
					}
					resolved[ref] = value
				}
				entry.Env[envKey] = value
			}
			servers[serverName] = entry
		}
	}
	return nil
}

func resolveSecretReference(ref adapter.SecretReference) (string, error) {
	switch ref.Provider {
	case adapter.SecretProviderEnvironment:
		value, ok := os.LookupEnv(ref.Name)
		if !ok || value == "" {
			return "", fmt.Errorf("environment variable %q is not set or is empty", ref.Name)
		}
		return value, nil
	case adapter.SecretProviderPass:
		output, err := exec.Command("pass", "show", "--", ref.Name).Output()
		if err != nil {
			return "", fmt.Errorf("pass entry %q could not be read", ref.Name)
		}
		if newline := bytes.IndexByte(output, '\n'); newline >= 0 {
			output = output[:newline]
		}
		output = bytes.TrimSuffix(output, []byte{'\r'})
		if len(output) == 0 {
			return "", fmt.Errorf("pass entry %q is empty", ref.Name)
		}
		return string(output), nil
	default:
		return "", fmt.Errorf("secret provider is unsupported")
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
