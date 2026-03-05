// Package healthcare provides public-facing SDK components for HL7 v2, X12 EDI, FHIR, and the Dialect Pack.

package healthcare

import (
	"embed"
	"fmt"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
)

//go:embed std/*.yaml
var stdFS embed.FS

// LoadStandardDialect retrieves and parses an embedded standard dialect by name.
// Valid names: hl7_v25, x12_837, x12_835. Returns a dialect for use with EDI/X12 readers.
func LoadStandardDialect(name string) (*dialects.Dialect, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	path := "std/" + name + ".yaml"
	data, err := stdFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unknown standard dialect %q (valid: hl7_v25, x12_837, x12_835): %w", name, err)
	}
	return dialects.ParseDialect(data)
}
