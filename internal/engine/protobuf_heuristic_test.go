package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

// appendFieldVarint encodes field num with VarintType and value v.
func appendFieldVarint(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	b = protowire.AppendVarint(b, v)
	return b
}

// appendFieldBytes encodes field num with BytesType and length-delimited payload.
func appendFieldBytes(b []byte, num protowire.Number, payload []byte) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	b = protowire.AppendBytes(b, payload)
	return b
}

func appendFieldFixed32(b []byte, num protowire.Number, v uint32) []byte {
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	b = protowire.AppendFixed32(b, v)
	return b
}

func appendFieldFixed64(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	b = protowire.AppendFixed64(b, v)
	return b
}

func TestDecodeProtobuf_Empty(t *testing.T) {
	got, err := DecodeProtobuf(nil)
	if err != nil {
		t.Fatalf("DecodeProtobuf(nil): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestDecodeProtobuf_Varint(t *testing.T) {
	var b []byte
	b = appendFieldVarint(b, 1, 42)
	b = appendFieldVarint(b, 2, 0)
	b = appendFieldVarint(b, 3, 1)

	got, err := DecodeProtobuf(b)
	if err != nil {
		t.Fatalf("DecodeProtobuf: %v", err)
	}
	if got["1"] != uint64(42) {
		t.Errorf("field 1: got %v, want 42", got["1"])
	}
	if got["2"] != uint64(0) {
		t.Errorf("field 2: got %v, want 0", got["2"])
	}
	if got["3"] != uint64(1) {
		t.Errorf("field 3: got %v, want 1", got["3"])
	}
}

func TestDecodeProtobuf_BytesAsString(t *testing.T) {
	var b []byte
	b = appendFieldBytes(b, 1, []byte("hello"))

	got, err := DecodeProtobuf(b)
	if err != nil {
		t.Fatalf("DecodeProtobuf: %v", err)
	}
	if got["1"] != "hello" {
		t.Errorf("field 1: got %q, want \"hello\"", got["1"])
	}
}

func TestDecodeProtobuf_NestedMessage(t *testing.T) {
	var nested []byte
	nested = appendFieldVarint(nested, 1, 99)
	var b []byte
	b = appendFieldBytes(b, 1, nested)

	got, err := DecodeProtobuf(b)
	if err != nil {
		t.Fatalf("DecodeProtobuf: %v", err)
	}
	inner, ok := got["1"].(map[string]interface{})
	if !ok {
		t.Fatalf("field 1: expected nested map, got %T %v", got["1"], got["1"])
	}
	if inner["1"] != uint64(99) {
		t.Errorf("nested field 1: got %v, want 99", inner["1"])
	}
}

func TestDecodeProtobuf_RepeatedVarint(t *testing.T) {
	var b []byte
	b = appendFieldVarint(b, 1, 10)
	b = appendFieldVarint(b, 1, 20)
	b = appendFieldVarint(b, 1, 30)

	got, err := DecodeProtobuf(b)
	if err != nil {
		t.Fatalf("DecodeProtobuf: %v", err)
	}
	slice, ok := got["1"].([]interface{})
	if !ok {
		t.Fatalf("field 1: expected slice, got %T %v", got["1"], got["1"])
	}
	want := []interface{}{uint64(10), uint64(20), uint64(30)}
	if !reflect.DeepEqual(slice, want) {
		t.Errorf("field 1: got %v, want %v", slice, want)
	}
}

func TestDecodeProtobuf_Fixed32Fixed64(t *testing.T) {
	var b []byte
	b = appendFieldFixed32(b, 1, 0x12345678)
	b = appendFieldFixed64(b, 2, 0x123456789abcdef0)

	got, err := DecodeProtobuf(b)
	if err != nil {
		t.Fatalf("DecodeProtobuf: %v", err)
	}
	if got["1"] != uint32(0x12345678) {
		t.Errorf("field 1 (fixed32): got %v", got["1"])
	}
	if got["2"] != uint64(0x123456789abcdef0) {
		t.Errorf("field 2 (fixed64): got %v", got["2"])
	}
}

func TestDecodeProtobuf_InvalidTag(t *testing.T) {
	_, err := DecodeProtobuf([]byte{0xff, 0xff, 0xff, 0xff, 0xff})
	if err == nil {
		t.Error("expected error for invalid/malformed tag")
	}
}

func TestDecodeProtobuf_RoundTripJSON(t *testing.T) {
	var b []byte
	b = appendFieldVarint(b, 1, 42)
	b = appendFieldBytes(b, 2, []byte("world"))
	var nested []byte
	nested = appendFieldVarint(nested, 1, 7)
	b = appendFieldBytes(b, 3, nested)

	got, err := DecodeProtobuf(b)
	if err != nil {
		t.Fatalf("DecodeProtobuf: %v", err)
	}
	// Marshal to JSON and unmarshal to verify structure is serializable
	js, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(js, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	// Numbers may become float64 in JSON; check keys exist and types
	if decoded["1"] == nil {
		t.Error("key 1 missing after JSON round trip")
	}
	if decoded["2"] != "world" {
		t.Errorf("key 2: got %v", decoded["2"])
	}
	inner, _ := decoded["3"].(map[string]interface{})
	if inner == nil || inner["1"] == nil {
		t.Errorf("nested key 3.1 missing: %v", decoded["3"])
	}
}
