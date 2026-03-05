// Package engine: analyze a live database to produce a MirrorSpec (schema + statistics).

package engine

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// AnalyzeDatabase connects to the source DB (Postgres or SQLite), reads schema and column stats, returns a MirrorSpec.
func AnalyzeDatabase(connStr string) (*MirrorSpec, error) {
	driver, dsn := parseDBURLForMirror(connStr)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	spec := &MirrorSpec{Tables: nil}
	if driver == "postgres" {
		return analyzePostgres(db, spec)
	}
	return analyzeSQLite(db, spec)
}

func parseDBURLForMirror(dbURL string) (driver, dsn string) {
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

func analyzePostgres(db *sql.DB, spec *MirrorSpec) (*MirrorSpec, error) {
	// Tables (from information_schema.tables, exclude system)
	rows, err := db.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Primary keys
	pkMap := make(map[string][]string)
	pkRows, err := db.Query(`
		SELECT kcu.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = 'public'
		ORDER BY kcu.table_name, kcu.ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("primary keys: %w", err)
	}
	defer pkRows.Close()
	for pkRows.Next() {
		var tbl, col string
		if err := pkRows.Scan(&tbl, &col); err != nil {
			return nil, err
		}
		pkMap[tbl] = append(pkMap[tbl], col)
	}
	pkRows.Close()

	// Foreign keys (child -> parent)
	fkMap := make(map[string][]ForeignKey) // key = child table
	fkRows, err := db.Query(`
		SELECT kcu.table_name, kcu.column_name, ccu.table_name AS parent_table, ccu.column_name AS parent_column
		FROM information_schema.referential_constraints rc
		JOIN information_schema.key_column_usage kcu
		  ON rc.constraint_name = kcu.constraint_name AND rc.constraint_schema = kcu.constraint_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON rc.unique_constraint_name = ccu.constraint_name AND rc.unique_constraint_schema = ccu.constraint_schema
		WHERE kcu.table_schema = 'public'
		ORDER BY kcu.table_name, kcu.ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("foreign keys: %w", err)
	}
	defer fkRows.Close()
	for fkRows.Next() {
		var childTable, childCol, parentTable, parentCol string
		if err := fkRows.Scan(&childTable, &childCol, &parentTable, &parentCol); err != nil {
			return nil, err
		}
		fkMap[childTable] = append(fkMap[childTable], ForeignKey{
			ChildColumn:  childCol,
			ParentTable:  parentTable,
			ParentColumn: parentCol,
		})
	}
	fkRows.Close()

	// Columns and stats per table
	for _, tableName := range tableNames {
		colRows, err := db.Query(`
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position`, tableName)
		if err != nil {
			return nil, fmt.Errorf("columns for %s: %w", tableName, err)
		}
		var cols []ColumnSpec
		for colRows.Next() {
			var name, dataType, isNullable string
			if err := colRows.Scan(&name, &dataType, &isNullable); err != nil {
				colRows.Close()
				return nil, err
			}
			col := ColumnSpec{
				Name:     name,
				Type:     dataType,
				Nullable: strings.EqualFold(isNullable, "YES"),
			}
			stats, err := columnStatsPostgres(db, tableName, name, dataType)
			if err != nil {
				colRows.Close()
				return nil, fmt.Errorf("stats %s.%s: %w", tableName, name, err)
			}
			col.Stats = stats
			cols = append(cols, col)
		}
		colRows.Close()

		ts := TableSpec{
			Name:          tableName,
			Columns:       cols,
			PrimaryKey:    pkMap[tableName],
			Relationships: fkMap[tableName],
		}
		spec.Tables = append(spec.Tables, ts)
	}
	return spec, nil
}

func columnStatsPostgres(db *sql.DB, table, column, dataType string) (*ColumnStats, error) {
	quotedTable := `"` + strings.Replace(table, `"`, `""`, -1) + `"`
	quotedCol := `"` + strings.Replace(column, `"`, `""`, -1) + `"`
	// COUNT(*), COUNT(DISTINCT col), MIN(col), MAX(col), COUNT(*) FILTER (WHERE col IS NULL)
	q := fmt.Sprintf(`SELECT
		COUNT(*)::bigint,
		COUNT(DISTINCT %s)::bigint,
		MIN(%s),
		MAX(%s),
		COUNT(*) FILTER (WHERE %s IS NULL)::bigint
		FROM %s`, quotedCol, quotedCol, quotedCol, quotedCol, quotedTable)
	var count, distinct, nullCount int64
	var minVal, maxVal sql.NullString
	if err := db.QueryRow(q).Scan(&count, &distinct, &minVal, &maxVal, &nullCount); err != nil {
		return nil, err
	}
	nullPct := 0.0
	if count > 0 {
		nullPct = float64(nullCount) / float64(count) * 100
	}
	minI, maxI := interface{}(nil), interface{}(nil)
	if minVal.Valid {
		minI = coerceStatValue(minVal.String, dataType)
	}
	if maxVal.Valid {
		maxI = coerceStatValue(maxVal.String, dataType)
	}
	return &ColumnStats{
		Min:            minI,
		Max:            maxI,
		Count:          count,
		NullCount:      nullCount,
		NullPercentage: nullPct,
		Cardinality:    distinct,
	}, nil
}

func analyzeSQLite(db *sql.DB, spec *MirrorSpec) (*MirrorSpec, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	rows.Close()

	fkMap := make(map[string][]ForeignKey)

	for _, tableName := range tableNames {
		// PRAGMA table_info(sqlite_quote(table))
		infoRows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdentifier(tableName)))
		if err != nil {
			return nil, fmt.Errorf("table_info %s: %w", tableName, err)
		}
		var cols []ColumnSpec
		var pkCols []string
		for infoRows.Next() {
			var cid int64
			var name, colType string
			var notNull int
			var dflt interface{}
			var pk int
			if err := infoRows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
				infoRows.Close()
				return nil, err
			}
			if pk > 0 {
				pkCols = append(pkCols, name)
			}
			col := ColumnSpec{
				Name:     name,
				Type:     colType,
				Nullable: notNull == 0,
			}
			stats, err := columnStatsSQLite(db, tableName, name, colType)
			if err != nil {
				infoRows.Close()
				return nil, fmt.Errorf("stats %s.%s: %w", tableName, name, err)
			}
			col.Stats = stats
			cols = append(cols, col)
		}
		infoRows.Close()

		// SQLite PRAGMA foreign_key_list(table)
		fkRows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteSQLiteIdentifier(tableName)))
		if err == nil {
			for fkRows.Next() {
				var id, seq int
				var parentTable, fromCol, toCol string
				if err := fkRows.Scan(&id, &seq, &parentTable, &fromCol, &toCol); err != nil {
					fkRows.Close()
					break
				}
				fkMap[tableName] = append(fkMap[tableName], ForeignKey{
					ChildColumn:  fromCol,
					ParentTable:  parentTable,
					ParentColumn: toCol,
				})
			}
			fkRows.Close()
		}

		spec.Tables = append(spec.Tables, TableSpec{
			Name:          tableName,
			Columns:       cols,
			PrimaryKey:    pkCols,
			Relationships: fkMap[tableName],
		})
	}
	return spec, nil
}

func columnStatsSQLite(db *sql.DB, table, column, dataType string) (*ColumnStats, error) {
	quoted := quoteSQLiteIdentifier(table)
	q := fmt.Sprintf("SELECT COUNT(*), COUNT(DISTINCT %s), MIN(%s), MAX(%s), SUM(CASE WHEN %s IS NULL THEN 1 ELSE 0 END) FROM %s",
		quoteSQLiteIdentifier(column), quoteSQLiteIdentifier(column), quoteSQLiteIdentifier(column),
		quoteSQLiteIdentifier(column), quoted)
	var count, nullCount int64
	var minVal, maxVal sql.NullString
	var distinct sql.NullInt64
	if err := db.QueryRow(q).Scan(&count, &distinct, &minVal, &maxVal, &nullCount); err != nil {
		return nil, err
	}
	nullPct := 0.0
	if count > 0 {
		nullPct = float64(nullCount) / float64(count) * 100
	}
	card := int64(0)
	if distinct.Valid {
		card = distinct.Int64
	}
	minI, maxI := interface{}(nil), interface{}(nil)
	if minVal.Valid {
		minI = coerceStatValue(minVal.String, dataType)
	}
	if maxVal.Valid {
		maxI = coerceStatValue(maxVal.String, dataType)
	}
	return &ColumnStats{
		Min:            minI,
		Max:            maxI,
		Count:          count,
		NullCount:      nullCount,
		NullPercentage: nullPct,
		Cardinality:    card,
	}, nil
}

func quoteSQLiteIdentifier(name string) string {
	return `"` + strings.Replace(name, `"`, `""`, -1) + `"`
}

// coerceStatValue keeps numbers as float64/int64 where possible for uniform sampling; otherwise string.
func coerceStatValue(s, dataType string) interface{} {
	lower := strings.ToLower(dataType)
	switch {
	case strings.Contains(lower, "int"):
		var i int64
		if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
			return i
		}
	case strings.Contains(lower, "float") || strings.Contains(lower, "double") || strings.Contains(lower, "numeric") || strings.Contains(lower, "decimal") || strings.Contains(lower, "real"):
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			return f
		}
	}
	return s
}
