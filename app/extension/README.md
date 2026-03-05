# DUMP Protobuf Decoder — Chrome DevTools Extension

Decode Protobuf and gRPC-Web binary payloads in the browser using DUMP's heuristic decoder (Go → WASM).

## Build

From the repo root:

```bash
make extension
```

This runs `make wasm` (builds `dump.wasm`) and `make wasm-exec` (copies `wasm_exec.js` from your Go installation into `lib/`).

## Load in Chrome

1. Open `chrome://extensions/`
2. Enable **Developer mode**
3. Click **Load unpacked**
4. Select the `app/extension` directory

## Use

1. Open DevTools (F12) on any page
2. Go to the **DUMP Proto** tab
3. WASM loads automatically. Optionally click **Decode last Proto** to decode the most recently intercepted response with a Protobuf-like Content-Type (`application/grpc-web+proto`, `application/x-protobuf`, etc.).
4. Use the search box to filter the JSON tree by key or value.

## Requirements

- Go 1.21+ (for building WASM)
- Chrome (Manifest V3)
