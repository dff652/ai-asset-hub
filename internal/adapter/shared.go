package adapter

import "github.com/dff652/ai-asset-hub/internal/build"

type sharedAdapter struct{}

func (sharedAdapter) ID() string { return TargetShared }

func (sharedAdapter) Compile(manifest build.PackageManifest, files map[string][]byte) ([]StagedFile, CompileReport) {
	return compileToolTree(TargetShared, ".agents", manifest, files, map[string]bool{
		"skill": true,
	})
}
