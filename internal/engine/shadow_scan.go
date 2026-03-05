// Package engine: Shadow IT scan — row count, PII density, schema complexity for data files.

package engine

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ShadowFileResult holds the result of a silent analyze on one data file.
type ShadowFileResult struct {
	Path              string  `json:"path"`
	Format            string  `json:"format"`
	RowCount          int64   `json:"row_count"`
	PIIDensity        float64 `json:"pii_density"`         // 0..1, fraction of fields that look like PII
	SchemaComplexity  int     `json:"schema_complexity"`   // number of columns/keys
	SuggestedCommand  string  `json:"suggested_command"`   // dump map command for Vericore migration
}

// ScanDataFile performs a silent analyze on path: row count, PII density, schema complexity.
// Supported extensions: .csv, .xlsx, .jsonl, .edi. Returns error if format unsupported or unreadable.
func ScanDataFile(path string, vericoreStoreDir string) (*ShadowFileResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var format string
	switch ext {
	case ".csv":
		format = "csv"
	case ".xlsx":
		format = "xlsx"
	case ".jsonl", ".json":
		format = "jsonl"
	case ".edi", ".hl7", ".x12":
		format = "edi"
	default:
		return nil, fmt.Errorf("unsupported extension %s", ext)
	}

	rowCount, err := countRows(path, format)
	if err != nil {
		return nil, fmt.Errorf("row count: %w", err)
	}

	piiDensity, complexity, err := sampleStats(path, format, 50)
	if err != nil {
		piiDensity = 0
		complexity = 0
	}

	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if vericoreStoreDir == "" {
		vericoreStoreDir = "./vericore_ingest"
	}
	outPath := filepath.Join(vericoreStoreDir, stem+".parquet")
	var suggested string
	if format == "xlsx" {
		suggested = fmt.Sprintf("dump map <exported.csv> --input-type csv --schema <inferred.yaml> --output %q  # export XLSX to CSV first, then migrate to Vericore", outPath)
	} else {
		suggested = fmt.Sprintf("dump map %q --input-type %s --schema <inferred.yaml> --output %q  # migrate to Vericore central store", path, format, outPath)
	}

	return &ShadowFileResult{
		Path:             path,
		Format:           format,
		RowCount:         rowCount,
		PIIDensity:       piiDensity,
		SchemaComplexity: complexity,
		SuggestedCommand: suggested,
	}, nil
}

func countRows(path, format string) (int64, error) {
	switch format {
	case "xlsx":
		f, err := excelize.OpenFile(path)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		sheet := f.GetSheetList()[0]
		rows, err := f.GetRows(sheet)
		if err != nil {
			return 0, err
		}
		n := int64(len(rows)) - 1
		if n < 0 {
			n = 0
		}
		return n, nil
	case "csv":
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		r := csv.NewReader(f)
		var count int64
		for {
			_, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return 0, err
			}
			count++
		}
		if count > 0 {
			count-- // header
		}
		return count, nil
	case "jsonl":
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		var count int64
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) > 0 && line[0] == '{' {
				count++
			}
		}
		return count, scanner.Err()
	case "edi":
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		var count int64
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if len(bytes.TrimSpace(scanner.Bytes())) > 0 {
				count++
			}
		}
		return count, scanner.Err()
	default:
		return 0, fmt.Errorf("unsupported format %s", format)
	}
}

func sampleStats(path, format string, maxSample int) (piiDensity float64, schemaComplexity int, err error) {
	var rr RowReader
	switch format {
	case "xlsx":
		rr, err = NewExcelReader(path, "")
	case "jsonl":
		rr, err = NewJSONLRowReader(path)
	case "csv":
		rr, err = NewCSVRowReader(path)
	case "edi":
		f, oerr := os.Open(path)
		if oerr != nil {
			err = oerr
			return 0, 0, err
		}
		defer f.Close()
		dialect, _ := dialectForEDI("")
		sample, serr := ExtractSampleWithDialect(f, "edi", "", maxSample, dialect)
		if serr != nil {
			err = serr
			return 0, 0, err
		}
		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(sample), &rows); err != nil {
			return 0, 0, err
		}
		return computePIIAndComplexity(rows)
	default:
		return 0, 0, fmt.Errorf("unsupported format %s", format)
	}
	if err != nil {
		return 0, 0, err
	}
	if closer, ok := rr.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	var rows []map[string]interface{}
	for i := 0; i < maxSample; i++ {
		row, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		rows = append(rows, row)
	}
	return computePIIAndComplexity(rows)
}

func computePIIAndComplexity(rows []map[string]interface{}) (piiDensity float64, schemaComplexity int, err error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}
	for _, row := range rows {
		if n := countKeys(row); n > schemaComplexity {
			schemaComplexity = n
		}
	}
	var totalFields int
	var maskedFields int
	for _, row := range rows {
		masked := MaskRow(row)
		total, changed := countChangedFields(row, masked)
		totalFields += total
		maskedFields += changed
	}
	if totalFields == 0 {
		return 0, schemaComplexity, nil
	}
	piiDensity = float64(maskedFields) / float64(totalFields)
	return piiDensity, schemaComplexity, nil
}

func countKeys(m map[string]interface{}) int {
	n := 0
	for _, v := range m {
		if sub, ok := v.(map[string]interface{}); ok {
			n += countKeys(sub)
		} else {
			n++
		}
	}
	return n
}

func countChangedFields(a, b map[string]interface{}) (total int, changed int) {
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			total++
			changed++
			continue
		}
		if subA, okA := va.(map[string]interface{}); okA {
			if subB, okB := vb.(map[string]interface{}); okB {
				t, c := countChangedFields(subA, subB)
				total += t
				changed += c
			} else {
				total++
				changed++
			}
		} else {
			total++
			if !reflect.DeepEqual(va, vb) {
				changed++
			}
		}
	}
	return total, changed
}
