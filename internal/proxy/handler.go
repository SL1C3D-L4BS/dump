// Package proxy: live interception pipeline — forward request to upstream,
// optionally virtualize JSON↔EDI/X12, and stream responses through MapStream.

package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/pkg/healthcare"
)

// handleProxy constructs the upstream request, executes it, and streams the
// response body through the mapping engine without loading it into memory.
// When Virtualize is enabled on the proxy, it performs:
// - Inbound: JSON → MapStream → X12 (up-convert) before sending to upstream.
// - Outbound: X12 from upstream → X12Reader → MapStream → FHIR Bundle (JSON).
func handleProxy(p *JITProxy, w http.ResponseWriter, r *http.Request) {
	targetURL := p.Upstream + r.URL.RequestURI()

	// Default request body just forwards the client body.
	upstreamBody := r.Body

	// Inbound virtualization: JSON → X12 via MapStream + X12 sink.
	if p.Virtualize && r.Body != nil {
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "application/json") || strings.Contains(ct, "+json") {
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body: "+err.Error(), http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()

			ediPayload, err := upConvertJSONToX12(payload, p.Schema)
			if err != nil {
				http.Error(w, "failed to up-convert JSON to X12: "+err.Error(), http.StatusBadGateway)
				return
			}
			upstreamBody = io.NopCloser(bytes.NewReader(ediPayload))
			r.Header.Set("Content-Type", "application/edi-x12")
		}
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, upstreamBody)
	if err != nil {
		http.Error(w, "bad gateway: "+err.Error(), http.StatusBadGateway)
		return
	}
	// Forward selected headers from the client
	for k, v := range r.Header {
		switch strings.ToLower(k) {
		case "host", "connection", "te", "trailer", "transfer-encoding":
			continue
		default:
			req.Header[k] = v
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	if p.Virtualize {
		// Outbound virtualization: EDI/X12 → FHIR Bundle (JSON).
		if err := downConvertX12ToFHIR(w, resp.Body, p.Schema); err != nil {
			// Response already started; best-effort logging only.
			_ = err
		}
		return
	}

	// Legacy mode: XML → JSONL via XMLReader + MapStream.
	xmlReader := engine.NewXMLReader(resp.Body, p.XMLBlock)
	jsonlSink := engine.JSONLWriter{W: w}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)

	_, err = engine.MapStream(xmlReader, p.Schema, jsonlSink)
	if err != nil {
		// Response already started; we cannot send a proper error code.
		// Best effort: log or write nothing (headers/type already sent).
		_ = err
	}
}

// upConvertJSONToX12 runs the JSON body through MapStream using the configured schema
// and then serializes the mapped fields into an X12 837 payload using the standard dialect.
// Convention: target fields in the schema should be of the form SEGID.FieldName,
// where SEGID is an X12 segment ID (e.g. ISA, GS, ST, BHT, NM1, CLM, SE, GE, IEA)
// and FieldName matches the dialect's segment field names.
func upConvertJSONToX12(body []byte, schema *engine.Schema) ([]byte, error) {
	src := bytes.TrimSpace(body)
	if len(src) == 0 {
		return src, nil
	}
	// Ensure MapStream sees at least one JSONL line.
	if !bytes.HasSuffix(src, []byte("\n")) {
		src = append(src, '\n')
	}

	dialect, err := healthcare.LoadStandardDialect("x12_837")
	if err != nil {
		return nil, fmt.Errorf("load x12 dialect: %w", err)
	}

	var buf bytes.Buffer
	sink := newX12Sink(&buf, dialect)

	if _, err := engine.MapStream(bytes.NewReader(src), schema, sink); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downConvertX12ToFHIR streams an X12 837 response body through the X12Reader,
// then maps to FHIR resources and writes a FHIR Bundle JSON to the client.
func downConvertX12ToFHIR(w http.ResponseWriter, body io.Reader, schema *engine.Schema) error {
	dialect, err := healthcare.LoadStandardDialect("x12_837")
	if err != nil {
		http.Error(w, "failed to load X12 dialect: "+err.Error(), http.StatusBadGateway)
		return err
	}

	x12Reader := healthcare.NewX12Reader(body, dialect)
	fhirWriter := healthcare.NewFHIRWriter(w)
	defer fhirWriter.Close()

	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(http.StatusOK)

	_, err = engine.MapStream(x12Reader, schema, fhirWriter)
	return err
}

// x12Sink implements engine.RowSink and serializes mapped rows into an X12 837 payload.
// It expects flat JSON rows where keys are of the form SEGID.FieldName and uses the
// dialect field metadata to order elements within each segment.
type x12Sink struct {
	buf     *bytes.Buffer
	dialect *dialects.Dialect
}

func newX12Sink(buf *bytes.Buffer, d *dialects.Dialect) *x12Sink {
	return &x12Sink{buf: buf, dialect: d}
}

// WriteRow assembles segments for one X12 transaction based on the mapped fields.
func (s *x12Sink) WriteRow(row []byte) error {
	if s == nil || s.buf == nil || s.dialect == nil {
		return fmt.Errorf("x12 sink not initialized")
	}
	var flat map[string]interface{}
	if err := json.Unmarshal(row, &flat); err != nil {
		return err
	}
	if len(flat) == 0 {
		return nil
	}

	// Group fields by segment ID.
	segFields := make(map[string]map[string]string)
	for fullKey, v := range flat {
		parts := strings.SplitN(fullKey, ".", 2)
		if len(parts) != 2 {
			continue
		}
		segID := parts[0]
		fieldName := parts[1]
		m := segFields[segID]
		if m == nil {
			m = make(map[string]string)
			segFields[segID] = m
		}
		m[fieldName] = fmt.Sprint(v)
	}
	if len(segFields) == 0 {
		return nil
	}

	// Preferred segment order for 837; any additional segments are appended sorted.
	preferredOrder := []string{"ISA", "GS", "ST", "BHT", "NM1", "CLM", "SE", "GE", "IEA"}
	seen := make(map[string]bool, len(segFields))

	writeSegment := func(segID string, fields map[string]string) error {
		if len(fields) == 0 {
			return nil
		}
		names := s.dialect.Segments[segID]
		indexByName := make(map[string]int, len(names))
		for i, n := range names {
			if n == "" {
				continue
			}
			indexByName[n] = i
		}
		maxIdx := -1
		for name := range fields {
			if idx, ok := indexByName[name]; ok {
				if idx > maxIdx {
					maxIdx = idx
				}
			}
		}
		if maxIdx < 0 {
			// No known fields for this segment; nothing to write.
			return nil
		}
		elems := make([]string, maxIdx+1)
		for name, val := range fields {
			idx, ok := indexByName[name]
			if !ok || idx < 0 || idx >= len(elems) {
				continue
			}
			elems[idx] = val
		}
		// Trim trailing empty elements.
		end := len(elems)
		for end > 0 && elems[end-1] == "" {
			end--
		}
		if end == 0 {
			return nil
		}
		segTerm := s.dialect.Delimiters.Segment
		if segTerm == "" {
			segTerm = "~"
		}
		elemSep := s.dialect.Delimiters.Field
		if elemSep == "" {
			elemSep = "*"
		}
		var b strings.Builder
		b.WriteString(segID)
		for i := 0; i < end; i++ {
			b.WriteString(elemSep)
			b.WriteString(elems[i])
		}
		b.WriteString(segTerm)
		if _, err := s.buf.WriteString(b.String()); err != nil {
			return err
		}
		return nil
	}

	// Write preferred segments in order.
	for _, segID := range preferredOrder {
		fields := segFields[segID]
		if fields == nil {
			continue
		}
		if err := writeSegment(segID, fields); err != nil {
			return err
		}
		seen[segID] = true
	}

	// Write any remaining segments in lexicographic order.
	var rest []string
	for segID := range segFields {
		if !seen[segID] {
			rest = append(rest, segID)
		}
	}
	sort.Strings(rest)
	for _, segID := range rest {
		if err := writeSegment(segID, segFields[segID]); err != nil {
			return err
		}
	}
	return nil
}
