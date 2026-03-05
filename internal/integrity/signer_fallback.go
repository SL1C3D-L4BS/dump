//go:build cgo
// +build cgo

package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/vericore/go-pq-mmr"
)

// SignFileGo signs the file at path using the pure-Go MMR+PQC path and persisted keys.
// Used as fallback when SignFileRust fails so the audit log is always written.
func SignFileGo(filePath string) (string, error) {
	if err := EnsureKeys(); err != nil {
		return "", fmt.Errorf("ensure keys: %w", err)
	}
	_, privKey, err := LoadKeys(DefaultKeysPath())
	if err != nil {
		return "", fmt.Errorf("load keys: %w", err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	h := sha256.Sum256(data)
	hashSlice := h[:]

	tree := mmr.NewTree()
	position, err := tree.AppendSigned(hashSlice, privKey)
	if err != nil {
		return "", fmt.Errorf("append signed: %w", err)
	}
	proof, err := tree.GenerateProof(position)
	if err != nil || proof == nil {
		return "", fmt.Errorf("generate proof: %w", err)
	}
	root := tree.Root()
	if len(root) == 0 {
		return "", fmt.Errorf("empty MMR root")
	}
	seal := fmt.Sprintf("Vericore Seal\n  MMR Root:  %s\n  PQC Sig:   %s\n  File Hash: %s",
		hex.EncodeToString(root),
		hex.EncodeToString(proof.PQCSignature),
		hex.EncodeToString(hashSlice))
	return seal, nil
}
