package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/migration"
	"github.com/dff652/ai-asset-hub/internal/readiness"
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
		{name: "migration readiness", handler: handleMigrationReadiness},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := testCase.handler(json.RawMessage(`{}`)); !errors.Is(err, errInvalidArguments) {
				t.Fatalf("err = %v, want errInvalidArguments", err)
			}
		})
	}
}

func TestMigrationReadinessReturnsSharedCoreReport(t *testing.T) {
	home := copyTree(t, filepath.Join("..", "..", "testdata", "home-basic"))
	workspaceRoot := copyTree(t, filepath.Join("..", "..", "testdata", "workspace-valid"))
	project := t.TempDir()
	beforeWorkspace := snapshotTree(t, workspaceRoot)
	beforeHome := snapshotTree(t, home)
	beforeProject := snapshotTree(t, project)

	encoded, err := json.Marshal(map[string]any{
		"workspace": workspaceRoot,
		"profile":   "personal",
		"home":      home,
		"project":   project,
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := handleMigrationReadiness(encoded)
	if err != nil {
		t.Fatalf("migration readiness: %v", err)
	}
	report, ok := value.(readiness.Report)
	if !ok {
		t.Fatalf("report type = %T, want readiness.Report", value)
	}
	expected, err := readiness.Inspect(readiness.Options{
		WorkspaceRoot: workspaceRoot,
		Profile:       "personal",
		Home:          home,
		Project:       project,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != "migration-readiness" || report.Level != expected.Level ||
		report.PackageReadiness.Status != expected.PackageReadiness.Status ||
		report.BackupEvidence.Status != readiness.StatusMissing ||
		report.RestoreExercise.Status != readiness.StatusMissing {
		t.Fatalf("readiness report = %#v\nexpected level/package=%s/%s",
			report, expected.Level, expected.PackageReadiness.Status)
	}
	for name, pair := range map[string][2]map[string]string{
		"workspace": {beforeWorkspace, snapshotTree(t, workspaceRoot)},
		"home":      {beforeHome, snapshotTree(t, home)},
		"project":   {beforeProject, snapshotTree(t, project)},
	} {
		if diff := treeDiff(pair[0], pair[1]); diff != "" {
			t.Fatalf("%s changed after readiness tool:\n%s", name, diff)
		}
	}
}

func TestMigrationReadinessRequiresProfileAndRejectsUnknownFields(t *testing.T) {
	workspaceRoot := copyTree(t, filepath.Join("..", "..", "testdata", "workspace-valid"))
	if _, err := handleMigrationReadiness(mustJSON(t, map[string]any{
		"workspace": workspaceRoot,
	})); !errors.Is(err, errInvalidArguments) {
		t.Fatalf("missing profile err = %v", err)
	}
	if _, err := handleMigrationReadiness(mustJSON(t, map[string]any{
		"workspace": workspaceRoot,
		"profile":   "personal",
		"unknown":   true,
	})); !errors.Is(err, errInvalidArguments) {
		t.Fatalf("unknown field err = %v", err)
	}
}

func TestMigrationReadinessProtocolCallReturnsSameShape(t *testing.T) {
	home := copyTree(t, filepath.Join("..", "..", "testdata", "home-basic"))
	workspaceRoot := copyTree(t, filepath.Join("..", "..", "testdata", "workspace-valid"))
	request := mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "aiah_migration_readiness",
			"arguments": map[string]any{
				"workspace": workspaceRoot,
				"profile":   "personal",
				"home":      home,
			},
		},
	})
	result := resultOf(t, serve(t, string(request))[0])
	if isError, ok := result["isError"].(bool); ok && isError {
		t.Fatalf("tool error: %v", result)
	}
	text := textContentOf(t, result)
	var report map[string]any
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report["kind"] != "migration-readiness" {
		t.Fatalf("kind = %v", report["kind"])
	}
	backup, _ := report["backupEvidence"].(map[string]any)
	restore, _ := report["restoreExercise"].(map[string]any)
	if backup["status"] != "missing" || restore["status"] != "missing" {
		t.Fatalf("evidence statuses = backup %#v restore %#v", backup, restore)
	}
}

func TestMigrationReadinessDoesNotLeakEvidenceReference(t *testing.T) {
	home := t.TempDir()
	workspaceRoot := copyTree(t, filepath.Join("..", "..", "testdata", "workspace-valid"))
	base, err := readiness.Inspect(readiness.Options{
		WorkspaceRoot: workspaceRoot, Profile: "personal", Home: home,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	privateRef := "git-commit:private-mcp-reference-must-not-leak"
	body := []byte(
		"schemaVersion: 1\nkind: backup-evidence\nsubject:\n  name: " + base.Subject.Name +
			"\n  version: \"" + base.Subject.Version + "\"\n  profile: " + base.Subject.Profile +
			"\n  selectionSHA256: " + base.Subject.SelectionSHA256 +
			"\ncopy:\n  type: git-commit\n  reference: \"" + privateRef +
			"\"\nrecordedAt: \"2026-08-01T00:00:00Z\"\n",
	)
	if err := os.WriteFile(filepath.Join(evidenceRoot, "backup.yaml"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := handleMigrationReadiness(mustJSON(t, map[string]any{
		"workspace":      workspaceRoot,
		"profile":        "personal",
		"home":           home,
		"backupEvidence": "backup.yaml",
	}))
	if err != nil {
		t.Fatal(err)
	}
	report := value.(readiness.Report)
	if report.BackupEvidence.Status != readiness.StatusRecorded {
		t.Fatalf("backup status = %s", report.BackupEvidence.Status)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), privateRef) {
		t.Fatal("MCP readiness report leaked full external reference")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestToolSchemasDeclareUnknownArgumentsInvalid(t *testing.T) {
	for _, tool := range Tools() {
		if tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("%s schema permits unknown arguments: %#v", tool.Name, tool.InputSchema)
		}
	}
}
