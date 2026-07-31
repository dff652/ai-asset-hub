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
	"github.com/dff652/ai-asset-hub/internal/pkgload"
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

const (
	codePreflightUnsupportedTarget = "preflight_unsupported_target"
	codePreflightAdapterDropped    = "preflight_adapter_dropped"
	codePreflightAdapterDegraded   = "preflight_adapter_degraded"
	codePreflightMissingSecret     = "preflight_missing_secret"
	codePreflightMCPInvalid        = "preflight_mcp_invalid"
	codePreflightReleaseMismatch   = "preflight_release_mismatch"
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

// PackagePreflightOptions checks one already retrieved immutable package on
// the target device. Expected fields bind the package to the exact channel
// release selected by the caller.
type PackagePreflightOptions struct {
	Package  string
	Home     string
	Project  string
	Expected ReleaseIdentity
}

// ReleaseIdentity is the immutable channel coordinate and archive digest a
// caller selected before target-device inspection.
type ReleaseIdentity struct {
	Name    string
	Version string
	Profile string
	SHA256  string
}

// PreflightSubject identifies the exact workspace selection or immutable
// package checked by this report.
type PreflightSubject struct {
	Source  string `json:"source"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Profile string `json:"profile"`
	Package string `json:"package,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
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
	Subject       PreflightSubject              `json:"subject"`
	Targets       []TargetPreflight             `json:"targets"`
	Secrets       []apply.SecretReferenceStatus `json:"secrets"`
	DevicePrivate []DevicePrivateItem           `json:"devicePrivate"`
	Summary       PreflightSummary              `json:"summary"`
	Findings      []workspace.Finding           `json:"findings"`
}

// InspectPreflight evaluates one migration profile without building, publishing,
// pulling, applying, or writing any repository or device state.
func InspectPreflight(options PreflightOptions) (PreflightReport, error) {
	report := newPreflightReport(strings.TrimSpace(options.Profile))
	if strings.TrimSpace(options.WorkspaceRoot) == "" ||
		report.Profile == "" ||
		strings.TrimSpace(options.Home) == "" {
		return report, build.ErrInvalidOptions
	}

	if err := inspectDevicePrivate(&report, options.Home, options.Project); err != nil {
		return report, err
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
	report.Subject = PreflightSubject{
		Source:  "workspace",
		Name:    prepared.Manifest.Name,
		Version: prepared.Manifest.Version,
		Profile: prepared.Manifest.Profile,
	}
	return inspectPrepared(report, prepared.Manifest, prepared.Files)
}

// InspectPackagePreflight evaluates the exact package selected from a channel.
// It reads package and device state but never builds, publishes, applies, or
// writes an asset/tool directory.
func InspectPackagePreflight(options PackagePreflightOptions) (PreflightReport, error) {
	report := newPreflightReport(strings.TrimSpace(options.Expected.Profile))
	if strings.TrimSpace(options.Package) == "" || strings.TrimSpace(options.Home) == "" {
		return report, build.ErrInvalidOptions
	}
	pkg, err := pkgload.Open(options.Package)
	if err != nil {
		return report, err
	}
	report.Profile = pkg.Manifest.Profile
	report.Subject = PreflightSubject{
		Source:  "package",
		Name:    pkg.Manifest.Name,
		Version: pkg.Manifest.Version,
		Profile: pkg.Manifest.Profile,
		Package: filepath.Base(pkg.Source),
		SHA256:  pkg.ArchiveSHA256,
	}
	checks := []struct {
		path string
		want string
		got  string
	}{
		{path: "name", want: options.Expected.Name, got: pkg.Manifest.Name},
		{path: "version", want: options.Expected.Version, got: pkg.Manifest.Version},
		{path: "profile", want: options.Expected.Profile, got: pkg.Manifest.Profile},
		{path: "sha256", want: options.Expected.SHA256, got: pkg.ArchiveSHA256},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.want) != "" && strings.TrimSpace(check.want) != check.got {
			report.Findings = append(report.Findings, workspace.Finding{
				Code:     codePreflightReleaseMismatch,
				Severity: workspace.SeverityError,
				Message:  "The retrieved package no longer matches the selected channel release.",
				Paths:    []string{"release/" + check.path},
			})
		}
	}
	if err := inspectDevicePrivate(&report, options.Home, options.Project); err != nil {
		return report, err
	}
	return inspectPrepared(report, pkg.Manifest, pkg.Files)
}

func newPreflightReport(profile string) PreflightReport {
	return PreflightReport{
		SchemaVersion: 1,
		Kind:          "migration-preflight",
		ProducedBy:    version.ProducedBy(),
		Profile:       profile,
		Subject:       PreflightSubject{Source: "workspace", Profile: profile},
		Targets:       []TargetPreflight{},
		Secrets:       []apply.SecretReferenceStatus{},
		DevicePrivate: []DevicePrivateItem{},
		Findings:      []workspace.Finding{},
	}
}

func inspectDevicePrivate(report *PreflightReport, home, project string) error {
	inventoryReport, err := inventory.Scan(inventory.Options{Home: home, Project: project})
	if err != nil {
		return err
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
	return nil
}

func inspectPrepared(
	report PreflightReport,
	manifest build.PackageManifest,
	files map[string][]byte,
) (PreflightReport, error) {
	selected, rejected := adapter.ResolveTargets(nil, manifest.Targets)
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
		manifest,
		files,
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
