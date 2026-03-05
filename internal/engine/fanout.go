// Package engine: multi-writer fan-out for multiplexing a single stream to multiple sinks.

package engine

// MultiSink holds a slice of RowSinks and writes each row to all of them sequentially (V1: stable memory bounds).
type MultiSink struct {
	Sinks []RowSink
}

// WriteRow implements RowSink. Writes the row to every sink in order; returns the first error if any.
func (m *MultiSink) WriteRow(row []byte) error {
	for _, s := range m.Sinks {
		if err := s.WriteRow(row); err != nil {
			return err
		}
	}
	return nil
}
