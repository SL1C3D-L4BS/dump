// Package engine: streaming FHIR reader (Bundle or single resource) as RowReader and io.Reader.

package engine

import (
	"encoding/json"
	"io"
)

// FHIRReader reads FHIR JSON (Bundle or standalone resource) and yields one resource per Next().
// Implements RowReader. Also implements io.Reader by streaming JSONL (one line per resource) for MapStream.
type FHIRReader struct {
	decoder *json.Decoder
	state   fhirReaderState
	buf     []byte
}

type fhirReaderState int

const (
	fhirStateInit fhirReaderState = iota
	fhirStateScanRoot
	fhirStateInEntryArray
	fhirStateDone
)

// NewFHIRReader creates a reader over r (FHIR JSON). Bundle entry.resource items or a single resource are yielded.
func NewFHIRReader(r io.Reader) *FHIRReader {
	return &FHIRReader{
		decoder: json.NewDecoder(r),
		state:   fhirStateInit,
		buf:     nil,
	}
}

// Next implements RowReader. Yields each entry.resource from a Bundle, or the single resource once.
func (f *FHIRReader) Next() (map[string]interface{}, error) {
	if f.state == fhirStateDone {
		return nil, io.EOF
	}
	if f.state == fhirStateInit {
		tok, err := f.decoder.Token()
		if err != nil {
			return nil, err
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '{' {
			return nil, io.EOF
		}
		f.state = fhirStateScanRoot
	}
	// Scan root object for resourceType and entry
	for f.state == fhirStateScanRoot {
		tok, err := f.decoder.Token()
		if err != nil {
			return nil, err
		}
		if delim, ok := tok.(json.Delim); ok && delim == '}' {
			f.state = fhirStateDone
			return nil, io.EOF
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		switch key {
		case "resourceType":
			next, err := f.decoder.Token()
			if err != nil {
				return nil, err
			}
			if rt, ok := next.(string); ok && rt == "Bundle" {
				continue
			}
			// Standalone resource: value is resourceType (e.g. "Patient"); decode rest of object
			if rt, ok := next.(string); ok {
				rest := map[string]interface{}{"resourceType": rt}
				for f.decoder.More() {
					k, err := f.decoder.Token()
					if err != nil {
						return nil, err
					}
					keyStr, _ := k.(string)
					var v interface{}
					if err := f.decoder.Decode(&v); err != nil {
						return nil, err
					}
					rest[keyStr] = v
				}
				f.decoder.Token() // consume }
				f.state = fhirStateDone
				return rest, nil
			}
		case "entry":
			delim, err := f.decoder.Token()
			if err != nil {
				return nil, err
			}
			if d, ok := delim.(json.Delim); ok && d == '[' {
				f.state = fhirStateInEntryArray
				break
			}
			// value wasn't array, skip it
			if delim == json.Delim('{') {
				skipToMatching(f.decoder, json.Delim('}'))
			} else if delim == json.Delim('[') {
				skipToMatching(f.decoder, json.Delim(']'))
			}
		default:
			// Skip this key's value
			next, err := f.decoder.Token()
			if err != nil {
				return nil, err
			}
			if d, ok := next.(json.Delim); ok {
				if d == '{' {
					skipToMatching(f.decoder, json.Delim('}'))
				} else if d == '[' {
					skipToMatching(f.decoder, json.Delim(']'))
				}
			}
		}
	}
	if f.state == fhirStateInEntryArray {
		if !f.decoder.More() {
			f.decoder.Token() // ]
			f.state = fhirStateDone
			return nil, io.EOF
		}
		var entry struct {
			Resource map[string]interface{} `json:"resource"`
		}
		if err := f.decoder.Decode(&entry); err != nil {
			return nil, err
		}
		return entry.Resource, nil
	}
	return nil, io.EOF
}

func skipToMatching(dec *json.Decoder, end json.Delim) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		if d, ok := tok.(json.Delim); ok {
			if d == end {
				return
			}
			if d == '{' {
				skipToMatching(dec, json.Delim('}'))
			} else if d == '[' {
				skipToMatching(dec, json.Delim(']'))
			}
		}
	}
}

// Read implements io.Reader. Streams JSONL (one line per resource) for MapStream.
func (f *FHIRReader) Read(p []byte) (n int, err error) {
	for len(f.buf) < len(p) {
		row, err := f.Next()
		if err == io.EOF {
			n = copy(p, f.buf)
			f.buf = f.buf[n:]
			if len(f.buf) > 0 {
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
		f.buf = append(f.buf, line...)
		f.buf = append(f.buf, '\n')
	}
	n = copy(p, f.buf)
	f.buf = f.buf[n:]
	return n, nil
}
