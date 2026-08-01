// Package mcp exposes a read-only subset of aiah over the Model Context
// Protocol so AI tools can inspect assets and deployments without being able to
// modify them.
//
// The surface is deliberately narrower than the CLI. apply and rollback write
// the user's HOME, and Claude Code reads its own configuration from that same
// HOME: letting an agent invoke them means letting it rewrite its own runtime
// configuration mid-session, where a harness reload can leave the result
// unpredictable and a single bad prompt can damage dotfiles. build is excluded
// for a quieter reason -- it is the only otherwise-eligible command that writes
// at all, and its destination is caller-supplied, so an agent could aim it at
// ~/.claude. Leaving it out makes the invariant absolute and testable: no tool
// reachable through this server writes anything.
//
// Every handler must preserve that invariant. See ADR-0005.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/migration"
	"github.com/dff652/ai-asset-hub/internal/readiness"
	"github.com/dff652/ai-asset-hub/internal/validate"
	"github.com/dff652/ai-asset-hub/internal/version"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

// errInvalidArguments marks a caller mistake (bad or unknown argument) as
// opposed to a failure inside the underlying command.
var errInvalidArguments = errors.New("invalid tool arguments")

// Tool is one callable entry of the read-only surface.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	// Handler decodes its own arguments and returns a JSON-encodable report.
	// It must not write to the filesystem.
	Handler func(arguments json.RawMessage) (any, error)
}

// Tools returns the registered read-only tools ordered by name.
//
// Adding a tool here is a boundary change, not a feature toggle: anything that
// writes belongs on the CLI, not on this surface.
func Tools() []Tool {
	tools := []Tool{
		{
			Name: "aiah_asset_status",
			Description: "Compare discovered Claude Code, Codex, Grok and shared assets with " +
				"one editable asset library. Reports unmanaged, managed, source-changed, " +
				"library-only and blocked items without modifying either side.",
			InputSchema: objectSchema(map[string]any{
				"workspace": stringProperty("Asset library directory containing manifest.yaml."),
				"manifest": stringProperty("Optional manifest path inside the asset library. " +
					"Defaults to <workspace>/manifest.yaml."),
				"home":    stringProperty("Home directory to scan. Defaults to the current user's home."),
				"project": stringProperty("Optional project directory to scan for project-scoped assets."),
			}, []string{"workspace"}),
			Handler: handleAssetStatus,
		},
		{
			Name: "aiah_scan",
			Description: "Read-only inventory of Claude Code, Codex, Grok and shared AI assets " +
				"under a home directory and an optional project directory. Never writes, " +
				"never executes hooks, never follows escaping symlinks.",
			InputSchema: objectSchema(map[string]any{
				"home":    stringProperty("Home directory to scan. Defaults to the current user's home."),
				"project": stringProperty("Optional project directory to scan for project-scoped assets."),
			}, nil),
			Handler: handleScan,
		},
		{
			Name: "aiah_validate",
			Description: "Validate an asset workspace manifest and report findings. " +
				"Reads the workspace only.",
			InputSchema: objectSchema(map[string]any{
				"manifest": stringProperty("Path to manifest.yaml or manifest.json."),
				"root":     stringProperty("Workspace root. Defaults to the manifest directory."),
			}, []string{"manifest"}),
			Handler: handleValidate,
		},
		{
			Name: "aiah_diff",
			Description: "Plan what applying a package would change, without writing anything. " +
				"This is the read-only half of apply; use it to answer " +
				"\"what would this package do to my machine?\".",
			InputSchema: objectSchema(map[string]any{
				"package": stringProperty("Package .tar file or extracted package directory."),
				"home":    stringProperty("Target home directory."),
				"project": stringProperty("Optional project directory for project-scoped assets."),
				"targets": arrayProperty("Optional target subset, e.g. [\"claude\",\"codex\",\"grok\"]."),
			}, []string{"package"}),
			Handler: handleDiff,
		},
		{
			Name: "aiah_doctor",
			Description: "Read-only health check of a deployment: unfinished journals, leftover " +
				"staging directories, backup integrity, deployment drift and MCP native " +
				"config preconditions.",
			InputSchema: objectSchema(map[string]any{
				"home":    stringProperty("Home directory to inspect. Defaults to the current user's home."),
				"project": stringProperty("Optional project directory used at apply time."),
			}, nil),
			Handler: handleDoctor,
		},
		{
			Name: "aiah_migration_status",
			Description: "Read-only cross-device status for one asset library, the current " +
				"managed installation and an optional immutable release channel. Reports " +
				"version alignment but never builds, publishes, pulls or applies.",
			InputSchema: objectSchema(map[string]any{
				"workspace": stringProperty("Asset library directory containing manifest.yaml."),
				"manifest": stringProperty("Optional manifest path inside the asset library. " +
					"Defaults to <workspace>/manifest.yaml."),
				"channel": stringProperty("Optional existing Git, NAS or removable-media channel directory."),
				"home":    stringProperty("Home directory to inspect. Defaults to the current user's home."),
				"project": stringProperty("Optional project directory used by the managed installation."),
			}, []string{"workspace"}),
			Handler: handleMigrationStatus,
		},
		{
			Name: "aiah_migration_readiness",
			Description: "Read-only migration preparation report for one asset-library profile: " +
				"package readiness, migration preflight, optional external-copy evidence and " +
				"restore-exercise evidence. Never builds, publishes, pulls, applies, rolls back " +
				"or creates evidence files.",
			InputSchema: objectSchema(map[string]any{
				"workspace": stringProperty("Asset library directory containing manifest.yaml."),
				"manifest": stringProperty("Optional manifest path inside the asset library. " +
					"Defaults to <workspace>/manifest.yaml."),
				"profile": stringProperty("Profile name from the manifest. Required; never guessed."),
				"home":    stringProperty("Home directory to inspect. Defaults to the current user's home."),
				"project": stringProperty("Optional project directory used by migration preflight."),
				"backupEvidence": stringProperty(
					"Optional external-copy evidence file under <workspace>/.aiah/evidence.",
				),
				"restoreExercise": stringProperty(
					"Optional restore-exercise evidence file under <workspace>/.aiah/evidence.",
				),
			}, []string{"workspace", "profile"}),
			Handler: handleMigrationReadiness,
		},
		{
			Name:        "aiah_version",
			Description: "Report the aiah build identity (version, commit, build date).",
			InputSchema: objectSchema(map[string]any{}, nil),
			Handler:     handleVersion,
		},
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

type assetStatusArguments struct {
	Workspace string `json:"workspace"`
	Manifest  string `json:"manifest"`
	Home      string `json:"home"`
	Project   string `json:"project"`
}

func handleAssetStatus(arguments json.RawMessage) (any, error) {
	var args assetStatusArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Workspace) == "" {
		return nil, fmt.Errorf("%w: workspace is required", errInvalidArguments)
	}
	home, err := resolveHome(args.Home)
	if err != nil {
		return nil, err
	}
	inventoryReport, err := inventory.Scan(inventory.Options{
		Home: home, Project: args.Project,
	})
	if err != nil {
		if errors.Is(err, inventory.ErrInvalidRoot) {
			return nil, fmt.Errorf(
				"%w: home or project is not an accessible directory",
				errInvalidArguments,
			)
		}
		return nil, errors.New("asset status scan failed")
	}
	report, err := workspace.Catalog(workspace.CatalogOptions{
		WorkspaceRoot: args.Workspace,
		ManifestPath:  args.Manifest,
		Home:          home,
		Project:       args.Project,
		Assets:        inventoryReport.Assets,
	})
	if err != nil {
		switch {
		case errors.Is(err, workspace.ErrInvalidOptions):
			return nil, fmt.Errorf(
				"%w: workspace or manifest path is invalid",
				errInvalidArguments,
			)
		case errors.Is(err, workspace.ErrInvalidManifest):
			return nil, errors.New("asset status failed: manifest is invalid")
		default:
			return nil, errors.New("asset status failed")
		}
	}
	return report, nil
}

type scanArguments struct {
	Home    string `json:"home"`
	Project string `json:"project"`
}

func handleScan(arguments json.RawMessage) (any, error) {
	var args scanArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	home, err := resolveHome(args.Home)
	if err != nil {
		return nil, err
	}
	report, err := inventory.Scan(inventory.Options{Home: home, Project: args.Project})
	if err != nil {
		if errors.Is(err, inventory.ErrInvalidRoot) {
			return nil, fmt.Errorf("%w: scan root is not an accessible directory", errInvalidArguments)
		}
		return nil, errors.New("scan failed")
	}
	return report, nil
}

type validateArguments struct {
	Manifest string `json:"manifest"`
	Root     string `json:"root"`
}

func handleValidate(arguments json.RawMessage) (any, error) {
	var args validateArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Manifest) == "" {
		return nil, fmt.Errorf("%w: manifest is required", errInvalidArguments)
	}
	report, err := validate.Validate(validate.Options{Manifest: args.Manifest, Root: args.Root})
	if err != nil {
		if errors.Is(err, validate.ErrInvalidOptions) {
			return nil, fmt.Errorf("%w: validate options are invalid", errInvalidArguments)
		}
		return nil, errors.New("validate failed")
	}
	return report, nil
}

type diffArguments struct {
	Package string   `json:"package"`
	Home    string   `json:"home"`
	Project string   `json:"project"`
	Targets []string `json:"targets"`
}

func handleDiff(arguments json.RawMessage) (any, error) {
	var args diffArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Package) == "" {
		return nil, fmt.Errorf("%w: package is required", errInvalidArguments)
	}
	if args.Home == "" && args.Project == "" {
		home, err := resolveHome("")
		if err != nil {
			return nil, err
		}
		args.Home = home
	}
	// apply.Diff, not apply.Apply with DryRun: the read-only entry point should
	// not be a flag away from the writing one.
	report, err := apply.Diff(apply.Options{
		Package: args.Package,
		Home:    args.Home,
		Project: args.Project,
		Targets: args.Targets,
	})
	if err != nil {
		if errors.Is(err, apply.ErrInvalidOptions) {
			return nil, fmt.Errorf("%w: diff options are invalid", errInvalidArguments)
		}
		return nil, errors.New("diff failed")
	}
	return report, nil
}

type doctorArguments struct {
	Home    string `json:"home"`
	Project string `json:"project"`
}

func handleDoctor(arguments json.RawMessage) (any, error) {
	var args doctorArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	home, err := resolveHome(args.Home)
	if err != nil {
		return nil, err
	}
	report, err := apply.Doctor(apply.DoctorOptions{Home: home, Project: args.Project})
	if err != nil {
		if errors.Is(err, apply.ErrInvalidOptions) {
			return nil, fmt.Errorf("%w: doctor root is not an accessible directory", errInvalidArguments)
		}
		return nil, errors.New("doctor failed")
	}
	return report, nil
}

type migrationStatusArguments struct {
	Workspace string `json:"workspace"`
	Manifest  string `json:"manifest"`
	Channel   string `json:"channel"`
	Home      string `json:"home"`
	Project   string `json:"project"`
}

func handleMigrationStatus(arguments json.RawMessage) (any, error) {
	var args migrationStatusArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Workspace) == "" {
		return nil, fmt.Errorf("%w: workspace is required", errInvalidArguments)
	}
	home, err := resolveHome(args.Home)
	if err != nil {
		return nil, err
	}
	report, err := migration.Inspect(migration.Options{
		WorkspaceRoot: args.Workspace,
		ManifestPath:  args.Manifest,
		Channel:       args.Channel,
		Home:          home,
		Project:       args.Project,
	})
	if err != nil {
		switch {
		case errors.Is(err, workspace.ErrInvalidOptions),
			errors.Is(err, apply.ErrInvalidOptions),
			errors.Is(err, channel.ErrChannelBlocked):
			return nil, fmt.Errorf(
				"%w: workspace, manifest, channel, home or project is invalid",
				errInvalidArguments,
			)
		case errors.Is(err, workspace.ErrInvalidManifest):
			return nil, errors.New("migration status failed: manifest is invalid")
		default:
			return nil, errors.New("migration status failed")
		}
	}
	return report, nil
}

type migrationReadinessArguments struct {
	Workspace       string `json:"workspace"`
	Manifest        string `json:"manifest"`
	Profile         string `json:"profile"`
	Home            string `json:"home"`
	Project         string `json:"project"`
	BackupEvidence  string `json:"backupEvidence"`
	RestoreExercise string `json:"restoreExercise"`
}

func handleMigrationReadiness(arguments json.RawMessage) (any, error) {
	var args migrationReadinessArguments
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Workspace) == "" {
		return nil, fmt.Errorf("%w: workspace is required", errInvalidArguments)
	}
	if strings.TrimSpace(args.Profile) == "" {
		return nil, fmt.Errorf("%w: profile is required", errInvalidArguments)
	}
	home, err := resolveHome(args.Home)
	if err != nil {
		return nil, err
	}
	report, err := readiness.Inspect(readiness.Options{
		WorkspaceRoot:       args.Workspace,
		ManifestPath:        args.Manifest,
		Profile:             args.Profile,
		Home:                home,
		Project:             args.Project,
		BackupEvidencePath:  args.BackupEvidence,
		RestoreExercisePath: args.RestoreExercise,
	})
	if err != nil {
		switch {
		case errors.Is(err, readiness.ErrInvalidOptions),
			errors.Is(err, workspace.ErrInvalidOptions):
			return nil, fmt.Errorf(
				"%w: workspace, manifest, profile, home, project or evidence path is invalid",
				errInvalidArguments,
			)
		default:
			return nil, errors.New("migration readiness failed")
		}
	}
	return report, nil
}

func handleVersion(arguments json.RawMessage) (any, error) {
	var args struct{}
	if err := decodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	return struct {
		SchemaVersion int    `json:"schemaVersion"`
		Kind          string `json:"kind"`
		ProducedBy    string `json:"producedBy"`
		Version       string `json:"version"`
		Commit        string `json:"commit"`
		Date          string `json:"date"`
	}{1, "version", version.ProducedBy(), version.Version, version.Commit, version.Date}, nil
}

// decodeArguments rejects unknown fields so a misspelled argument fails loudly
// instead of silently changing which paths a tool reads.
func decodeArguments(arguments json.RawMessage, target any) error {
	trimmed := strings.TrimSpace(string(arguments))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %s", errInvalidArguments, err.Error())
	}
	return nil
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func arrayProperty(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": description,
	}
}
