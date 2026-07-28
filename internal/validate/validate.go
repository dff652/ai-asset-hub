package validate

import (
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

// Options configures a read-only workspace validation.
type Options struct {
	// Manifest is the path to manifest.yaml or manifest.json.
	Manifest string
	// Root is the workspace root used to resolve asset paths.
	// Empty means the directory containing the manifest.
	Root string
}

// Validate checks a manifest and the asset files it references. It never writes
// to the workspace and never follows escaping symlinks for content reads.
func Validate(options Options) (Report, error) {
	inspection, err := workspace.Inspect(workspace.Options{
		Manifest:      options.Manifest,
		Root:          options.Root,
		CollectBodies: false,
	})
	if err != nil {
		return Report{}, err
	}

	errors, warnings := 0, 0
	for _, finding := range inspection.Findings {
		switch finding.Severity {
		case workspace.SeverityError:
			errors++
		case workspace.SeverityWarning:
			warnings++
		}
	}

	return Report{
		SchemaVersion: 1,
		Kind:          "validation",
		ProducedBy:    version.ProducedBy(),
		Ok:            errors == 0,
		Summary: Summary{
			AssetCount:   len(inspection.Document.Assets),
			ProfileCount: len(inspection.Document.Profiles),
			CheckedPaths: inspection.Checked,
			ErrorCount:   errors,
			WarningCount: warnings,
		},
		Findings: inspection.Findings,
	}, nil
}
