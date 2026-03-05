// Package engine: interception sink that applies semantic masking to each row before writing.

package engine

import (
	"encoding/json"
)

// MaskingSink wraps an underlying RowSink and passes each row through MaskRow before writing.
type MaskingSink struct {
	Underlying RowSink
}

// WriteRow unmarshals row into a map, applies MaskRow, marshals back, and writes to the underlying sink.
func (m *MaskingSink) WriteRow(row []byte) error {
	var kv map[string]interface{}
	if err := json.Unmarshal(row, &kv); err != nil {
		return err
	}
	masked := MaskRow(kv)
	out, err := json.Marshal(masked)
	if err != nil {
		return err
	}
	return m.Underlying.WriteRow(out)
}
