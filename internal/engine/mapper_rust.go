//go:build cgo
// +build cgo

package engine

import (
	"bufio"
	"bytes"
	"io"
)

// MapStream reads JSONL from in, applies schema via the Rust performance core, and sends each mapped row to sink.
func MapStream(in io.Reader, schema *Schema, sink RowSink) (rowsWritten int64, err error) {
	if err = SetRustSchema(schema); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(in)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		out, err := RustMapRow(string(line))
		if err != nil {
			return rowsWritten, err
		}
		if err = sink.WriteRow([]byte(out)); err != nil {
			return rowsWritten, err
		}
		rowsWritten++
	}
	if err = scanner.Err(); err != nil {
		return rowsWritten, err
	}
	return rowsWritten, nil
}
