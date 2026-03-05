package engine

import (
	"os"

	"gopkg.in/yaml.v3"
)

// MappingRule defines a single field mapping: source path (e.g. gjson path) → target field.
type MappingRule struct {
	TargetField string `yaml:"target_field"`
	SourcePath  string `yaml:"source_path"`
	Type        string `yaml:"type"`
}

// Schema is the YAML mapping manifest (matches LLM-generated schema).
type Schema struct {
	Rules []MappingRule `yaml:"rules"`
}

// LoadSchema reads and parses a YAML schema file from path.
func LoadSchema(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
