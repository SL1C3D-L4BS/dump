// Package protobufdecode provides heuristic Protobuf decoding without a .proto schema.
// Used by the engine and by the WASM build (minimal deps for GOOS=js GOARCH=wasm).

package protobufdecode

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// Decode recursively decodes a binary Protobuf payload into a map[string]interface{}
// without requiring a .proto schema. Field keys are stringified field numbers.
// Repeated fields are coalesced into slices.
func Decode(b []byte) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	seen := make(map[string][]interface{})
	for len(b) > 0 {
		fieldNumber, wireType, tagLen := protowire.ConsumeTag(b)
		if tagLen < 0 {
			return nil, fmt.Errorf("protobuf tag: %w", protowire.ParseError(tagLen))
		}
		b = b[tagLen:]

		key := strconv.FormatInt(int64(fieldNumber), 10)
		var val interface{}
		var consumed int

		switch wireType {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return nil, fmt.Errorf("protobuf varint field %s: %w", key, protowire.ParseError(n))
			}
			val, consumed = v, n

		case protowire.Fixed32Type:
			v, n := protowire.ConsumeFixed32(b)
			if n < 0 {
				return nil, fmt.Errorf("protobuf fixed32 field %s: %w", key, protowire.ParseError(n))
			}
			val, consumed = v, n

		case protowire.Fixed64Type:
			v, n := protowire.ConsumeFixed64(b)
			if n < 0 {
				return nil, fmt.Errorf("protobuf fixed64 field %s: %w", key, protowire.ParseError(n))
			}
			val, consumed = v, n

		case protowire.BytesType:
			raw, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, fmt.Errorf("protobuf bytes field %s: %w", key, protowire.ParseError(n))
			}
			consumed = n
			val = decodeBytesHeuristic(raw)

		default:
			return nil, fmt.Errorf("protobuf unsupported wire type %d for field %s", wireType, key)
		}

		b = b[consumed:]
		seen[key] = append(seen[key], val)
	}

	for k, list := range seen {
		if len(list) == 1 {
			out[k] = list[0]
		} else {
			out[k] = list
		}
	}
	return out, nil
}

func decodeBytesHeuristic(raw []byte) interface{} {
	if len(raw) == 0 {
		return ""
	}
	nested, err := Decode(raw)
	if err == nil && len(nested) > 0 {
		return nested
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	return base64.StdEncoding.EncodeToString(raw)
}
