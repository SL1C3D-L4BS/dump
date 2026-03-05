package crypto

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// AuditEntry represents one persisted Vericore audit event.
type AuditEntry struct {
	ID        int64
	Timestamp time.Time
	Tool      string
	FilePath  string
	MMRRoot   string
	FileHash  string
	PQCSig    string
}

const (
	defaultAuditDir  = ".vericore"
	defaultAuditFile = "audit.db"
)

func auditDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	dir := filepath.Join(home, defaultAuditDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir audit dir: %w", err)
	}
	return filepath.Join(dir, defaultAuditFile), nil
}

func openAuditDB() (*sql.DB, error) {
	path, err := auditDBPath()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open audit db: %w", err)
	}
	schema := `
CREATE TABLE IF NOT EXISTS audit_events (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  ts        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tool      TEXT      NOT NULL,
  file_path TEXT      NOT NULL,
  mmr_root  TEXT      NOT NULL,
  file_hash TEXT      NOT NULL,
  pqc_sig   TEXT      NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_rotations (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  ts            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  old_keys_path TEXT      NOT NULL,
  archive_path  TEXT      NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init audit schema: %w", err)
	}
	return db, nil
}

// AppendRotation logs a key rotation event (old keys path and archive path).
func AppendRotation(oldKeysPath, archivePath string) error {
	db, err := openAuditDB()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`INSERT INTO audit_rotations (old_keys_path, archive_path) VALUES (?, ?)`,
		oldKeysPath, archivePath)
	if err != nil {
		return fmt.Errorf("insert rotation event: %w", err)
	}
	return nil
}

// AppendFromSeal parses a Vericore Seal string and appends an audit entry
// for the given tool (e.g. "map" or "mirror") and file path.
func AppendFromSeal(tool, filePath, seal string) error {
	root, sig, hash, err := parseSeal(seal)
	if err != nil {
		return err
	}
	db, err := openAuditDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		`INSERT INTO audit_events (tool, file_path, mmr_root, file_hash, pqc_sig) VALUES (?, ?, ?, ?, ?)`,
		strings.ToLower(strings.TrimSpace(tool)),
		filePath,
		root,
		hash,
		sig,
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// GetByID returns a single audit entry by id, or nil if not found.
func GetByID(id int64) (*AuditEntry, error) {
	db, err := openAuditDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var e AuditEntry
	err = db.QueryRow(
		`SELECT id, ts, tool, file_path, mmr_root, file_hash, pqc_sig
         FROM audit_events WHERE id = ?`, id).Scan(
		&e.ID, &e.Timestamp, &e.Tool, &e.FilePath, &e.MMRRoot, &e.FileHash, &e.PQCSig)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query audit event: %w", err)
	}
	return &e, nil
}

// List returns the most recent n audit entries (default ordering: newest first).
// If limit is 0 or negative, defaults to 100.
func List(limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	db, err := openAuditDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT id, ts, tool, file_path, mmr_root, file_hash, pqc_sig
         FROM audit_events
         ORDER BY id DESC
         LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Tool, &e.FilePath, &e.MMRRoot, &e.FileHash, &e.PQCSig); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAll returns all audit entries (newest first). Use with care on large logs.
func ListAll() ([]AuditEntry, error) {
	db, err := openAuditDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT id, ts, tool, file_path, mmr_root, file_hash, pqc_sig
         FROM audit_events ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Tool, &e.FilePath, &e.MMRRoot, &e.FileHash, &e.PQCSig); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// parseSeal extracts the MMR root, PQC signature, and file hash from a Vericore Seal string.
func parseSeal(seal string) (root, sig, hash string, err error) {
	lines := strings.Split(seal, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "MMR Root:"):
			root = strings.TrimSpace(strings.TrimPrefix(line, "MMR Root:"))
		case strings.HasPrefix(line, "PQC Sig:"):
			sig = strings.TrimSpace(strings.TrimPrefix(line, "PQC Sig:"))
		case strings.HasPrefix(line, "File Hash:"):
			hash = strings.TrimSpace(strings.TrimPrefix(line, "File Hash:"))
		}
	}
	if root == "" || sig == "" || hash == "" {
		return "", "", "", fmt.Errorf("invalid Vericore Seal: missing fields")
	}
	return root, sig, hash, nil
}

