// Package engine: CSV (and future XML) readers that produce JSONL for MapStream.
// Headers map to keys so path-based mapping (e.g. user.name) works when headers use dot notation or flat names.

package engine

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strings"
)

// CSVReader implements io.Reader by reading CSV and streaming rows as JSONL.
// First row is treated as headers; each subsequent row becomes a JSON object with header[i] -> value.
// Headers are normalized (trimmed) so source_path in schema can use the header name as key (e.g. "Email" -> "Email").
type CSVReader struct {
	reader *csv.Reader
	headers []string
	buf    []byte
	first  bool
}

// NewCSVReader wraps r and returns a reader that outputs one JSON object per CSV data row.
func NewCSVReader(r io.Reader) *CSVReader {
	c := csv.NewReader(r)
	c.TrimLeadingSpace = true
	return &CSVReader{reader: c, first: true}
}

// Read implements io.Reader. Streams JSONL (one line per CSV row).
func (c *CSVReader) Read(p []byte) (n int, err error) {
	for len(c.buf) < len(p) {
		row, err := c.reader.Read()
		if err == io.EOF {
			n = copy(p, c.buf)
			c.buf = c.buf[n:]
			if len(c.buf) > 0 {
				return n, nil
			}
			return n, io.EOF
		}
		if err != nil {
			return 0, err
		}
		if c.first {
			c.first = false
			c.headers = make([]string, len(row))
			for i, h := range row {
				c.headers[i] = strings.TrimSpace(h)
			}
			continue
		}
		m := make(map[string]interface{}, len(c.headers))
		for i, h := range c.headers {
			val := ""
			if i < len(row) {
				val = strings.TrimSpace(row[i])
			}
			m[h] = val
		}
		line, _ := json.Marshal(m)
		c.buf = append(c.buf, line...)
		c.buf = append(c.buf, '\n')
	}
	n = copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}
