// Package engine: heuristic Protobuf decoding without a .proto schema.

package engine

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// DecodeProtobuf recursively decodes a binary Protobuf payload into a map[string]interface{}
// without requiring a .proto schema. Field keys are stringified field numbers.
// Repeated fields are coalesced into slices.
func DecodeProtobuf(b []byte) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	// Track existing values by field key for coalescing repeated fields into slices
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
			// StartGroupType, EndGroupType or unknown: skip unknown wire type by consuming rest or one byte to avoid infinite loop
			// For unknown types we skip this field value; ConsumeTag already consumed the tag.
			// We need to skip the value. For groups we'd need to parse group boundaries; for unknown wire type we skip.
			return nil, fmt.Errorf("protobuf unsupported wire type %d for field %s", wireType, key)
		}

		b = b[consumed:]

		// Coalesce repeated fields into slices
		seen[key] = append(seen[key], val)
	}

	// Build final map: single value vs slice
	for k, list := range seen {
		if len(list) == 1 {
			out[k] = list[0]
		} else {
			out[k] = list
		}
	}
	return out, nil
}

// decodeBytesHeuristic interprets length-delimited bytes as nested message, UTF-8 string, or base64.
func decodeBytesHeuristic(raw []byte) interface{} {
	if len(raw) == 0 {
		return ""
	}
	// 1) Try recursive decode as nested message
	nested, err := DecodeProtobuf(raw)
	if err == nil && len(nested) > 0 {
		return nested
	}
	// 2) Valid UTF-8 -> string
	if utf8.Valid(raw) {
		return string(raw)
	}
	// 3) Otherwise base64
	return base64.StdEncoding.EncodeToString(raw)
}
