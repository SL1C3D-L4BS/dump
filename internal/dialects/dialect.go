// Package dialects holds YAML definition files that map EDI/HL7 segment IDs
// to human-readable field names for structured output.

package dialects

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Dialect defines how to parse and name fields for an EDI format (e.g. HL7 ADT, X12 837/835).
type Dialect struct {
	Name                string              `yaml:"name"`
	MessageStartSegment string              `yaml:"message_start_segment"`
	Segments            map[string][]string  `yaml:"segments"`
	Delimiters          struct {
		Segment   string `yaml:"segment"`
		Field     string `yaml:"field"`
		Component string `yaml:"component"`
	} `yaml:"delimiters"`

	// X12: transaction boundaries (ST = start, SE = end of transaction set).
	TransactionBoundary struct {
		Start string `yaml:"start"` // default "ST"
		End   string `yaml:"end"`   // default "SE"
	} `yaml:"transaction_boundary"`

	// X12: when segment ID + element value match, enter the named loop (push to context).
	// Key = segment ID (e.g. "NM1"); each rule: ElementIndex + Value -> EnterLoop.
	LoopTriggers map[string][]LoopTriggerRule `yaml:"loop_triggers"`

	// X12: yield one row (claim) when this segment is seen (e.g. "CLM" for 837 claims).
	YieldTrigger struct {
		Segment string `yaml:"segment"`
	} `yaml:"yield_trigger"`
}

// LoopTriggerRule defines: when segment has element at ElementIndex equal to Value, enter EnterLoop.
type LoopTriggerRule struct {
	ElementIndex int    `yaml:"element_index"`
	Value        string `yaml:"value"`
	EnterLoop    string `yaml:"enter_loop"`
}

// ParseDialect parses a dialect from YAML bytes (e.g. LLM-generated custom dialect).
func ParseDialect(data []byte) (*Dialect, error) {
	var d Dialect
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return normalizeDialect(&d), nil
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
	return normalizeDialect(&d), nil
}

func normalizeDialect(d *Dialect) *Dialect {
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
	return d
}

// MergeDialect merges segment definitions from other into d (in place).
// Segments in other override or add to d.Segments. Other may be nil.
func (d *Dialect) MergeDialect(other *Dialect) {
	if other == nil || other.Segments == nil {
		return
	}
	if d.Segments == nil {
		d.Segments = make(map[string][]string)
	}
	for id, names := range other.Segments {
		d.Segments[id] = names
	}
}
