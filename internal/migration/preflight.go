package migration

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/build"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

const (
	codePreflightUnsupportedTarget = "preflight_unsupported_target"
	codePreflightAdapterDropped    = "preflight_adapter_dropped"
	codePreflightAdapterDegraded   = "preflight_adapter_degraded"
	codePreflightMissingSecret     = "preflight_missing_secret"
	codePreflightMCPInvalid        = "preflight_mcp_invalid"
)

// PreflightOptions selects the profile and local device roots inspected before
// publishing or applying a migration package.
type PreflightOptions struct {
	WorkspaceRoot string
	ManifestPath  string
	Profile       string
	Home          string
	Project       string
}

// DevicePrivateItem is local harness state that is intentionally excluded
// from cross-device migration.
type DevicePrivateItem struct {
	LogicalPath string `json:"logicalPath"`
	Source      string `json:"source"`
	Type        string `json:"type"`
	Status      string `json:"status"`
}

// TargetPreflight reports what an adapter can emit for one selected target.
type TargetPreflight struct {
	Target    string   `json:"target"`
	Supported bool     `json:"supported"`
	Emitted   int      `json:"emitted"`
	Dropped   []string `json:"dropped"`
	Degraded  []string `json:"degraded"`
}

type PreflightSummary struct {
	TargetCount        int `json:"targetCount"`
	UnsupportedTargets int `json:"unsupportedTargets"`
	DroppedItems       int `json:"droppedItems"`
	DegradedItems      int `json:"degradedItems"`
	SecretReferences   int `json:"secretReferences"`
	MissingSecrets     int `json:"missingSecrets"`
	DevicePrivateItems int `json:"devicePrivateItems"`
}

// PreflightReport is a deterministic, value-free readiness checklist.
type PreflightReport struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Kind          string                        `json:"kind"`
	ProducedBy    string                        `json:"producedBy"`
	Ok            bool                          `json:"ok"`
	Profile       string                        `json:"profile"`
	Targets       []TargetPreflight             `json:"targets"`
	Secrets       []apply.SecretReferenceStatus `json:"secrets"`
	DevicePrivate []DevicePrivateItem           `json:"devicePrivate"`
	Summary       PreflightSummary              `json:"summary"`
	Findings      []workspace.Finding           `json:"findings"`
}

// InspectPreflight evaluates one migration profile without building, publishing,
// pulling, applying, or writing any repository or device state.
func InspectPreflight(options PreflightOptions) (PreflightReport, error) {
	report := PreflightReport{
		SchemaVersion: 1,
		Kind:          "migration-preflight",
		ProducedBy:    version.ProducedBy(),
		Profile:       strings.TrimSpace(options.Profile),
		Targets:       []TargetPreflight{},
		Secrets:       []apply.SecretReferenceStatus{},
		DevicePrivate: []DevicePrivateItem{},
		Findings:      []workspace.Finding{},
	}
	if strings.TrimSpace(options.WorkspaceRoot) == "" ||
		report.Profile == "" ||
		strings.TrimSpace(options.Home) == "" {
		return report, build.ErrInvalidOptions
	}

	inventoryReport, err := inventory.Scan(inventory.Options{
		Home: options.Home, Project: options.Project,
	})
	if err != nil {
		return report, err
	}
	for _, entry := range inventoryReport.Entries {
		if entry.Scope != inventory.ScopeDevicePrivate {
			continue
		}
		report.DevicePrivate = append(report.DevicePrivate, DevicePrivateItem{
			LogicalPath: entry.LogicalPath,
			Source:      string(entry.Source),
			Type:        string(entry.Type),
			Status:      string(entry.Status),
		})
	}

	manifestPath := strings.TrimSpace(options.ManifestPath)
	if manifestPath == "" {
		manifestPath = filepath.Join(options.WorkspaceRoot, "manifest.yaml")
	}
	prepared, buildReport, err := build.Prepare(build.PrepareOptions{
		Manifest: manifestPath,
		Root:     options.WorkspaceRoot,
		Profile:  report.Profile,
	})
	if err != nil {
		return report, err
	}
	report.Findings = append(report.Findings, buildReport.Findings...)
	if !buildReport.Ok {
		return finishPreflight(report), nil
	}

	selected, rejected := adapter.ResolveTargets(nil, prepared.Manifest.Targets)
	for _, target := range rejected {
		report.Targets = append(report.Targets, TargetPreflight{
			Target: target, Supported: false, Dropped: []string{}, Degraded: []string{},
		})
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codePreflightUnsupportedTarget,
			Severity: workspace.SeverityError,
			Message:  "The selected profile includes a target with no built-in adapter.",
			Paths:    []string{"targets/" + target},
		})
	}

	staged, compileReports := adapter.CompileTargets(
		prepared.Manifest,
		prepared.Files,
		selected,
	)
	for _, compiled := range compileReports {
		target := TargetPreflight{
			Target:    compiled.Target,
			Supported: true,
			Emitted:   compiled.Emitted,
			Dropped:   append([]string(nil), compiled.Dropped...),
			Degraded:  append([]string(nil), compiled.Degraded...),
		}
		report.Targets = append(report.Targets, target)
		for _, item := range target.Dropped {
			report.Findings = append(report.Findings, workspace.Finding{
				Code:     codePreflightAdapterDropped,
				Severity: workspace.SeverityError,
				Message:  "The target adapter would omit a selected asset.",
				Paths:    []string{"targets/" + target.Target, item},
			})
		}
		for _, item := range target.Degraded {
			report.Findings = append(report.Findings, workspace.Finding{
				Code:     codePreflightAdapterDegraded,
				Severity: workspace.SeverityWarning,
				Message:  "The target adapter would install an asset with reduced semantics.",
				Paths:    []string{"targets/" + target.Target, item},
			})
		}
	}

	secrets, err := apply.InspectSecretReferences(staged)
	if err != nil {
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codePreflightMCPInvalid,
			Severity: workspace.SeverityError,
			Message:  "A selected MCP template cannot be checked safely.",
			Paths:    []string{"mcp"},
		})
		return finishPreflight(report), nil
	}
	report.Secrets = secrets
	for _, secret := range secrets {
		if secret.Available {
			continue
		}
		report.Findings = append(report.Findings, workspace.Finding{
			Code:     codePreflightMissingSecret,
			Severity: workspace.SeverityError,
			Message:  "A secret reference is not available on this device.",
			Paths: []string{
				fmt.Sprintf("secrets/%s/%s", secret.Provider, secret.Name),
			},
		})
	}
	return finishPreflight(report), nil
}

func finishPreflight(report PreflightReport) PreflightReport {
	sort.Slice(report.DevicePrivate, func(i, j int) bool {
		return report.DevicePrivate[i].LogicalPath < report.DevicePrivate[j].LogicalPath
	})
	sort.Slice(report.Targets, func(i, j int) bool {
		return report.Targets[i].Target < report.Targets[j].Target
	})
	sort.SliceStable(report.Findings, func(i, j int) bool {
		left, right := report.Findings[i], report.Findings[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return strings.Join(left.Paths, "\x00") < strings.Join(right.Paths, "\x00")
	})

	summary := PreflightSummary{
		TargetCount:        len(report.Targets),
		SecretReferences:   len(report.Secrets),
		DevicePrivateItems: len(report.DevicePrivate),
	}
	for _, target := range report.Targets {
		if !target.Supported {
			summary.UnsupportedTargets++
		}
		summary.DroppedItems += len(target.Dropped)
		summary.DegradedItems += len(target.Degraded)
	}
	for _, secret := range report.Secrets {
		if !secret.Available {
			summary.MissingSecrets++
		}
	}
	report.Summary = summary
	report.Ok = !workspace.HasError(report.Findings)
	return report
}
