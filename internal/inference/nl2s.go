package inference

import (
	"encoding/json"
	"fmt"
	"strings"
)

const nl2sSystemPromptFmt = `You are a high-precision data extraction engine. Extract information from the following text into a JSON object that matches this template: %s
Rules:
- Output ONLY valid JSON.
- Use 'null' for missing fields.
- No preamble, no conversational filler, no markdown blocks.
- Strictly follow the keys provided in the template.`

// ExtractToTemplate sends inputText to the local Ollama model with the given template,
// then validates and optionally strict-filters the response to match the template keys.
// Returns the sanitized JSON string or an error.
func ExtractToTemplate(inputText string, templateJSON string, model string, strict bool) (string, error) {
	inf := NewSchemaInferencer("", nil)
	prompt := fmt.Sprintf(nl2sSystemPromptFmt, templateJSON) + "\n\nText to extract from:\n" + inputText
	raw, err := inf.Generate(model, prompt)
	if err != nil {
		return "", err
	}
	// Strip any accidental markdown code fence
	raw = stripJSONFence(raw)
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return "", fmt.Errorf("nl2s: LLM output is not valid JSON: %w", err)
	}
	allowedKeys := templateKeySet(templateJSON)
	if strict && len(allowedKeys) > 0 {
		response = filterKeysStrict(response, allowedKeys, "")
	}
	out, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("nl2s: marshal result: %w", err)
	}
	return string(out), nil
}

// stripJSONFence removes ```json ... ``` or ``` ... ``` wrappers.
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	const jsonBlock = "```json"
	const block = "```"
	if strings.HasPrefix(s, jsonBlock) {
		s = strings.TrimPrefix(s, jsonBlock)
	} else if strings.HasPrefix(s, block) {
		s = strings.TrimPrefix(s, block)
		if rest := strings.TrimPrefix(s, "json"); rest != s {
			s = rest
		}
	}
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, block)
	return strings.TrimSpace(s)
}

// templateKeySet returns the set of allowed key paths from templateJSON (e.g. "name", "address.city").
func templateKeySet(templateJSON string) map[string]bool {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(templateJSON), &m); err != nil {
		return nil
	}
	set := make(map[string]bool)
	collectKeys("", m, set)
	return set
}

func collectKeys(prefix string, m map[string]interface{}, set map[string]bool) {
	for k, v := range m {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		set[path] = true
		if nested, ok := v.(map[string]interface{}); ok {
			collectKeys(path, nested, set)
		}
	}
}

// filterKeysStrict keeps only keys in allowed (path set). Removes keys not in template.
func filterKeysStrict(response map[string]interface{}, allowed map[string]bool, pathPrefix string) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range response {
		path := k
		if pathPrefix != "" {
			path = pathPrefix + "." + k
		}
		if !allowed[path] {
			continue
		}
		if nested, ok := v.(map[string]interface{}); ok {
			out[k] = filterKeysStrict(nested, allowed, path)
		} else {
			out[k] = v
		}
	}
	return out
}
