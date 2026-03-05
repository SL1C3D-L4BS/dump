package integrity

import (
	"os"
	"path/filepath"
)

// DefaultKeysPath returns the path to the persistent Vericore PQC keypair.
// ~/.config/vericore/keys.json (or $XDG_CONFIG_HOME/vericore/keys.json).
func DefaultKeysPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "vericore", "keys.json")
}
