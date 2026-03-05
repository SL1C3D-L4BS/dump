//go:build !cgo
// +build !cgo

package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/vericore/go-pq-mmr"
)

// SignResult hashes the file, appends the hash to an MMR, signs it with ML-DSA (PQC),
// and returns the Vericore Seal. Go implementation (used when building without cgo).
func SignResult(filePath string) (string, error) {
	data, err := readFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	h := hashFileBytes(data)

	tree := mmr.NewTree()
	pubKey, privKey, err := mmr.GeneratePQCKeys()
	if err != nil {
		return "", fmt.Errorf("generate PQC keys: %w", err)
	}
	position, err := tree.AppendSigned(h, privKey)
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
		hex.EncodeToString(h))
	_ = pubKey
	return seal, nil
}

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func hashFileBytes(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
