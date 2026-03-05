// Package engine: create tables from MirrorSpec and bulk-insert synthetic rows into a target DB.

package engine

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// SQLWriter writes a MirrorSpec (CREATE TABLE) and bulk inserts rows to a target DB.
type SQLWriter struct {
	db     *sql.DB
	driver string
}

// NewSQLWriter opens the target DB (postgres or sqlite) and returns a writer.
func NewSQLWriter(targetConn string) (*SQLWriter, error) {
	driver, dsn := parseDBURLForMirror(targetConn)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open target db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping target db: %w", err)
	}
	return &SQLWriter{db: db, driver: driver}, nil
}

// CreateTables creates all tables from the spec (DROP IF EXISTS then CREATE).
func (w *SQLWriter) CreateTables(spec *MirrorSpec) error {
	order := topologicalSort(spec)
	// Drop in reverse order so children are dropped before parents (avoid FK violations)
	for i := len(order) - 1; i >= 0; i-- {
		tableName := order[i]
		dropSQL := "DROP TABLE IF EXISTS " + quoteIdentSQLite(tableName)
		if w.driver == "postgres" {
			dropSQL = "DROP TABLE IF EXISTS " + quoteIdentPostgres(tableName) + " CASCADE"
		}
		if _, err := w.db.Exec(dropSQL); err != nil {
			return fmt.Errorf("drop table %s: %w", tableName, err)
		}
	}
	for _, tableName := range order {
		ts := findTableSpec(spec, tableName)
		if ts == nil {
			continue
		}
		ddl := w.buildCreateTable(ts)
		if _, err := w.db.Exec(ddl); err != nil {
			return fmt.Errorf("create table %s: %w", tableName, err)
		}
	}
	return nil
}

func (w *SQLWriter) buildCreateTable(ts *TableSpec) string {
	var sb strings.Builder
	if w.driver == "sqlite" {
		sb.WriteString("CREATE TABLE ")
		sb.WriteString(quoteIdentSQLite(ts.Name))
		sb.WriteString(" ( ")
	} else {
		sb.WriteString("CREATE TABLE ")
		sb.WriteString(quoteIdentPostgres(ts.Name))
		sb.WriteString(" ( ")
	}
	for i, col := range ts.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		if w.driver == "sqlite" {
			sb.WriteString(quoteIdentSQLite(col.Name))
		} else {
			sb.WriteString(quoteIdentPostgres(col.Name))
		}
		sb.WriteString(" ")
		sb.WriteString(w.mapType(col.Type))
		if !col.Nullable {
			sb.WriteString(" NOT NULL")
		}
	}
	if w.driver == "postgres" && len(ts.PrimaryKey) > 0 {
		sb.WriteString(", PRIMARY KEY (")
		for i, pk := range ts.PrimaryKey {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(quoteIdentPostgres(pk))
		}
		sb.WriteString(")")
	}
	if w.driver == "sqlite" && len(ts.PrimaryKey) > 0 {
		// SQLite: single column PK can be in column def; multi-column needs separate line
		// For simplicity we add PRIMARY KEY (a, b) before closing paren
		sb.WriteString(", PRIMARY KEY (")
		for i, pk := range ts.PrimaryKey {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(quoteIdentSQLite(pk))
		}
		sb.WriteString(")")
	}
	sb.WriteString(" );")
	return sb.String()
}

func (w *SQLWriter) mapType(dataType string) string {
	lower := strings.ToLower(dataType)
	switch {
	case strings.Contains(lower, "serial") || strings.Contains(lower, "identity"):
		if w.driver == "sqlite" {
			return "INTEGER"
		}
		return "SERIAL"
	case strings.Contains(lower, "bigserial"):
		if w.driver == "sqlite" {
			return "INTEGER"
		}
		return "BIGSERIAL"
	case strings.Contains(lower, "int") && !strings.Contains(lower, "interval"):
		return "INTEGER"
	case strings.Contains(lower, "bigint"):
		return "BIGINT"
	case strings.Contains(lower, "smallint"):
		return "SMALLINT"
	case strings.Contains(lower, "float") || strings.Contains(lower, "double") || strings.Contains(lower, "real"):
		return "REAL"
	case strings.Contains(lower, "numeric") || strings.Contains(lower, "decimal"):
		return "REAL"
	case strings.Contains(lower, "bool"):
		return "INTEGER" // SQLite no native bool; 0/1
	case strings.Contains(lower, "date") || strings.Contains(lower, "time"):
		return "TEXT"
	default:
		return "TEXT"
	}
}

func quoteIdentPostgres(name string) string {
	return `"` + strings.Replace(name, `"`, `""`, -1) + `"`
}

func quoteIdentSQLite(name string) string {
	return `"` + strings.Replace(name, `"`, `""`, -1) + `"`
}

// InsertRows bulk-inserts generated rows per table. Tables must be in dependency order (parents first).
func (w *SQLWriter) InsertRows(spec *MirrorSpec, data map[string][]map[string]interface{}) error {
	order := topologicalSort(spec)
	for _, tableName := range order {
		ts := findTableSpec(spec, tableName)
		if ts == nil {
			continue
		}
		rows := data[tableName]
		if len(rows) == 0 {
			continue
		}
		if err := w.bulkInsert(ts, rows); err != nil {
			return fmt.Errorf("insert %s: %w", tableName, err)
		}
	}
	return nil
}

func (w *SQLWriter) bulkInsert(ts *TableSpec, rows []map[string]interface{}) error {
	colNames := make([]string, 0, len(ts.Columns))
	for _, c := range ts.Columns {
		colNames = append(colNames, c.Name)
	}
	placeholders := make([]string, len(colNames))
	if w.driver == "postgres" {
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
	} else {
		for i := range placeholders {
			placeholders[i] = "?"
		}
	}
	quotedCols := make([]string, len(colNames))
	for i, n := range colNames {
		if w.driver == "postgres" {
			quotedCols[i] = quoteIdentPostgres(n)
		} else {
			quotedCols[i] = quoteIdentSQLite(n)
		}
	}
	tableQuoted := quoteIdentSQLite(ts.Name)
	if w.driver == "postgres" {
		tableQuoted = quoteIdentPostgres(ts.Name)
	}
	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableQuoted,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "))
	for _, row := range rows {
		args := make([]interface{}, 0, len(colNames))
		for _, col := range colNames {
			args = append(args, row[col])
		}
		if _, err := w.db.Exec(stmt, args...); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the database connection.
func (w *SQLWriter) Close() error {
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}
