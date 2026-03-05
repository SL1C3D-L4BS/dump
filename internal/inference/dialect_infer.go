// Package inference: InferCustomDialect uses the LLM to infer custom (e.g. Z-) segment definitions.

package inference

import (
	"fmt"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
)

const customDialectSystemPrompt = `You are an elite healthcare data architect. Analyze the following raw EDI/HL7 sample. It contains standard segments and custom, undocumented segments (often starting with 'Z'). Based on the surrounding context and data payloads, infer the semantic meaning and field names of the custom segments.
Output Requirement: Produce a valid DUMP Dialect YAML containing ONLY the definitions for the custom segments.
Format:
segments:
  ZFBH: [Field1_Name, Field2_Name, ...]
  ZXXX: [Field1_Name, Field2_Name, ...]`

// InferCustomDialect sends the raw EDI/HL7 sample to the LLM and returns YAML defining
// only the custom (e.g. Z-) segments. baseDialect is used for context; the model infers
// segments not present in the standard. Uses the existing Ollama client (SchemaInferencer).
func InferCustomDialect(inf *SchemaInferencer, sampleData string, baseDialect *dialects.Dialect, model string) (string, error) {
	if model == "" {
		model = "llama3"
	}
	prompt := customDialectSystemPrompt + "\n\n---\n\n" + sampleData
	out, err := inf.Generate(model, prompt)
	if err != nil {
		return "", fmt.Errorf("infer custom dialect: %w", err)
	}
	return stripMarkdownBackticks(strings.TrimSpace(out)), nil
}
