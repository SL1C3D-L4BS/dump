// Package dialects: LoadStandardDialect loads embedded base standard dialect YAMLs.

package dialects

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed std/*.yaml
var stdFS embed.FS

// LoadStandardDialect retrieves and parses an embedded standard dialect by name.
// Valid names: hl7_v25, x12_837, x12_835.
func LoadStandardDialect(name string) (*Dialect, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	path := "std/" + name + ".yaml"
	data, err := stdFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unknown standard dialect %q (valid: hl7_v25, x12_837, x12_835): %w", name, err)
	}
	var d Dialect
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse standard dialect %s: %w", name, err)
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
	if d.TransactionBoundary.Start == "" {
		d.TransactionBoundary.Start = "ST"
	}
	if d.TransactionBoundary.End == "" {
		d.TransactionBoundary.End = "SE"
	}
	return &d, nil
}
