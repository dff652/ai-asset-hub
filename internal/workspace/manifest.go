package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidManifest means the file is not a usable manifest document.
	ErrInvalidManifest = errors.New("invalid manifest")
	// ErrInvalidOptions means required options are missing or unusable.
	ErrInvalidOptions = errors.New("invalid workspace options")
)

// Document is the in-memory manifest v1 shape used after schema validation.
type Document struct {
	SchemaVersion int                `json:"schemaVersion" yaml:"schemaVersion"`
	Name          string             `json:"name" yaml:"name"`
	Version       string             `json:"version" yaml:"version"`
	Assets        []ManifestAsset    `json:"assets" yaml:"assets"`
	Profiles      map[string]Profile `json:"profiles" yaml:"profiles"`
}

type ManifestAsset struct {
	ID           string   `json:"id" yaml:"id"`
	Type         string   `json:"type" yaml:"type"`
	Path         string   `json:"path" yaml:"path"`
	Targets      []string `json:"targets" yaml:"targets"`
	Scope        string   `json:"scope" yaml:"scope"`
	Portability  string   `json:"portability" yaml:"portability"`
	Sensitivity  string   `json:"sensitivity" yaml:"sensitivity"`
	Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Conflicts    []string `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
}

type Profile struct {
	Include []string `json:"include" yaml:"include"`
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	Targets []string `json:"targets,omitempty" yaml:"targets,omitempty"`
}

// LoadManifest parses a YAML or JSON manifest into a typed document and a
// schema-friendly normalized value. It does not validate against the schema.
func LoadManifest(path string) (Document, any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, nil, fmt.Errorf("%w: read manifest", err)
	}
	var document any
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(raw, &document); err != nil {
			return Document{}, nil, fmt.Errorf("%w: parse yaml", ErrInvalidManifest)
		}
	case ".json":
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return Document{}, nil, fmt.Errorf("%w: parse json", ErrInvalidManifest)
		}
	default:
		if err := yaml.Unmarshal(raw, &document); err != nil {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&document); err != nil {
				return Document{}, nil, fmt.Errorf("%w: parse manifest", ErrInvalidManifest)
			}
		}
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return Document{}, nil, fmt.Errorf("%w: normalize manifest", ErrInvalidManifest)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return Document{}, nil, fmt.Errorf("%w: normalize manifest", ErrInvalidManifest)
	}
	var typed Document
	if err := json.Unmarshal(encoded, &typed); err != nil {
		return Document{}, nil, fmt.Errorf("%w: decode manifest", ErrInvalidManifest)
	}
	return typed, normalized, nil
}
