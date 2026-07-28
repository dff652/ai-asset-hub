package adapter

import "github.com/dff652/ai-asset-hub/internal/build"

type claudeAdapter struct{}

func (claudeAdapter) ID() string { return TargetClaude }

func (claudeAdapter) Compile(manifest build.PackageManifest, files map[string][]byte) ([]StagedFile, CompileReport) {
	return compileToolTree(TargetClaude, ".claude", manifest, files, map[string]bool{
		"skill": true,
		"rules": true,
		"agent": true,
		"hook":  true,
		"mcp":   true,
	})
}
