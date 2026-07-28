package adapter

import "github.com/dff652/ai-asset-hub/internal/build"

type grokAdapter struct{}

func (grokAdapter) ID() string { return TargetGrok }

// Compile installs a Grok Build subset: skills, rules, hooks, agents.
// MCP is accepted as files under .grok/mcp with no config.toml merge.
func (grokAdapter) Compile(manifest build.PackageManifest, files map[string][]byte) ([]StagedFile, CompileReport) {
	return compileToolTree(TargetGrok, ".grok", manifest, files, map[string]bool{
		"skill": true,
		"rules": true,
		"agent": true,
		"hook":  true,
		"mcp":   true,
	})
}
