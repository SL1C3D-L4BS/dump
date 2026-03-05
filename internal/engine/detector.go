// Package engine: heuristic format detection for zero-knowledge schema discovery.

package engine

import (
	"bytes"
	"strings"
)

// DetectFormat analyzes a byte signature (e.g. peek of first 1024 bytes) to guess the format.
// Returns one of: "xml", "jsonl", "edi", "csv", "unknown".
func DetectFormat(peek []byte) string {
	trimmed := bytes.TrimLeft(peek, " \t\r\n")
	if len(trimmed) == 0 {
		return "unknown"
	}
	first := trimmed[0]
	s := string(peek)

	// Starts with { or [ -> jsonl
	if first == '{' || first == '[' {
		return "jsonl"
	}

	// Starts with or contains < -> xml
	if first == '<' || bytes.Contains(trimmed, []byte("<")) {
		return "xml"
	}

	// Contains MSH (HL7) or ISA* (X12) -> edi
	if strings.Contains(s, "MSH") || strings.Contains(s, "ISA*") {
		return "edi"
	}

	// First line: content up to newline
	firstLine := s
	if idx := bytes.IndexByte(peek, '\n'); idx >= 0 {
		firstLine = string(peek[:idx])
	}
	if idx := bytes.IndexByte(peek, '\r'); idx >= 0 && (idx < bytes.IndexByte(peek, '\n') || bytes.IndexByte(peek, '\n') < 0) {
		firstLine = string(peek[:idx])
	}
	firstLine = strings.TrimSpace(firstLine)

	// First line contains , or | without XML/JSON markers -> csv
	if firstLine != "" && (strings.Contains(firstLine, ",") || strings.Contains(firstLine, "|")) {
		return "csv"
	}

	return "unknown"
}
