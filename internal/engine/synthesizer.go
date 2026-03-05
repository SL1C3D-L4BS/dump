// Package engine: generate synthetic rows from a MirrorSpec with topological ordering for FKs.

package engine

import (
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateSyntheticData returns per-table rows: map[tableName][]map[columnName]value.
// Tables are topologically sorted so parents are generated before children; FK values reference parent IDs.
func GenerateSyntheticData(spec *MirrorSpec, rowCountPerTable int) map[string][]map[string]interface{} {
	if spec == nil || len(spec.Tables) == 0 {
		return nil
	}
	order := topologicalSort(spec)
	generated := make(map[string][]map[string]interface{})

	for _, tableName := range order {
		ts := findTableSpec(spec, tableName)
		if ts == nil {
			continue
		}
		rows := generateTableRows(ts, rowCountPerTable, generated)
		generated[tableName] = rows
	}
	return generated
}

func findTableSpec(spec *MirrorSpec, name string) *TableSpec {
	for i := range spec.Tables {
		if spec.Tables[i].Name == name {
			return &spec.Tables[i]
		}
	}
	return nil
}

// topologicalSort returns table names in dependency order (parents before children).
func topologicalSort(spec *MirrorSpec) []string {
	// Build adjacency: child -> parents (we need order such that no child is before its parent)
	indegree := make(map[string]int)
	for _, t := range spec.Tables {
		indegree[t.Name] = 0
	}
	for _, t := range spec.Tables {
		for range t.Relationships {
			indegree[t.Name]++ // child depends on parent
		}
	}
	// Actually: we want parents first. So "parent" has no outgoing FKs to tables we generate; "child" has FKs to parent.
	// So dependency is: child depends on parent. Topological order = parent before child.
	// Build reverse: for each table, which tables depend on it? Then we can BFS from roots (tables no one depends on).
	dependsOn := make(map[string][]string) // table -> list of tables that depend on it (children)
	for _, t := range spec.Tables {
		for _, fk := range t.Relationships {
			dependsOn[fk.ParentTable] = append(dependsOn[fk.ParentTable], t.Name)
		}
	}
	indegree = make(map[string]int)
	for _, t := range spec.Tables {
		indegree[t.Name] = 0
	}
	for _, t := range spec.Tables {
		for range t.Relationships {
			indegree[t.Name]++ // number of parents this table has (in our set)
		}
	}
	var queue []string
	for _, t := range spec.Tables {
		if indegree[t.Name] == 0 {
			queue = append(queue, t.Name)
		}
	}
	var order []string
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		for _, v := range dependsOn[u] {
			indegree[v]--
			if indegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	// If there are cycles, some tables never get indegree 0; add them at end
	seen := make(map[string]bool)
	for _, n := range order {
		seen[n] = true
	}
	for _, t := range spec.Tables {
		if !seen[t.Name] {
			order = append(order, t.Name)
		}
	}
	return order
}

func generateTableRows(ts *TableSpec, rowCount int, generated map[string][]map[string]interface{}) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		row := make(map[string]interface{})
		for _, col := range ts.Columns {
			v := generateCell(ts, &col, i, rowCount, row, generated)
			row[col.Name] = v
		}
		rows = append(rows, row)
	}
	return rows
}

func generateCell(ts *TableSpec, col *ColumnSpec, rowIndex, rowCount int, currentRow map[string]interface{}, generated map[string][]map[string]interface{}) interface{} {
	lowerName := strings.ToLower(col.Name)
	lowerType := strings.ToLower(col.Type)

	// 1) Primary key (integer): use 1-based row index so FKs can reference these IDs
	if isPrimaryKey(ts, col.Name) && isIntegerType(lowerType) {
		return int64(rowIndex + 1)
	}

	// 2) Foreign key: pick random from parent's generated column
	for _, fk := range ts.Relationships {
		if fk.ChildColumn != col.Name {
			continue
		}
		parentRows := generated[fk.ParentTable]
		if len(parentRows) == 0 {
			return nil
		}
		idx := rand.Intn(len(parentRows))
		return parentRows[idx][fk.ParentColumn]
	}

	// 3) Nullable with null_percentage
	if col.Nullable && col.Stats != nil && col.Stats.NullPercentage > 0 {
		if rand.Float64()*100 < col.Stats.NullPercentage {
			return nil
		}
	}

	// 4) PII-like columns: gofakeit
	if isPII(lowerName) {
		return generatePII(lowerName, lowerType)
	}

	// 5) Numeric/Date: uniform between min and max
	if col.Stats != nil {
		if col.Stats.Min != nil && col.Stats.Max != nil {
			return uniformFromStats(col.Stats, lowerType)
		}
	}

	// 6) Default by type
	return defaultByType(lowerType, col.Name)
}

func isPrimaryKey(ts *TableSpec, colName string) bool {
	for _, pk := range ts.PrimaryKey {
		if pk == colName {
			return true
		}
	}
	return false
}

func isIntegerType(dataType string) bool {
	return strings.Contains(dataType, "int") && !strings.Contains(dataType, "interval")
}

func isPII(colName string) bool {
	pii := []string{"name", "firstname", "first_name", "lastname", "last_name", "email", "username", "phone", "address", "city", "country"}
	for _, p := range pii {
		if strings.Contains(colName, p) {
			return true
		}
	}
	return false
}

func generatePII(colName, colType string) interface{} {
	if strings.Contains(colName, "email") {
		return gofakeit.Email()
	}
	if strings.Contains(colName, "phone") {
		return gofakeit.Phone()
	}
	if strings.Contains(colName, "address") {
		return gofakeit.Street() + ", " + gofakeit.City()
	}
	if strings.Contains(colName, "city") {
		return gofakeit.City()
	}
	if strings.Contains(colName, "country") {
		return gofakeit.Country()
	}
	if strings.Contains(colName, "name") || strings.Contains(colName, "first") || strings.Contains(colName, "last") {
		return gofakeit.Name()
	}
	if strings.Contains(colName, "username") {
		return gofakeit.Username()
	}
	return gofakeit.Word()
}

func uniformFromStats(stats *ColumnStats, dataType string) interface{} {
	min, max := stats.Min, stats.Max
	switch m := min.(type) {
	case int64:
		if mx, ok := max.(int64); ok {
			if m == mx {
				return m
			}
			u := rand.Float64()
			span := float64(mx - m)
			return m + int64(u*span)
		}
	case float64:
		if mx, ok := max.(float64); ok {
			if m == mx {
				return m
			}
			u := rand.Float64()
			return m + u*(mx-m)
		}
	}
	// string (e.g. date): treat as comparable; for V1 return min or random between if we parse
	return min
}

func defaultByType(dataType, colName string) interface{} {
	switch {
	case strings.Contains(dataType, "int"):
		return int64(rand.Intn(10000))
	case strings.Contains(dataType, "float") || strings.Contains(dataType, "double") || strings.Contains(dataType, "numeric") || strings.Contains(dataType, "real"):
		return rand.Float64() * 10000
	case strings.Contains(dataType, "bool"):
		return rand.Float32() < 0.5
	case strings.Contains(dataType, "date") || strings.Contains(dataType, "time"):
		return gofakeit.Date().Format("2006-01-02")
	default:
		return gofakeit.Word()
	}
}
