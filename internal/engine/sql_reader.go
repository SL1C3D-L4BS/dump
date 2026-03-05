package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"database/sql"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// SQLReader streams a SQL query result as JSONL (one JSON object per row) for MapStream.
// Supports Postgres (postgres://) and SQLite (file: or sqlite:).
type SQLReader struct {
	db    *sql.DB
	rows  *sql.Rows
	buf   []byte
	done  bool
}

// NewSQLReader opens dbURL, runs query, and returns an io.Reader that streams JSONL.
func NewSQLReader(dbURL, query string) (*SQLReader, error) {
	driver, dsn := parseDBURL(dbURL)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	rows, err := db.Query(query)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("query: %w", err)
	}
	return &SQLReader{db: db, rows: rows}, nil
}

func parseDBURL(dbURL string) (driver, dsn string) {
	s := strings.TrimSpace(dbURL)
	switch {
	case strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://"):
		return "postgres", s
	case strings.HasPrefix(s, "file:") || strings.HasPrefix(s, "sqlite:"):
		return "sqlite", s
	default:
		return "sqlite", s
	}
}

func (r *SQLReader) Read(p []byte) (n int, err error) {
	for len(r.buf) < len(p) && !r.done {
		if r.rows == nil {
			r.done = true
			break
		}
		if !r.rows.Next() {
			r.rows.Close()
			r.rows = nil
			r.done = true
			break
		}
		row, err := rowToMap(r.rows)
		if err != nil {
			return 0, err
		}
		line, err := json.Marshal(row)
		if err != nil {
			return 0, err
		}
		r.buf = append(r.buf, line...)
		r.buf = append(r.buf, '\n')
	}
	if len(r.buf) == 0 {
		return 0, io.EOF
	}
	n = copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func rowToMap(rows *sql.Rows) (map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}
	if err := rows.Scan(valPtrs...); err != nil {
		return nil, err
	}
	m := make(map[string]interface{}, len(cols))
	for i, c := range cols {
		v := vals[i]
		if b, ok := v.([]byte); ok {
			v = string(b)
		}
		m[c] = v
	}
	return m, nil
}

func (r *SQLReader) Close() error {
	r.done = true
	if r.rows != nil {
		r.rows.Close()
		r.rows = nil
	}
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
