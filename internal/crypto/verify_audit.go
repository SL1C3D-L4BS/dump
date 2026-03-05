package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/integrity"
)

// VerifyResult holds the outcome of verifying one audit entry.
type VerifyResult struct {
	ID       int64
	FilePath string
	Tool     string
	Status   string // VERIFIED, TAMPERED, MISSING, SIG_INVALID
	Reason   string
}

// SealFromEntry reconstructs the Vericore Seal string from an audit entry.
func SealFromEntry(e *AuditEntry) string {
	return fmt.Sprintf("Vericore Seal\n  MMR Root:  %s\n  PQC Sig:   %s\n  File Hash: %s",
		strings.TrimSpace(e.MMRRoot),
		strings.TrimSpace(e.PQCSig),
		strings.TrimSpace(e.FileHash))
}

// HashFile computes SHA256 of the file at path and returns hex-encoded hash.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyEntry checks the current state of the file for the given audit entry.
// 1) If file is missing → MISSING.
// 2) Re-compute file hash; if different from stored → TAMPERED.
// 3) Reconstruct seal and verify PQC signature against MMR root using the
//    SL1C3D-L4BS public key (via integrity.VerifyResult).
func VerifyEntry(e *AuditEntry) (VerifyResult, error) {
	out := VerifyResult{ID: e.ID, FilePath: e.FilePath, Tool: e.Tool}
	storedHash := strings.TrimSpace(strings.ToLower(e.FileHash))

	if _, err := os.Stat(e.FilePath); err != nil {
		if os.IsNotExist(err) {
			out.Status = "MISSING"
			out.Reason = "file not found"
			return out, nil
		}
		return out, fmt.Errorf("stat file: %w", err)
	}

	currentHash, err := HashFile(e.FilePath)
	if err != nil {
		return out, fmt.Errorf("hash file: %w", err)
	}
	currentHash = strings.ToLower(currentHash)

	if currentHash != storedHash {
		out.Status = "TAMPERED"
		out.Reason = "file hash changed since audit"
		return out, nil
	}

	seal := SealFromEntry(e)
	status, err := integrity.VerifyResult(e.FilePath, seal)
	if err != nil {
		return out, fmt.Errorf("verify seal: %w", err)
	}
	if status == "VERIFIED" {
		out.Status = "VERIFIED"
		return out, nil
	}
	out.Status = "SIG_INVALID"
	out.Reason = "PQC signature verification failed"
	return out, nil
}

// VerifyByID verifies the single audit entry with the given id.
func VerifyByID(id int64) (VerifyResult, error) {
	e, err := GetByID(id)
	if err != nil {
		return VerifyResult{}, err
	}
	if e == nil {
		return VerifyResult{}, fmt.Errorf("audit entry %d not found", id)
	}
	return VerifyEntry(e)
}

// VerifyAll verifies every entry in the audit log and returns results.
func VerifyAll() ([]VerifyResult, error) {
	entries, err := ListAll()
	if err != nil {
		return nil, err
	}
	results := make([]VerifyResult, 0, len(entries))
	for _, e := range entries {
		r, err := VerifyEntry(&e)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}

// VerifyEntries verifies the given list of audit entries.
func VerifyEntries(entries []AuditEntry) ([]VerifyResult, error) {
	results := make([]VerifyResult, 0, len(entries))
	for i := range entries {
		r, err := VerifyEntry(&entries[i])
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}
