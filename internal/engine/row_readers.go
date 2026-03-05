package engine

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// JSONLRowReader implements RowReader by reading a JSONL file (one JSON object per line).
type JSONLRowReader struct {
	f       *os.File
	scanner *bufio.Scanner
}

// NewJSONLRowReader opens the file at path and returns a RowReader.
func NewJSONLRowReader(path string) (*JSONLRowReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open jsonl: %w", err)
	}
	scanner := bufio.NewScanner(f)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)
	return &JSONLRowReader{f: f, scanner: scanner}, nil
}

// Next returns the next line as a map. Returns (nil, io.EOF) when done.
func (r *JSONLRowReader) Next() (map[string]interface{}, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	line := strings.TrimSpace(r.scanner.Text())
	if line == "" {
		return r.Next()
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return nil, fmt.Errorf("jsonl line: %w", err)
	}
	return m, nil
}

// Close closes the file.
func (r *JSONLRowReader) Close() error {
	if r.f != nil {
		return r.f.Close()
	}
	return nil
}

// CSVRowReader implements RowReader by reading a CSV file (first row = headers).
type CSVRowReader struct {
	f       *os.File
	reader  *csv.Reader
	headers []string
	first   bool
}

// NewCSVRowReader opens the file at path and returns a RowReader.
func NewCSVRowReader(path string) (*CSVRowReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	return &CSVRowReader{f: f, reader: reader, first: true}, nil
}

// Next returns the next row as a map keyed by headers. Returns (nil, io.EOF) when done.
func (r *CSVRowReader) Next() (map[string]interface{}, error) {
	row, err := r.reader.Read()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}
	if r.first {
		r.first = false
		r.headers = make([]string, len(row))
		for i, h := range row {
			r.headers[i] = strings.TrimSpace(h)
		}
		return r.Next()
	}
	m := make(map[string]interface{}, len(r.headers))
	for i, h := range r.headers {
		val := ""
		if i < len(row) {
			val = strings.TrimSpace(row[i])
		}
		m[h] = val
	}
	return m, nil
}

// Close closes the file.
func (r *CSVRowReader) Close() error {
	if r.f != nil {
		return r.f.Close()
	}
	return nil
}

// DiffSourceKind is the detected type of a diff source (file format or sql).
type DiffSourceKind string

const (
	DiffSourceExcel DiffSourceKind = "excel"
	DiffSourceJSON  DiffSourceKind = "json"
	DiffSourceCSV   DiffSourceKind = "csv"
	DiffSourceSQL   DiffSourceKind = "sql"
)

// DetectDiffSource returns the kind of source (excel, json, csv, sql) from path or connection string.
func DetectDiffSource(source string) DiffSourceKind {
	s := strings.TrimSpace(source)
	if strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://") ||
		strings.HasPrefix(s, "file:") || strings.HasPrefix(s, "sqlite:") {
		return DiffSourceSQL
	}
	ext := strings.ToLower(filepath.Ext(s))
	switch ext {
	case ".xlsx":
		return DiffSourceExcel
	case ".json", ".jsonl":
		return DiffSourceJSON
	case ".csv":
		return DiffSourceCSV
	default:
		return DiffSourceJSON
	}
}

// NewRowReaderFromSource creates a RowReader from a path or connection string.
// For Excel, sheet is used (optional); for SQL, query is required.
func NewRowReaderFromSource(source string, sheet string, query string) (RowReader, error) {
	kind := DetectDiffSource(source)
	switch kind {
	case DiffSourceExcel:
		return NewExcelReader(source, sheet)
	case DiffSourceJSON:
		return NewJSONLRowReader(source)
	case DiffSourceCSV:
		return NewCSVRowReader(source)
	case DiffSourceSQL:
		if query == "" {
			return nil, fmt.Errorf("SQL source requires a query (e.g. --s1-query \"SELECT * FROM t\")")
		}
		return NewSQLRowReader(source, query)
	default:
		return NewJSONLRowReader(source)
	}
}
