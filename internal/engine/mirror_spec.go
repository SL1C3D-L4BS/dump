// Package engine: MirrorSpec describes schema and statistics for synthetic data mirroring.

package engine

// MirrorSpec is the YAML-friendly structure for a database mirror: tables, column stats, and relationships.
type MirrorSpec struct {
	Tables []TableSpec `yaml:"tables"`
}

// TableSpec describes one table: name, columns (with types and stats), and foreign keys.
type TableSpec struct {
	Name          string         `yaml:"name"`
	Columns       []ColumnSpec   `yaml:"columns"`
	PrimaryKey    []string       `yaml:"primary_key,omitempty"`
	Relationships []ForeignKey   `yaml:"relationships,omitempty"`
}

// ColumnSpec is a column with name, type, nullability, and optional statistics.
type ColumnSpec struct {
	Name     string       `yaml:"name"`
	Type     string       `yaml:"type"`
	Nullable bool         `yaml:"nullable"`
	Stats    *ColumnStats `yaml:"stats,omitempty"`
}

// ColumnStats holds min, max, count, null_percentage, and cardinality for a column.
type ColumnStats struct {
	Min            interface{} `yaml:"min,omitempty"`
	Max            interface{} `yaml:"max,omitempty"`
	Count          int64       `yaml:"count"`
	NullCount      int64       `yaml:"null_count"`
	NullPercentage float64     `yaml:"null_percentage"`
	Cardinality    int64       `yaml:"cardinality"`
}

// ForeignKey describes a child column referencing a parent table/column.
type ForeignKey struct {
	ChildColumn   string `yaml:"child_column"`
	ParentTable   string `yaml:"parent_table"`
	ParentColumn  string `yaml:"parent_column"`
}
