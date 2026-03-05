//go:build cgo
// +build cgo

package integrity

import (
	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// SignResult returns the Vericore Seal using the Rust PQC kernel and persistent keys.
func SignResult(filePath string) (string, error) {
	return engine.SignFileRust(filePath, DefaultKeysPath())
}
