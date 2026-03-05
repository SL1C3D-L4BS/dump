package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultOllamaURL = "http://localhost:11434"

// SchemaInferencer calls a local Ollama instance to infer a YAML mapping from sample data.
type SchemaInferencer struct {
	BaseURL     string
	HTTPClient  *http.Client
}

// NewSchemaInferencer returns a SchemaInferencer. If client is nil, http.DefaultClient is used.
func NewSchemaInferencer(baseURL string, client *http.Client) *SchemaInferencer {
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &SchemaInferencer{BaseURL: baseURL, HTTPClient: client}
}

// InferMapping sends the sample data to Ollama and returns the raw YAML mapping for the target format.
// model is the Ollama model name (e.g. "llama3"). sourceFormat may be "json", "xml", or "edi"
// to tailor the system prompt. The response is cleaned of markdown backticks if present.
func (s *SchemaInferencer) InferMapping(sampleData []byte, targetFormat string, model string, sourceFormat string) (string, error) {
	if model == "" {
		model = "llama3"
	}
	prompt := buildInferPrompt(string(sampleData), targetFormat, sourceFormat)
	reqBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(s.BaseURL, "/") + "/api/generate"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama api error %d: %s", resp.StatusCode, string(slurp))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return stripMarkdownBackticks(result.Response), nil
}

// AnalyzeAndInfer takes a normalized JSON data sample (e.g. from ExtractSample), infers
// semantics, and returns a DUMP YAML mapping for the target format. Used by dump analyze.
func (s *SchemaInferencer) AnalyzeAndInfer(sampleData string, targetFormat string, model string) (string, error) {
	if model == "" {
		model = "llama3"
	}
	prompt := "You are an elite data architect. Here is a normalized JSON data sample extracted from a mystery source. Infer its semantic meaning (e.g., 'customer transactions', 'server logs'). Produce a DUMP YAML mapping that maps these fields to a clean " + targetFormat + " schema. CRITICAL: OUTPUT ONLY VALID YAML. DO NOT USE MARKDOWN FORMATTING. DO NOT WRAP IN BACKTICKS. NO EXPLANATIONS.\n\n" + sampleData
	reqBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	url := strings.TrimSuffix(s.BaseURL, "/") + "/api/generate"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama api error %d: %s", resp.StatusCode, string(slurp))
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return stripMarkdownBackticks(result.Response), nil
}

// buildInferPrompt returns the prompt text based on sourceFormat (json, xml, edi).
func buildInferPrompt(sampleData, targetFormat, sourceFormat string) string {
	base := "You are an elite data architect. "
	switch sourceFormat {
	case "xml":
		base += fmt.Sprintf("Analyze this XML sample. Produce a DUMP YAML mapping that flattens this structure into a clean %s schema. ", targetFormat)
	case "edi":
		base += fmt.Sprintf("Analyze this raw EDI/HL7 segment sample. Produce a DUMP YAML mapping that translates these segments into a structured %s schema. ", targetFormat)
	default:
		base += fmt.Sprintf("Analyze this JSON sample. Generate a YAML mapping file that converts this data into a valid %s schema. ", targetFormat)
	}
	base += "CRITICAL: OUTPUT ONLY VALID YAML. DO NOT USE MARKDOWN FORMATTING. DO NOT WRAP IN BACKTICKS. NO EXPLANATIONS. Sample: " + sampleData
	return base
}

// stripMarkdownBackticks removes ```yaml ... ``` or ``` ... ``` wrappers from the model output.
func stripMarkdownBackticks(s string) string {
	s = strings.TrimSpace(s)
	const yamlBlock = "```yaml"
	const block = "```"
	if strings.HasPrefix(s, yamlBlock) {
		s = strings.TrimPrefix(s, yamlBlock)
	} else if strings.HasPrefix(s, block) {
		s = strings.TrimPrefix(s, block)
		if rest := strings.TrimPrefix(s, "yaml"); rest != s {
			s = rest
		}
	}
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, block) {
		s = strings.TrimSuffix(s, block)
	}
	return strings.TrimSpace(s)
}
