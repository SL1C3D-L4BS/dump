// Package engine: universal sampler that normalizes any supported format into
// a JSON array of maps for the LLM.

package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
)

// ExtractRawEDILines reads up to maxLines raw segment lines from r (for Z-segment detection and LLM input).
func ExtractRawEDILines(r io.Reader, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		maxLines = 500
	}
	scanner := bufio.NewScanner(r)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)
	var lines []string
	for scanner.Scan() && len(lines) < maxLines {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// SegmentIDsFromRawEDILines returns the set of segment IDs found in raw EDI lines.
// Detects HL7 (MSH|, |) vs X12 (ISA*, ~, *) from content and parses segment ID accordingly.
func SegmentIDsFromRawEDILines(lines []string) map[string]struct{} {
	seen := make(map[string]struct{})
	if len(lines) == 0 {
		return seen
	}
	isX12 := strings.Contains(lines[0], "ISA*") || strings.Contains(lines[0], "~")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var segID string
		if isX12 {
			if idx := strings.Index(line, "*"); idx >= 0 {
				segID = strings.TrimSpace(line[:idx])
			} else {
				segID = strings.TrimSpace(line)
			}
		} else {
			if idx := strings.Index(line, "|"); idx >= 0 {
				segID = strings.TrimSpace(line[:idx])
			} else {
				segID = strings.TrimSpace(line)
			}
		}
		if len(segID) > 0 {
			seen[segID] = struct{}{}
		}
	}
	return seen
}

// ExtractSampleWithDialect is like ExtractSample but uses the provided dialect for EDI when non-nil.
// When format is "edi" and dialect != nil, that dialect is used instead of loading from dialectPath.
func ExtractSampleWithDialect(r io.Reader, format string, dialectPath string, limit int, dialect *dialects.Dialect) (string, error) {
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
		var d *dialects.Dialect
		var err error
		if dialect != nil {
			d = dialect
		} else {
			d, err = dialectForEDI(dialectPath)
			if err != nil {
				return "", err
			}
		}
		src = NewEDIReader(r, d)
	default:
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
