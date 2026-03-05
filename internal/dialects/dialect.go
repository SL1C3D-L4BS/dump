// Package dialects holds YAML definition files that map EDI/HL7 segment IDs
// to human-readable field names for structured output.

package dialects

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Dialect defines how to parse and name fields for an EDI format (e.g. HL7 ADT).
type Dialect struct {
	Name                string              `yaml:"name"`
	MessageStartSegment string              `yaml:"message_start_segment"`
	Segments            map[string][]string `yaml:"segments"`
	Delimiters          struct {
		Segment   string `yaml:"segment"`
		Field     string `yaml:"field"`
		Component string `yaml:"component"`
	} `yaml:"delimiters"`
}

// LoadDialect reads and parses a dialect YAML file from path.
func LoadDialect(path string) (*Dialect, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Dialect
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if d.Delimiters.Segment == "" {
		d.Delimiters.Segment = "\n"
	}
	if d.Delimiters.Field == "" {
		d.Delimiters.Field = "|"
	}
	if d.Delimiters.Component == "" {
		d.Delimiters.Component = "^"
	}
	if d.MessageStartSegment == "" {
		d.MessageStartSegment = "MSH"
	}
	return &d, nil
}
