package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// RowReader yields rows as map[string]interface{} one at a time. When no more
// rows, Next returns (nil, io.EOF).
type RowReader interface {
	Next() (map[string]interface{}, error)
}

// DiffReport holds the result of comparing two sources on a primary key.
type DiffReport struct {
	OnlyInS1      []map[string]interface{}            `json:"only_in_s1"`
	OnlyInS2      []map[string]interface{}             `json:"only_in_s2"`
	Discrepancies map[string]map[string]FieldDiff      `json:"discrepancies"` // keyed by primary key value
}

// FieldDiff describes a single field mismatch.
type FieldDiff struct {
	Key   string      `json:"key"`
	Val1  interface{} `json:"val1"`
	Val2  interface{} `json:"val2"`
}

// CompareSources loads all rows from s1 keyed by primaryKey, then streams s2.
// OnlyInS2: keys in s2 not in s1; Discrepancies: same key, different fields; OnlyInS1: keys left in s1 after s2 is exhausted.
// ignore is a set of field names to exclude from comparison (e.g. updated_at).
func CompareSources(s1, s2 RowReader, primaryKey string, ignore map[string]bool) (*DiffReport, error) {
	report := &DiffReport{
		OnlyInS1:      nil,
		OnlyInS2:      nil,
		Discrepancies: make(map[string]map[string]FieldDiff),
	}
	if primaryKey == "" {
		return nil, fmt.Errorf("primary key is required")
	}

	// Load all rows from s1 into map keyed by primaryKey
	s1Map := make(map[string]map[string]interface{})
	for {
		row, err := s1.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("s1 read: %w", err)
		}
		k := keyString(row[primaryKey])
		s1Map[k] = row
	}

	// Stream s2 row-by-row
	for {
		row, err := s2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("s2 read: %w", err)
		}
		k := keyString(row[primaryKey])
		s1Row, exists := s1Map[k]
		if !exists {
			report.OnlyInS2 = append(report.OnlyInS2, row)
			continue
		}
		// Deep compare; record field-level differences (excluding primaryKey and ignore set)
		diffs := compareRows(s1Row, row, primaryKey, ignore)
		if len(diffs) > 0 {
			report.Discrepancies[k] = diffs
		}
		delete(s1Map, k)
	}

	// Remaining in s1Map are only in s1
	for _, row := range s1Map {
		report.OnlyInS1 = append(report.OnlyInS1, row)
	}
	return report, nil
}

func keyString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprint(v)
	}
}

func compareRows(r1, r2 map[string]interface{}, primaryKey string, ignore map[string]bool) map[string]FieldDiff {
	out := make(map[string]FieldDiff)
	allKeys := make(map[string]bool)
	for k := range r1 {
		allKeys[k] = true
	}
	for k := range r2 {
		allKeys[k] = true
	}
	for k := range allKeys {
		if k == primaryKey || ignore[k] {
			continue
		}
		v1, v2 := r1[k], r2[k]
		if !valuesEqual(v1, v2) {
			out[k] = FieldDiff{Key: k, Val1: v1, Val2: v2}
		}
	}
	return out
}

func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Normalize number types for comparison
	af := toFloat(a)
	bf := toFloat(b)
	if af != nil && bf != nil {
		return *af == *bf
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case float32:
		f := float64(x)
		return &f
	case int:
		f := float64(x)
		return &f
	case int64:
		f := float64(x)
		return &f
	case int32:
		f := float64(x)
		return &f
	default:
		return nil
	}
}

// Closer is implemented by readers that need to release resources.
type Closer interface {
	Close() error
}

// CloseRowReader calls Close on r if it implements Closer.
func CloseRowReader(r RowReader) {
	if c, ok := r.(Closer); ok {
		_ = c.Close()
	}
}

// DiffReportJSON returns the report as JSON bytes.
func DiffReportJSON(r *DiffReport) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
