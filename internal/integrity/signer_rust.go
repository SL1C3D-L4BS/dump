//go:build cgo
// +build cgo

package integrity

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// SignResult returns the Vericore Seal using the Rust PQC kernel and persistent keys.
// Ensures the keys directory exists so Rust can create the keypair if missing (auto-provisioning).
// If the target file is missing or unreadable, or Rust signing fails, falls back to Go signer
// so the audit log is always written ("Unicorn always signs").
func SignResult(filePath string) (string, error) {
	if err := EnsureKeysDir(); err != nil {
		return "", fmt.Errorf("ensure keys dir: %w", err)
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if err := ensureFileReadable(absPath); err != nil {
		return "", fmt.Errorf("file not ready for sign: %w", err)
	}
	seal, err := engine.SignFileRust(absPath, DefaultKeysPath())
	if err == nil {
		MaybePrintKeyProvisionedMessage()
		return seal, nil
	}
	// Force Go fallback: guarantee seal is produced and audit log written
	seal, fallbackErr := SignFileGo(absPath)
	if fallbackErr != nil {
		return "", fmt.Errorf("rust sign failed: %v; go fallback failed: %w", err, fallbackErr)
	}
	return seal, nil
}

// ensureFileReadable checks that the file exists and is readable before passing to Rust.
// Avoids generic failures when the path is relative (Rust cwd may differ) or file is locked.
func ensureFileReadable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	_ = f.Close()
	return nil
}
