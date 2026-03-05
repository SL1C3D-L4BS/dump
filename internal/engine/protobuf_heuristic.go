// Package engine: heuristic Protobuf decoding without a .proto schema.

package engine

import (
	"github.com/SL1C3D-L4BS/dump/internal/protobufdecode"
)

// DecodeProtobuf recursively decodes a binary Protobuf payload into a map[string]interface{}
// without requiring a .proto schema. Field keys are stringified field numbers.
// Repeated fields are coalesced into slices.
func DecodeProtobuf(b []byte) (map[string]interface{}, error) {
	return protobufdecode.Decode(b)
}
