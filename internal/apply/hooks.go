package apply

import (
	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

// applyHookPolicy validates hook content and assigns executable modes.
// Fail-closed on secrets, binary, empty, or script hooks missing shebang.
func applyHookPolicy(
	staged []adapter.StagedFile,
	_ stageContext,
) ([]adapter.StagedFile, []workspace.Finding) {
	annotated, notes, err := adapter.AnnotateHookPolicies(staged)
	if err != nil {
		return staged, []workspace.Finding{{
			Code:     codeHookInvalid,
			Severity: workspace.SeverityError,
			Message:  "Hook validation failed: " + err.Error(),
			Paths:    []string{"hooks"},
		}}
	}
	findings := make([]workspace.Finding, 0, len(notes))
	for _, note := range notes {
		findings = append(findings, workspace.Finding{
			Code:     codeHookPolicy,
			Severity: workspace.SeverityInfo,
			Message:  note,
			Paths:    []string{"hooks"},
		})
	}
	return annotated, findings
}
