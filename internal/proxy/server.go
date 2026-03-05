// Package proxy provides the JIT sidecar proxy that translates upstream XML
// (or other formats) into mapped JSONL on the fly using the DUMP engine.

package proxy

import (
	"net/http"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// JITProxy holds the upstream base URL and the loaded mapping schema for live translation.
type JITProxy struct {
	Upstream string
	Schema   *engine.Schema
	XMLBlock string
}

// NewJITProxy loads the schema from path and returns a JITProxy. XMLBlock is the
// repeating XML element name (e.g. "Record") for streaming.
func NewJITProxy(upstream, schemaPath, xmlBlock string) (*JITProxy, error) {
	schema, err := engine.LoadSchema(schemaPath)
	if err != nil {
		return nil, err
	}
	if xmlBlock == "" {
		xmlBlock = "Record"
	}
	return &JITProxy{
		Upstream: strings.TrimSuffix(upstream, "/"),
		Schema:   schema,
		XMLBlock: xmlBlock,
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
