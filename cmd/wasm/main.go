//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/SL1C3D-L4BS/dump/internal/protobufdecode"
)

func main() {
	// Expose decodeProtobuf(uint8Array) -> object | null; throws on error.
	js.Global().Set("decodeProtobuf", js.FuncOf(decodeProtobufJS))
	// Keep the Go runtime alive (required for wasm)
	<-make(chan struct{})
}

func decodeProtobufJS(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeObject {
		return js.ValueOf(map[string]interface{}{"error": "expected one argument: Uint8Array"})
	}
	// Copy JS Uint8Array into Go []byte
	length := args[0].Get("length").Int()
	data := make([]byte, length)
	js.CopyBytesToGo(data, args[0])
	out, err := protobufdecode.Decode(data)
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	// Return as JS Object via JSON round-trip so nested maps/slices become plain objects/arrays
	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	var parsed interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		return js.ValueOf(map[string]interface{}{"error": err.Error()})
	}
	return js.ValueOf(parsed)
}
