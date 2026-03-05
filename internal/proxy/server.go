// Package proxy provides the JIT sidecar proxy that translates upstream XML/EDI
// into mapped JSON/FHIR on the fly using the DUMP engine.

package proxy

import (
	"net/http"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// JITProxy holds the upstream base URL and the loaded mapping schema for live translation.
// When Virtualize is true, the proxy performs bi-directional JSON↔EDI virtualization:
// - Inbound: JSON request bodies are up-converted to X12 EDI before calling upstream.
// - Outbound: X12 EDI responses are down-converted to FHIR/JSON for the caller.
type JITProxy struct {
	Upstream   string
	Schema     *engine.Schema
	XMLBlock   string
	Virtualize bool
}

// NewJITProxy loads the schema from path and returns a JITProxy. XMLBlock is the
// repeating XML element name (e.g. "Record") for streaming when not virtualizing.
// When virtualize is true, XMLBlock is ignored and the proxy uses X12/FHIR pipelines.
func NewJITProxy(upstream, schemaPath, xmlBlock string, virtualize bool) (*JITProxy, error) {
	schema, err := engine.LoadSchema(schemaPath)
	if err != nil {
		return nil, err
	}
	if xmlBlock == "" {
		xmlBlock = "Record"
	}
	return &JITProxy{
		Upstream:   strings.TrimSuffix(upstream, "/"),
		Schema:     schema,
		XMLBlock:   xmlBlock,
		Virtualize: virtualize,
	}, nil
}

// HandleProxy is the HTTP handler that forwards the request to upstream and streams
// the response through the mapping engine. See handler.go for the pipeline logic.
func (p *JITProxy) HandleProxy(w http.ResponseWriter, r *http.Request) {
	handleProxy(p, w, r)
}

// ServeHTTP makes JITProxy implement http.Handler so it can be used with ListenAndServe.
func (p *JITProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.HandleProxy(w, r)
}
