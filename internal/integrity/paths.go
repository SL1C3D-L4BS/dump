package integrity

import "os"

func init() {
	// Align Go default key path with Rust signer expectation so the same path
	// is used everywhere (CLI, API, Tauri). If the Rust core reads an env var
	// for the key path, it will see this value.
	if path := DefaultKeysPath(); path != "" {
		_ = os.Setenv("VERICORE_KEY_PATH", path)
	}
}
