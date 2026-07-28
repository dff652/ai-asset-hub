package apply

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/dff652/ai-asset-hub/internal/adapter"
)

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
