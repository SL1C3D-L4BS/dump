// Package proxy: live interception pipeline — forward request to upstream,
// stream response body through XML reader and MapStream, write JSONL to client.

package proxy

import (
	"io"
	"net/http"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// handleProxy constructs the upstream request, executes it, and streams the
// response body through the mapping engine without loading it into memory.
func handleProxy(p *JITProxy, w http.ResponseWriter, r *http.Request) {
	targetURL := p.Upstream + r.URL.RequestURI()
	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
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

	// Do not load body into memory: pass resp.Body directly into the XML reader.
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
