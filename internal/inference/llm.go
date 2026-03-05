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
// model is the Ollama model name (e.g. "llama3"). The response is cleaned of markdown backticks if present.
func (s *SchemaInferencer) InferMapping(sampleData []byte, targetFormat string, model string) (string, error) {
	if model == "" {
		model = "llama3"
	}
	prompt := fmt.Sprintf(
		"You are an elite data architect. Analyze this JSON sample: %s. Generate a YAML mapping file that converts this data into a valid %s schema. CRITICAL: OUTPUT ONLY VALID YAML. DO NOT USE MARKDOWN FORMATTING. DO NOT WRAP IN BACKTICKS. NO EXPLANATIONS.",
		string(sampleData),
		targetFormat,
	)
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
