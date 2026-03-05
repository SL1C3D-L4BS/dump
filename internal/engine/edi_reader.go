// Package engine: EDI/HL7 streaming reader that produces JSONL for MapStream.
// Uses a dialect YAML to map segment IDs (e.g. PID, OBX) to human-readable
// field names and outputs one JSON object per message (group of segments).

package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
)

// EDIReader streams EDI/HL7 input and emits one JSONL row per message.
// Segments are parsed with configurable delimiters; the dialect maps
// segment IDs to field names for structured output.
type EDIReader struct {
	scanner     *bufio.Scanner
	dialect     *dialects.Dialect
	buf         []byte
	pendingLine string // buffered segment when we hit next message start
}

// NewEDIReader creates a reader that uses the given dialect to parse segments
// and emit one JSON object per message (message boundary = message_start_segment).
func NewEDIReader(r io.Reader, dialect *dialects.Dialect) *EDIReader {
	sc := bufio.NewScanner(r)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxCapacity)
	return &EDIReader{scanner: sc, dialect: dialect}
}

// Read implements io.Reader. Streams JSONL (one line per EDI message).
func (e *EDIReader) Read(p []byte) (n int, err error) {
	for len(e.buf) < len(p) {
		row, err := e.nextMessage()
		if err == io.EOF {
			n = copy(p, e.buf)
			e.buf = e.buf[n:]
			if len(e.buf) > 0 {
				return n, nil
			}
			return n, io.EOF
		}
		if err != nil {
			return 0, err
		}
		if row == nil {
			continue
		}
		line, err := json.Marshal(row)
		if err != nil {
			return 0, err
		}
		e.buf = append(e.buf, line...)
		e.buf = append(e.buf, '\n')
	}
	n = copy(p, e.buf)
	e.buf = e.buf[n:]
	return n, nil
}

// nextMessage reads segments until the next message start (or EOF), merges
// them into a single map with SegmentId.FieldName (or SegmentId_N for repeated segments) keys.
func (e *EDIReader) nextMessage() (map[string]interface{}, error) {
	out := make(map[string]interface{})
	startSeg := e.dialect.MessageStartSegment
	fieldSep := e.dialect.Delimiters.Field
	if fieldSep == "" {
		fieldSep = "|"
	}
	first := true
	segmentIndex := make(map[string]int) // segment type -> count for repeated segments
	for {
		var line string
		if e.pendingLine != "" {
			line = e.pendingLine
			e.pendingLine = ""
		} else {
			if !e.scanner.Scan() {
				if err := e.scanner.Err(); err != nil {
					return nil, err
				}
				if first {
					return nil, io.EOF
				}
				if len(out) > 0 {
					return out, nil
				}
				return nil, io.EOF
			}
			line = e.scanner.Text()
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		segID, fields := parseSegment(line, fieldSep)
		if segID == "" {
			continue
		}
		if segID == startSeg && !first && len(out) > 0 {
			e.pendingLine = line
			return out, nil
		}
		first = false
		idx := segmentIndex[segID]
		segmentIndex[segID] = idx + 1
		applySegment(out, e.dialect, segID, fields, idx)
	}
}

// parseSegment splits a segment line by fieldSep. Returns segment ID (first field) and remaining fields.
func parseSegment(line, fieldSep string) (segID string, fields []string) {
	if fieldSep == "|" {
		parts := strings.Split(line, "|")
		if len(parts) < 1 {
			return "", nil
		}
		return strings.TrimSpace(parts[0]), parts[1:]
	}
	parts := strings.Split(line, fieldSep)
	if len(parts) < 1 {
		return "", nil
	}
	return strings.TrimSpace(parts[0]), parts[1:]
}

// applySegment merges parsed segment fields into out using the dialect's field names.
// Writes nested maps (e.g. out["MSH"]["DateTime"]) so gjson paths like MSH.DateTime work.
// When segmentIndex > 0, uses Segment_N as key so repeated segments (e.g. OBX) don't overwrite.
func applySegment(out map[string]interface{}, d *dialects.Dialect, segID string, values []string, segmentIndex int) {
	prefix := segID
	if segmentIndex > 0 {
		prefix = fmt.Sprintf("%s_%d", segID, segmentIndex)
	}
	segMap, _ := out[prefix].(map[string]interface{})
	if segMap == nil {
		segMap = make(map[string]interface{})
		out[prefix] = segMap
	}
	names := d.Segments[segID]
	for i, v := range values {
		name := fieldNameAt(names, i)
		segMap[name] = v
	}
}

func fieldNameAt(names []string, i int) string {
	// names[0] = segment type label, names[1]+ = field labels for values[0], values[1], ...
	if names != nil && i+1 < len(names) && names[i+1] != "" {
		return names[i+1]
	}
	return "Field" + fmt.Sprint(i+1)
}