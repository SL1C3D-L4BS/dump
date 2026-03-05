//go:build !cgo
// +build !cgo

package integrity

// VerifyResult is only implemented when building with cgo (Rust). Without cgo, verification is not available.
func VerifyResult(filePath string, seal string) (string, error) {
	return "TAMPERED", nil
}
