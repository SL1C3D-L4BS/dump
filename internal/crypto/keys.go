package crypto

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SL1C3D-L4BS/dump/internal/integrity"
	"github.com/vericore/go-pq-mmr"
)

const (
	keysDir      = "keys"
	keysArchive  = "archive"
	keysFilename = "keys.json"
)

// KeysJSON is the on-disk format for the Vericore PQC keypair (hex-encoded).
type KeysJSON struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// VericoreKeysDir returns ~/.vericore/keys (creates if needed).
func VericoreKeysDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	dir := filepath.Join(home, defaultAuditDir, keysDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir keys dir: %w", err)
	}
	return dir, nil
}

// VericoreKeysArchiveDir returns ~/.vericore/keys/archive (creates if needed).
func VericoreKeysArchiveDir() (string, error) {
	base, err := VericoreKeysDir()
	if err != nil {
		return "", err
	}
	archive := filepath.Join(base, keysArchive)
	if err := os.MkdirAll(archive, 0o700); err != nil {
		return "", fmt.Errorf("mkdir archive: %w", err)
	}
	return archive, nil
}

// Rotate generates a new Dilithium2 keypair, archives the old keys to
// ~/.vericore/keys/archive/, writes the new keys to the default Vericore
// keys path, and logs the rotation in the audit_rotations table.
func Rotate() error {
	keysPath := integrity.DefaultKeysPath()
	keysDir := filepath.Dir(keysPath)
	if err := os.MkdirAll(keysDir, 0o700); err != nil {
		return fmt.Errorf("mkdir keys parent: %w", err)
	}

	// Archive existing keys if present
	var archivePath string
	if data, err := os.ReadFile(keysPath); err == nil {
		archiveDir, err := VericoreKeysArchiveDir()
		if err != nil {
			return err
		}
		ts := time.Now().Format("20060102-150405")
		archivePath = filepath.Join(archiveDir, ts+"-"+keysFilename)
		if err := os.WriteFile(archivePath, data, 0o600); err != nil {
			return fmt.Errorf("archive old keys: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read current keys: %w", err)
	} else {
		// No existing keys; rotation is initial key generation
		archivePath = "(none)"
	}

	// Generate new Dilithium2 keypair
	pub, priv, err := mmr.GeneratePQCKeys()
	if err != nil {
		return fmt.Errorf("generate PQC keys: %w", err)
	}

	// Encode as hex for storage
	keys := KeysJSON{
		PublicKey:  hex.EncodeToString(pub),
		PrivateKey: hex.EncodeToString(priv),
	}
	raw, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal keys: %w", err)
	}
	if err := os.WriteFile(keysPath, raw, 0o600); err != nil {
		return fmt.Errorf("write new keys: %w", err)
	}

	// Log rotation in audit_rotations (use actual path for old keys)
	oldKeysPath := keysPath
	if archivePath == "(none)" {
		oldKeysPath = "(none)"
	}
	if err := AppendRotation(oldKeysPath, archivePath); err != nil {
		return fmt.Errorf("log rotation: %w", err)
	}
	return nil
}
