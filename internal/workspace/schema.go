package workspace

import (
	_ "embed"
	"encoding/json"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed manifest.schema.json
var manifestSchemaJSON []byte

// ValidateManifestSchema checks a normalized manifest document against the
// embedded manifest v1 schema.
func ValidateManifestSchema(document any) error {
	var schemaDocument any
	if err := json.Unmarshal(manifestSchemaJSON, &schemaDocument); err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("manifest.schema.json", schemaDocument); err != nil {
		return err
	}
	schema, err := compiler.Compile("manifest.schema.json")
	if err != nil {
		return err
	}
	return schema.Validate(document)
}

// ManifestSchemaBytes returns the embedded schema for tests that compare to spec/.
func ManifestSchemaBytes() []byte {
	return append([]byte(nil), manifestSchemaJSON...)
}
