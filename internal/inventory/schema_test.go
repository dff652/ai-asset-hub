package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestReportsValidateAgainstInventorySchema(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "spec", "inventory.schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schemaDocument any
	if err := json.Unmarshal(schemaData, &schemaDocument); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("inventory.schema.json", schemaDocument); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	schema, err := compiler.Compile("inventory.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	cases := []Options{
		{Home: fixture(t, "home-basic")},
		{Home: fixture(t, "home-sensitive")},
		{Home: fixture(t, "home-grok")},
		{
			Home:    fixture(t, "home-conflicts"),
			Project: filepath.Join(fixture(t, "home-conflicts"), "workspace"),
		},
		{
			Home:    t.TempDir(),
			Project: filepath.Join(fixture(t, "home-grok-project"), "workspace"),
		},
		{Home: t.TempDir()},
	}
	for _, options := range cases {
		report, err := Scan(options)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		assertInventoryRelations(t, report)
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		var document any
		if err := json.Unmarshal(encoded, &document); err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if err := schema.Validate(document); err != nil {
			t.Fatalf("report failed inventory schema: %v", err)
		}
	}
}

func assertInventoryRelations(t *testing.T, report Report) {
	t.Helper()
	entryByPath := make(map[string]Entry, len(report.Entries))
	for _, entry := range report.Entries {
		if _, exists := entryByPath[entry.LogicalPath]; exists {
			t.Fatalf("duplicate entry path %q", entry.LogicalPath)
		}
		entryByPath[entry.LogicalPath] = entry
	}

	candidateByType := make(map[AssetType]int)
	statusCounts := make(map[AssetStatus]int)
	assetPaths := make(map[string]bool, len(report.Assets))
	for _, asset := range report.Assets {
		if assetPaths[asset.LogicalPath] {
			t.Fatalf("duplicate asset path %q", asset.LogicalPath)
		}
		assetPaths[asset.LogicalPath] = true
		statusCounts[asset.Status]++
		if asset.Status == AssetCandidate {
			candidateByType[asset.Type]++
		}
		for _, file := range asset.Files {
			entry, exists := entryByPath[file]
			if !exists {
				t.Fatalf("asset %q references missing entry %q", asset.LogicalPath, file)
			}
			if entry.AssetPath != asset.LogicalPath {
				t.Fatalf("entry %q points to %q, want %q", file, entry.AssetPath, asset.LogicalPath)
			}
		}
	}
	for _, entry := range report.Entries {
		if entry.AssetPath != "" && !assetPaths[entry.AssetPath] {
			t.Fatalf("entry %q references missing asset %q", entry.LogicalPath, entry.AssetPath)
		}
	}

	if report.Summary.TotalAssets != len(report.Assets) ||
		report.Summary.CandidateAssets != statusCounts[AssetCandidate] ||
		report.Summary.ExcludedAssets != statusCounts[AssetExcluded] ||
		report.Summary.UnreadableAssets != statusCounts[AssetUnreadable] {
		t.Fatalf("asset summary does not match assets: %#v", report.Summary)
	}
	if !reflect.DeepEqual(report.Summary.CandidateByType, candidateByType) {
		t.Fatalf("candidate type summary = %#v, want %#v",
			report.Summary.CandidateByType, candidateByType)
	}
}
