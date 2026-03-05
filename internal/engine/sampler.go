// Package engine: universal sampler that normalizes any supported format into
// a JSON array of maps for the LLM.

package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
)

// ExtractSample reads from r using the appropriate reader for format, collects
// up to limit rows as []map[string]interface{}, and returns a compact JSON string.
// dialectPath is used when format is "edi" (optional; uses default HL7-style dialect if empty).
// Unknown or unsupported format returns a best-effort single-row sample or error.
func ExtractSample(r io.Reader, format string, dialectPath string, limit int) (string, error) {
	if limit <= 0 {
		limit = 10
	}
	var src io.Reader
	switch format {
	case "jsonl":
		src = r
	case "csv":
		src = NewCSVReader(r)
	case "xml":
		src = NewXMLReader(r, "Record")
	case "edi":
		dialect, err := dialectForEDI(dialectPath)
		if err != nil {
			return "", err
		}
		src = NewEDIReader(r, dialect)
	default:
		// unknown: treat as jsonl and try to read lines
		src = r
	}

	rows, err := readRowsAsMaps(src, limit)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "[]", nil
	}
	out, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// dialectForEDI returns a dialect for EDI sampling. Uses dialectPath if non-empty;
// otherwise returns a minimal default (MSH, | delimiter) so sampling still works.
func dialectForEDI(dialectPath string) (*dialects.Dialect, error) {
	if dialectPath != "" {
		return dialects.LoadDialect(dialectPath)
	}
	d := &dialects.Dialect{
		MessageStartSegment: "MSH",
		Segments:            nil,
	}
	d.Delimiters.Segment = "\n"
	d.Delimiters.Field = "|"
	d.Delimiters.Component = "^"
	return d, nil
}

// readRowsAsMaps reads JSONL from src (line-by-line), unmarshals each line into
// map[string]interface{}, and returns up to limit rows. Malformed lines are skipped.
func readRowsAsMaps(src io.Reader, limit int) ([]map[string]interface{}, error) {
	scanner := bufio.NewScanner(src)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)
	var rows []map[string]interface{}
	for scanner.Scan() && len(rows) < limit {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		rows = append(rows, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}
