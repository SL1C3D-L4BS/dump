package engine

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// FieldSpec describes a single bit-level field in a binary payload.
type FieldSpec struct {
	Name      string `yaml:"name" json:"name"`
	BitOffset int    `yaml:"bit_offset" json:"bit_offset"`
	BitLength int    `yaml:"bit_length" json:"bit_length"`
	Type      string `yaml:"type" json:"type"` // uint8, int8, uint16_be, uint16_le, int16_be, int16_le, float32_be, float32_le, bool, uint_bits
}

// BinaryMappingSchema is the YAML root for source: binary mapping (bit-level fields).
type BinaryMappingSchema struct {
	Source string       `yaml:"source" json:"source"` // "binary"
	Fields []FieldSpec  `yaml:"fields" json:"fields"`
}

// DecodeBinary decodes a binary blob into a map using the given field specs.
// Uses encoding/binary for standard types; for arbitrary bit-lengths (uint_bits) a bit-shifter extracts the value.
func DecodeBinary(data []byte, specs []FieldSpec) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(specs))
	bitLen := len(data) * 8
	for _, spec := range specs {
		if spec.BitOffset < 0 || spec.BitLength <= 0 {
			return nil, fmt.Errorf("field %q: invalid bit_offset/bit_length", spec.Name)
		}
		end := spec.BitOffset + spec.BitLength
		if end > bitLen {
			return nil, fmt.Errorf("field %q: bits [%d:%d] exceed data length (%d bits)", spec.Name, spec.BitOffset, end, bitLen)
		}
		val, err := decodeField(data, spec)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", spec.Name, err)
		}
		out[spec.Name] = val
	}
	return out, nil
}

func decodeField(data []byte, spec FieldSpec) (interface{}, error) {
	typ := strings.TrimSpace(strings.ToLower(spec.Type))
	switch typ {
	case "bool":
		if spec.BitLength != 1 {
			return nil, fmt.Errorf("bool must have bit_length 1")
		}
		b := readBit(data, spec.BitOffset)
		return b != 0, nil
	case "uint8":
		if spec.BitLength > 8 {
			return nil, fmt.Errorf("uint8 bit_length must be <= 8")
		}
		v := readBits(data, spec.BitOffset, spec.BitLength)
		return uint8(v), nil
	case "int8":
		if spec.BitLength > 8 {
			return nil, fmt.Errorf("int8 bit_length must be <= 8")
		}
		v := readBitsSigned(data, spec.BitOffset, spec.BitLength)
		return int8(v), nil
	case "uint16_be", "uint16_le":
		if spec.BitLength != 16 {
			return nil, fmt.Errorf("uint16 requires bit_length 16")
		}
		b := readBytes(data, spec.BitOffset, 2)
		if typ == "uint16_be" {
			return binary.BigEndian.Uint16(b), nil
		}
		return binary.LittleEndian.Uint16(b), nil
	case "int16_be", "int16_le":
		if spec.BitLength != 16 {
			return nil, fmt.Errorf("int16 requires bit_length 16")
		}
		b := readBytes(data, spec.BitOffset, 2)
		var u uint16
		if typ == "int16_be" {
			u = binary.BigEndian.Uint16(b)
		} else {
			u = binary.LittleEndian.Uint16(b)
		}
		return int16(u), nil
	case "uint32_be", "uint32_le":
		if spec.BitLength != 32 {
			return nil, fmt.Errorf("uint32 requires bit_length 32")
		}
		b := readBytes(data, spec.BitOffset, 4)
		if typ == "uint32_be" {
			return binary.BigEndian.Uint32(b), nil
		}
		return binary.LittleEndian.Uint32(b), nil
	case "int32_be", "int32_le":
		if spec.BitLength != 32 {
			return nil, fmt.Errorf("int32 requires bit_length 32")
		}
		b := readBytes(data, spec.BitOffset, 4)
		var u uint32
		if typ == "int32_be" {
			u = binary.BigEndian.Uint32(b)
		} else {
			u = binary.LittleEndian.Uint32(b)
		}
		return int32(u), nil
	case "float32_be", "float32_le":
		if spec.BitLength != 32 {
			return nil, fmt.Errorf("float32 requires bit_length 32")
		}
		b := readBytes(data, spec.BitOffset, 4)
		var u uint32
		if typ == "float32_be" {
			u = binary.BigEndian.Uint32(b)
		} else {
			u = binary.LittleEndian.Uint32(b)
		}
		return math.Float32frombits(u), nil
	case "uint_bits":
		if spec.BitLength < 1 || spec.BitLength > 64 {
			return nil, fmt.Errorf("uint_bits bit_length must be 1-64")
		}
		v := readBits(data, spec.BitOffset, spec.BitLength)
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type %q", spec.Type)
	}
}

// readBit returns the bit at bitOffset (0 or 1). Bit 0 is MSB of first byte.
func readBit(data []byte, bitOffset int) uint64 {
	byteIdx := bitOffset / 8
	bitIdx := 7 - (bitOffset % 8)
	if byteIdx >= len(data) {
		return 0
	}
	if (data[byteIdx]>>bitIdx)&1 != 0 {
		return 1
	}
	return 0
}

// readBits extracts bitLength bits starting at bitOffset (big-endian bit order), returns as uint64.
func readBits(data []byte, bitOffset, bitLength int) uint64 {
	var v uint64
	for i := 0; i < bitLength; i++ {
		v = (v << 1) | readBit(data, bitOffset+i)
	}
	return v
}

// readBitsSigned extracts bitLength bits and sign-extends to int64.
func readBitsSigned(data []byte, bitOffset, bitLength int) int64 {
	u := readBits(data, bitOffset, bitLength)
	// sign extend
	if bitLength < 64 && (u>>(bitLength-1))&1 != 0 {
		mask := uint64(1)<<bitLength - 1
		u |= ^mask
	}
	return int64(u)
}

// readBytes reads exactly nBytes bytes starting at bitOffset (must be byte-aligned).
func readBytes(data []byte, bitOffset, nBytes int) []byte {
	if bitOffset%8 != 0 {
		return nil
	}
	start := bitOffset / 8
	end := start + nBytes
	if end > len(data) {
		return nil
	}
	out := make([]byte, nBytes)
	copy(out, data[start:end])
	return out
}

// LoadBinaryMappingSchema reads a YAML file with source: binary and bit-level field definitions.
// Returns the field specs for use with DecodeBinary.
func LoadBinaryMappingSchema(path string) ([]FieldSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s BinaryMappingSchema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("binary schema yaml: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(s.Source)) != "binary" {
		return nil, fmt.Errorf("schema source must be 'binary', got %q", s.Source)
	}
	return s.Fields, nil
}
