package engine

import (
	"encoding/binary"
	"testing"
)

func TestDecodeBinary(t *testing.T) {
	// 4 bytes: uint8(42), int16_be(1000), bool(1)
	buf := make([]byte, 4)
	buf[0] = 42
	binary.BigEndian.PutUint16(buf[1:3], 1000)
	buf[3] = 0x80 // bit 0 of last byte = 1

	specs := []FieldSpec{
		{Name: "id", BitOffset: 0, BitLength: 8, Type: "uint8"},
		{Name: "value", BitOffset: 8, BitLength: 16, Type: "int16_be"},
		{Name: "ok", BitOffset: 24, BitLength: 1, Type: "bool"},
	}
	out, err := DecodeBinary(buf, specs)
	if err != nil {
		t.Fatal(err)
	}
	if out["id"] != uint8(42) {
		t.Errorf("id: got %v", out["id"])
	}
	if out["value"] != int16(1000) {
		t.Errorf("value: got %v", out["value"])
	}
	if out["ok"] != true {
		t.Errorf("ok: got %v", out["ok"])
	}
}

func TestDecodeBinary_uint_bits(t *testing.T) {
	// 2 bytes: 12-bit value 0x0AB at offset 0
	buf := []byte{0x0A, 0xB0}
	specs := []FieldSpec{
		{Name: "x", BitOffset: 0, BitLength: 12, Type: "uint_bits"},
	}
	out, err := DecodeBinary(buf, specs)
	if err != nil {
		t.Fatal(err)
	}
	if out["x"] != uint64(0x0AB) {
		t.Errorf("x: got %v want 0x0AB", out["x"])
	}
}
