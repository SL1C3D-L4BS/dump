package integrity

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vericore/go-pq-mmr"
)

const violetKeyMsg = "\033[35m🔑 SL1C3D-L4BS Security: New Dilithium2 PQC keypair generated and secured.\033[0m\n"

// keysFileJSON matches the Rust KeysJson format (public_key_hex, secret_key_hex).
type keysFileJSON struct {
	PublicKeyHex string `json:"public_key_hex"`
	SecretKeyHex string `json:"secret_key_hex"`
}

// EnsureKeysDir creates the parent directory of DefaultKeysPath() so the Rust signer
// can create the key file (Rust uses Dilithium2; Go go-pq-mmr may differ). Call before
// SignFileRust when using cgo.
func EnsureKeysDir() error {
	path := DefaultKeysPath()
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0o700)
}

var printKeyProvisionedOnce sync.Once

// MaybePrintKeyProvisionedMessage prints the one-time violet message if the keys file
// was modified in the last 2 seconds (i.e. Rust just created it). Call after a successful
// SignFileRust when using cgo. Only prints once per process.
func MaybePrintKeyProvisionedMessage() {
	path := DefaultKeysPath()
	info, err := os.Stat(path)
	if err != nil || info.ModTime().Before(time.Now().Add(-2*time.Second)) {
		return
	}
	printKeyProvisionedOnce.Do(func() { fmt.Fprint(os.Stderr, violetKeyMsg) })
}

// EnsureKeys creates the default Vericore keypair at DefaultKeysPath() if it does not exist.
// Writes Rust-compatible JSON (public_key_hex, secret_key_hex). Use for the pure-Go signer
// (!cgo). For cgo, the Rust signer creates keys; call EnsureKeysDir before sign and
// MaybePrintKeyProvisionedMessage after success. Safe to call before every SignResult when !cgo.
func EnsureKeys() error {
	path := DefaultKeysPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat keys path: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir keys dir: %w", err)
	}

	pub, priv, err := mmr.GeneratePQCKeys()
	if err != nil {
		return fmt.Errorf("generate PQC keys: %w", err)
	}

	keys := keysFileJSON{
		PublicKeyHex: hex.EncodeToString(pub),
		SecretKeyHex: hex.EncodeToString(priv),
	}
	raw, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal keys: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write keys: %w", err)
	}

	fmt.Fprint(os.Stderr, violetKeyMsg)
	return nil
}

// LoadKeys reads the keypair from path (Rust-format JSON). Used by the Go signer when !cgo.
func LoadKeys(path string) (pubKey, privKey []byte, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read keys: %w", err)
	}
	var keys keysFileJSON
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, nil, fmt.Errorf("parse keys json: %w", err)
	}
	pubKey, err = hex.DecodeString(keys.PublicKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode public key hex: %w", err)
	}
	privKey, err = hex.DecodeString(keys.SecretKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode secret key hex: %w", err)
	}
	return pubKey, privKey, nil
}
