package apply

import (
	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type stageContext struct {
	home    string
	project string
}

type stagePolicy func(
	[]adapter.StagedFile,
	stageContext,
) ([]adapter.StagedFile, []workspace.Finding)

func applyStagePolicies(
	staged []adapter.StagedFile,
	context stageContext,
) ([]adapter.StagedFile, []workspace.Finding) {
	findings := make([]workspace.Finding, 0)
	policies := []stagePolicy{
		applyMCPPolicy,
		applyHookPolicy,
	}
	for _, policy := range policies {
		var next []workspace.Finding
		staged, next = policy(staged, context)
		findings = append(findings, next...)
		if workspace.HasError(findings) {
			return staged, findings
		}
	}
	return staged, findings
}
