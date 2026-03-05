//go:build !cgo
// +build !cgo

package engine

import (
	"bufio"
	"bytes"
	"io"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// MapStream reads JSONL from in, applies schema rules line-by-line, and sends each mapped row to sink.
// Pure Go implementation (used when building without cgo).
func MapStream(in io.Reader, schema *Schema, sink RowSink) (rowsWritten int64, err error) {
	scanner := bufio.NewScanner(in)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		row := "{}"
		for _, rule := range schema.Rules {
			res := gjson.GetBytes(line, rule.SourcePath)
			if !res.Exists() {
				continue
			}
			if res.Type == gjson.JSON {
				row, err = sjson.SetRaw(row, rule.TargetField, res.Raw)
			} else {
				row, err = sjson.Set(row, rule.TargetField, res.Value())
			}
			if err != nil {
				return rowsWritten, err
			}
		}
		if err = sink.WriteRow([]byte(row)); err != nil {
			return rowsWritten, err
		}
		rowsWritten++
	}
	if err = scanner.Err(); err != nil {
		return rowsWritten, err
	}
	return rowsWritten, nil
}
