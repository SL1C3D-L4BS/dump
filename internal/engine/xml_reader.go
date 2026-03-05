// Package engine: XML streaming reader that produces JSONL for MapStream.
// Identifies repeating blocks (e.g. <Record>...</Record>) and flattens nested
// elements within each block into a JSON-like map so path-based mapping
// (e.g. Record.Customer.Name) works with the existing mapper.

package engine

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"strings"
)

// XMLReader streams XML and emits one JSONL row per repeating block.
// Uses xml.Decoder to stream; nested elements within each block are
// represented as a nested map[string]interface{} for path-based mapping.
type XMLReader struct {
	decoder   *xml.Decoder
	blockName string // e.g. "Record" — the repeating wrapper element
	buf       []byte
}

// NewXMLReader creates a reader that emits one JSON object per occurrence of blockName.
// If blockName is empty, the first repeated element at the root level is used.
func NewXMLReader(r io.Reader, blockName string) *XMLReader {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) { return input, nil }
	return &XMLReader{decoder: dec, blockName: blockName}
}

// Read implements io.Reader. Streams JSONL (one line per repeating block).
func (x *XMLReader) Read(p []byte) (n int, err error) {
	for len(x.buf) < len(p) {
		tok, err := x.decoder.Token()
		if err == io.EOF {
			n = copy(p, x.buf)
			x.buf = x.buf[n:]
			if len(x.buf) > 0 {
				return n, nil
			}
			return n, io.EOF
		}
		if err != nil {
			return 0, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		blockTag := x.blockName
		if blockTag == "" {
			blockTag = start.Name.Local
		}
		if start.Name.Local != blockTag {
			continue
		}

		row := x.decodeBlockElement(&start)
		// Wrap in the block name so paths like Record.Customer.Name work.
		var wrapped map[string]interface{}
		if rowMap, ok := row.(map[string]interface{}); ok {
			wrapped = map[string]interface{}{blockTag: rowMap}
		} else {
			wrapped = map[string]interface{}{blockTag: row}
		}
		line, err := json.Marshal(wrapped)
		if err != nil {
			return 0, err
		}
		x.buf = append(x.buf, line...)
		x.buf = append(x.buf, '\n')
	}

	n = copy(p, x.buf)
	x.buf = x.buf[n:]
	return n, nil
}

// decodeBlockElement reads from the decoder (current element's children) until
// the matching EndElement. Returns either map[string]interface{} or string for leaf text.
func (x *XMLReader) decodeBlockElement(start *xml.StartElement) interface{} {
	out := make(map[string]interface{})
	var textParts []string
	for {
		tok, err := x.decoder.Token()
		if err != nil {
			return collapseOut(out, textParts)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name == start.Name {
				return collapseOut(out, textParts)
			}
		case xml.StartElement:
			child := x.decodeBlockElement(&t)
			name := t.Name.Local
			if existing, ok := out[name]; ok {
				var slice []interface{}
				switch v := existing.(type) {
				case []interface{}:
					slice = append(v, child)
				default:
					slice = []interface{}{v, child}
				}
				out[name] = slice
			} else {
				out[name] = child
			}
		case xml.CharData:
			s := strings.TrimSpace(string(t))
			if s != "" {
				textParts = append(textParts, s)
			}
		}
	}
}

func collapseOut(out map[string]interface{}, textParts []string) interface{} {
	if len(out) == 0 {
		if len(textParts) == 0 {
			return ""
		}
		return strings.Join(textParts, " ")
	}
	if len(textParts) > 0 {
		out["_text"] = strings.Join(textParts, " ")
	}
	return out
}
