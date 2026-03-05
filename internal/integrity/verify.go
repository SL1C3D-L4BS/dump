//go:build cgo
// +build cgo

package integrity

import (
	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// VerifyResult verifies the file at path against the seal using the persistent public key.
// Returns "VERIFIED" or "TAMPERED".
func VerifyResult(filePath string, seal string) (string, error) {
	return engine.VerifyFileRust(filePath, seal, DefaultKeysPath())
}
