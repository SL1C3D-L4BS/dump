// Package engine: semantic masking and anonymization of PII in the data pipeline.

package engine

import (
	"regexp"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
)

// Lightweight regex for value-based fallback (email and 16-digit card).
var (
	emailRegex = regexp.MustCompile(`(?i)^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$`)
	digitsOnly = regexp.MustCompile(`\D`)
)

// DetectAndMask returns a synthetic replacement for val when the key or value suggests PII.
// Key is converted to lowercase for heuristics. Nested maps/slices are recursively processed in MaskRow.
func DetectAndMask(key string, val interface{}) interface{} {
	k := strings.ToLower(key)

	// Key-based heuristics
	switch {
	case strings.Contains(k, "email"):
		return gofakeit.Email()
	case strings.Contains(k, "name") && !strings.Contains(k, "username"):
		return gofakeit.Name()
	case strings.Contains(k, "ssn") || strings.Contains(k, "social"):
		return gofakeit.SSN()
	case strings.Contains(k, "phone"):
		return gofakeit.Phone()
	case strings.Contains(k, "card") || strings.Contains(k, "ccnum") || strings.Contains(k, "credit"):
		return gofakeit.CreditCardNumber(nil)
	}

	// Value-based regex fallback (string only)
	if s, ok := val.(string); ok && s != "" {
		if emailRegex.MatchString(s) {
			return gofakeit.Email()
		}
		digits := digitsOnly.ReplaceAllString(s, "")
		if len(digits) >= 16 && len(digits) <= 19 {
			return gofakeit.CreditCardNumber(nil)
		}
	}

	return val
}

// MaskRow returns a new map with all fields passed through DetectAndMask.
// Nested map[string]interface{} and []interface{} are traversed recursively;
// keys are passed as-is for nested keys (path not concatenated).
func MaskRow(row map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(row))
	for key, val := range row {
		out[key] = maskValue(key, val)
	}
	return out
}

func maskValue(key string, val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		return MaskRow(v)
	case []interface{}:
		sl := make([]interface{}, len(v))
		for i, item := range v {
			sl[i] = maskValue(key, item)
		}
		return sl
	default:
		return DetectAndMask(key, val)
	}
}
