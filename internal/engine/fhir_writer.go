// Package engine: streaming FHIR Bundle writer (RowSink that wraps rows in a Bundle envelope).

package engine

import (
	"io"
)

// FHIRWriter writes mapped rows as FHIR Bundle entries. Implements RowSink.
// On first use writes the opening envelope; on Close() writes the closing ]} and flushes.
type FHIRWriter struct {
	w       io.Writer
	started bool
}

// NewFHIRWriter creates a sink that streams {"resourceType":"Bundle","type":"collection","entry":[ {...}, ... ]}.
func NewFHIRWriter(w io.Writer) *FHIRWriter {
	return &FHIRWriter{w: w, started: false}
}

// WriteRow implements RowSink. Wraps row in {"resource": <row>} and writes; adds commas between entries.
func (f *FHIRWriter) WriteRow(row []byte) error {
	if !f.started {
		_, err := f.w.Write([]byte(`{"resourceType":"Bundle","type":"collection","entry":[`))
		if err != nil {
			return err
		}
		f.started = true
	} else {
		if _, err := f.w.Write([]byte{','}); err != nil {
			return err
		}
	}
	_, err := f.w.Write(append(append([]byte(`{"resource":`), row...), '}'))
	return err
}

// Close writes the closing ]} and flushes. Idempotent.
func (f *FHIRWriter) Close() error {
	if !f.started {
		_, err := f.w.Write([]byte(`{"resourceType":"Bundle","type":"collection","entry":[]}`))
		return err
	}
	_, err := f.w.Write([]byte("]}"))
	if err != nil {
		return err
	}
	if flusher, ok := f.w.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
	return nil
}
