# DUMP build and extension targets

.PHONY: wasm wasm-exec extension

# Build WASM binary for the Chrome extension (requires GOOS=js GOARCH=wasm).
wasm:
	GOOS=js GOARCH=wasm go build -o app/extension/lib/dump.wasm ./cmd/wasm/

# Copy Go's wasm_exec.js into the extension (run after 'wasm' or when Go version changes).
# Go 1.24+ uses lib/wasm/wasm_exec.js; older Go uses misc/wasm/wasm_exec.js.
wasm-exec:
	@mkdir -p app/extension/lib
	@GOROOT=$$(go env GOROOT); \
	if [ -f "$$GOROOT/lib/wasm/wasm_exec.js" ]; then \
	  cp "$$GOROOT/lib/wasm/wasm_exec.js" app/extension/lib/; \
	elif [ -f "$$GOROOT/misc/wasm/wasm_exec.js" ]; then \
	  cp "$$GOROOT/misc/wasm/wasm_exec.js" app/extension/lib/; \
	else \
	  echo "wasm_exec.js not found in GOROOT. Copy it from $$GOROOT (lib/wasm or misc/wasm) to app/extension/lib/"; exit 1; \
	fi

# Build WASM and copy wasm_exec.js for the extension.
extension: wasm wasm-exec
