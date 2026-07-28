package adapter

import "github.com/dff652/ai-asset-hub/internal/build"

type codexAdapter struct{}

func (codexAdapter) ID() string { return TargetCodex }

func (codexAdapter) Compile(manifest build.PackageManifest, files map[string][]byte) ([]StagedFile, CompileReport) {
	return compileToolTree(TargetCodex, ".codex", manifest, files, map[string]bool{
		"skill": true,
		"rules": true,
		"agent": true,
		"hook":  true,
		"mcp":   true,
	})
}
