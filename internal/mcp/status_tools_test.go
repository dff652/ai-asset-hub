package mcp

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/migration"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

func TestAssetStatusReturnsTheUnifiedCoreReport(t *testing.T) {
	home := copyTree(t, filepath.Join("..", "..", "testdata", "home-basic"))
	workspaceRoot := copyTree(t, filepath.Join("..", "..", "testdata", "workspace-valid"))
	encoded, err := json.Marshal(map[string]any{
		"workspace": workspaceRoot,
		"home":      home,
	})
	if err != nil {
		t.Fatal(err)
	}

	value, err := handleAssetStatus(encoded)
	if err != nil {
		t.Fatalf("asset status: %v", err)
	}
	report, ok := value.(workspace.CatalogReport)
	if !ok {
		t.Fatalf("report type = %T, want workspace.CatalogReport", value)
	}
	if !report.Ok || report.Kind != "asset-catalog" ||
		report.ManifestPath != filepath.Join(workspaceRoot, "manifest.yaml") {
		t.Fatalf("asset status report = %#v", report)
	}
}

func TestMigrationStatusReturnsTheCrossDeviceCoreReport(t *testing.T) {
	home := copyTree(t, filepath.Join("..", "..", "testdata", "home-basic"))
	workspaceRoot := copyTree(t, filepath.Join("..", "..", "testdata", "workspace-valid"))
	channelRoot := t.TempDir()
	pkg := buildFixturePackage(t)
	if _, err := channel.Publish(channel.PublishOptions{
		Package: pkg, Channel: channelRoot,
	}); err != nil {
		t.Fatalf("publish fixture package: %v", err)
	}
	encoded, err := json.Marshal(map[string]any{
		"workspace": workspaceRoot,
		"channel":   channelRoot,
		"home":      home,
	})
	if err != nil {
		t.Fatal(err)
	}

	value, err := handleMigrationStatus(encoded)
	if err != nil {
		t.Fatalf("migration status: %v", err)
	}
	report, ok := value.(migration.Report)
	if !ok {
		t.Fatalf("report type = %T, want migration.Report", value)
	}
	if !report.Ok || report.Kind != "migration-status" ||
		report.Library.Root != workspaceRoot || !report.Channel.Selected {
		t.Fatalf("migration status report = %#v", report)
	}
}

func TestStatusToolsRequireAnExplicitAssetLibrary(t *testing.T) {
	tests := []struct {
		name    string
		handler func(json.RawMessage) (any, error)
	}{
		{name: "asset status", handler: handleAssetStatus},
		{name: "migration status", handler: handleMigrationStatus},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := testCase.handler(json.RawMessage(`{}`)); !errors.Is(err, errInvalidArguments) {
				t.Fatalf("err = %v, want errInvalidArguments", err)
			}
		})
	}
}

func TestToolSchemasDeclareUnknownArgumentsInvalid(t *testing.T) {
	for _, tool := range Tools() {
		if tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("%s schema permits unknown arguments: %#v", tool.Name, tool.InputSchema)
		}
	}
}
