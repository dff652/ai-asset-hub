package apply

import (
	"os"
	"sort"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

// applyMCPPolicy keeps mcp/*.json sidecars in their original order and creates
// missing native configs once. Existing native configs are never rewritten.
func applyMCPPolicy(
	staged []adapter.StagedFile,
	context stageContext,
) ([]adapter.StagedFile, []workspace.Finding) {
	findings := make([]workspace.Finding, 0)
	globalTemplates := make([]adapter.StagedFile, 0)
	projectTemplates := make([]adapter.StagedFile, 0)
	for _, file := range staged {
		if !adapter.IsMCPTemplatePath(file.RelPath) {
			continue
		}
		switch file.Scope {
		case "project":
			projectTemplates = append(projectTemplates, file)
		case "", "global":
			globalTemplates = append(globalTemplates, file)
		}
	}

	out := make([]adapter.StagedFile, 0, len(staged)+3)
	out = append(out, staged...)

	nativeGlobal, f := applyScopeMCPPolicy(globalTemplates, context.home, "global")
	findings = append(findings, f...)
	out = append(out, nativeGlobal...)

	nativeProject, f := applyScopeMCPPolicy(projectTemplates, context.project, "project")
	findings = append(findings, f...)
	out = append(out, nativeProject...)

	return out, findings
}

func applyScopeMCPPolicy(staged []adapter.StagedFile, root, scope string) ([]adapter.StagedFile, []workspace.Finding) {
	if len(staged) == 0 {
		return nil, nil
	}
	byTarget, err := adapter.CollectMCPTemplates(staged)
	if err != nil {
		return nil, []workspace.Finding{{
			Code:     codeMCPNativeFailed,
			Severity: workspace.SeverityError,
			Message:  "MCP template validation or native config preparation failed: " + err.Error(),
			Paths:    []string{"mcp"},
		}}
	}
	if len(byTarget) == 0 {
		return nil, nil
	}
	if root == "" {
		// planInstall owns missing-root findings for all staged assets.
		return nil, nil
	}
	if err := resolveMCPSecretReferences(byTarget); err != nil {
		return nil, []workspace.Finding{{
			Code:     codeMCPNativeFailed,
			Severity: workspace.SeverityError,
			Message:  "MCP secret reference could not be resolved: " + err.Error(),
			Paths:    nativeLogicalPaths(byTarget, scope),
		}}
	}
	existing, err := readExistingMCPConfigs(root, byTarget, scope)
	if err != nil {
		return nil, []workspace.Finding{{
			Code:     codeMCPNativeFailed,
			Severity: workspace.SeverityError,
			Message:  "Existing MCP native config is unreadable or unsafe.",
			Paths:    nativeLogicalPaths(byTarget, scope),
		}}
	}

	native, notes, err := adapter.BuildNativeMCPConfigFiles(byTarget, existing, scope)
	if err != nil {
		return nil, []workspace.Finding{{
			Code:     codeMCPNativeFailed,
			Severity: workspace.SeverityError,
			Message:  "MCP native config policy failed: " + err.Error(),
			Paths:    nativeLogicalPaths(byTarget, scope),
		}}
	}
	findings := make([]workspace.Finding, 0, len(native)+len(notes))
	for _, file := range native {
		findings = append(findings, workspace.Finding{
			Code:     codeMCPNativeCreated,
			Severity: workspace.SeverityInfo,
			Message:  file.Target + ": will create missing native MCP config.",
			Paths:    []string{nativeLogicalPath(scope, file.RelPath)},
		})
	}
	for _, note := range notes {
		severity := workspace.SeverityInfo
		code := codeMCPNativeUnchanged
		message := note.Target + ": native MCP config already contains the package servers; no native write."
		if note.Kind == adapter.MCPConfigSkipped {
			severity = workspace.SeverityWarning
			code = codeMCPNativeSkipped
			message = note.Target + ": existing native MCP config left unchanged; install uses the sidecar only."
		}
		findings = append(findings, workspace.Finding{
			Code:     code,
			Severity: severity,
			Message:  message,
			Paths:    []string{nativeLogicalPath(scope, note.RelPath)},
		})
	}
	return native, findings
}

func readExistingMCPConfigs(
	root string,
	byTarget map[string]map[string]adapter.MCPServerEntry,
	scope string,
) (map[string][]byte, error) {
	existing := make(map[string][]byte)
	for target := range byTarget {
		rel, ok := adapter.NativeMCPConfigPath(target, scope)
		if !ok {
			continue
		}
		absolute, err := securePath(root, rel)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(absolute)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, errUnsafePath
		}
		body, err := os.ReadFile(absolute)
		if err != nil {
			return nil, err
		}
		existing[rel] = body
	}
	return existing, nil
}

func nativeLogicalPath(scope, rel string) string {
	if scope == "project" {
		return "project/" + rel
	}
	return "home/" + rel
}

func nativeLogicalPaths(
	byTarget map[string]map[string]adapter.MCPServerEntry,
	scope string,
) []string {
	paths := make([]string, 0, len(byTarget))
	for target := range byTarget {
		if rel, ok := adapter.NativeMCPConfigPath(target, scope); ok {
			paths = append(paths, nativeLogicalPath(scope, rel))
		}
	}
	sort.Strings(paths)
	return paths
}
