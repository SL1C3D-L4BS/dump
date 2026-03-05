package engine

import "io"

// RowSink receives mapped rows (JSON bytes). Used for both JSONL output and Parquet.
type RowSink interface {
	WriteRow(row []byte) error
}

// JSONLWriter is a RowSink that writes each row as a line (row + newline).
type JSONLWriter struct{ W io.Writer }

// WriteRow implements RowSink.
func (w JSONLWriter) WriteRow(row []byte) error {
	_, err := w.W.Write(append(row, '\n'))
	return err
}
