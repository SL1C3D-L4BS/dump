package engine

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/parquet-go/parquet-go"
)

const parquetRowGroupSize = 10_000

// ParquetWriter writes mapped rows to Parquet using the engine schema.
// It buffers rows and flushes row groups for columnar compression.
type ParquetWriter struct {
	w           *parquet.Writer
	schema      *Schema
	parquetSch  *parquet.Schema
	columnPaths [][]string // columns[i] = path for column index i (e.g. ["id"])
	rowBuf      []parquet.Row
}

// NewParquetWriter creates a Parquet writer that writes to w with columns derived from s.
func NewParquetWriter(w io.Writer, s *Schema) (*ParquetWriter, error) {
	root := parquet.Group{}
	for _, rule := range s.Rules {
		node := yamlTypeToParquetNode(rule.Type)
		root[rule.TargetField] = parquet.Optional(node)
	}
	parquetSch := parquet.NewSchema("row", root)
	pw := &ParquetWriter{
		w:           parquet.NewWriter(w, parquetSch),
		schema:      s,
		parquetSch:  parquetSch,
		columnPaths: parquetSch.Columns(),
		rowBuf:      make([]parquet.Row, 0, parquetRowGroupSize),
	}
	return pw, nil
}

func yamlTypeToParquetNode(typ string) parquet.Node {
	switch strings.ToLower(typ) {
	case "int", "int64", "integer", "long":
		return parquet.Leaf(parquet.Int64Type)
	case "float", "float64", "double", "number":
		return parquet.Leaf(parquet.DoubleType)
	case "float32":
		return parquet.Leaf(parquet.FloatType)
	case "bool", "boolean":
		return parquet.Leaf(parquet.BooleanType)
	default:
		return parquet.String()
	}
}

// WriteRow parses the JSON line and appends one row to the buffer; flushes when buffer is full.
func (pw *ParquetWriter) WriteRow(jsonLine []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(jsonLine, &m); err != nil {
		return err
	}
	row := pw.buildRow(m)
	pw.rowBuf = append(pw.rowBuf, row)
	if len(pw.rowBuf) >= parquetRowGroupSize {
		return pw.Flush()
	}
	return nil
}

func (pw *ParquetWriter) buildRow(m map[string]interface{}) parquet.Row {
	row := make(parquet.Row, 0, len(pw.columnPaths))
	for i, path := range pw.columnPaths {
		fieldName := ""
		if len(path) > 0 {
			fieldName = path[0]
		}
		v, ok := m[fieldName]
		if !ok || v == nil {
			row = append(row, parquet.NullValue().Level(0, 0, i))
			continue
		}
		row = append(row, parquet.ValueOf(v).Level(0, 1, i))
	}
	return row
}

// Flush writes buffered rows as a row group.
func (pw *ParquetWriter) Flush() error {
	if len(pw.rowBuf) == 0 {
		return nil
	}
	_, err := pw.w.WriteRows(pw.rowBuf)
	if err != nil {
		return err
	}
	pw.rowBuf = pw.rowBuf[:0]
	return pw.w.Flush()
}

// Close flushes remaining rows and closes the Parquet writer.
func (pw *ParquetWriter) Close() error {
	if err := pw.Flush(); err != nil {
		return err
	}
	return pw.w.Close()
}
