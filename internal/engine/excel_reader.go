package engine

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelReader implements RowReader by reading an XLSX file. The first row is
// treated as headers; each subsequent row is returned as map[string]interface{}
// with header values as keys.
type ExcelReader struct {
	f      *excelize.File
	sheet  string
	rows   [][]string
	rowIdx int
}

// NewExcelReader opens the Excel file at path and reads from the given sheet.
// If sheet is empty, the first sheet is used.
func NewExcelReader(path string, sheet string) (*ExcelReader, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open excel: %w", err)
	}
	if sheet == "" {
		sheet = f.GetSheetList()[0]
		if sheet == "" {
			f.Close()
			return nil, fmt.Errorf("no sheets in workbook")
		}
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("get rows: %w", err)
	}
	return &ExcelReader{f: f, sheet: sheet, rows: rows, rowIdx: 0}, nil
}

// Next returns the next row as a map keyed by the header row. Returns (nil, io.EOF) when done.
func (r *ExcelReader) Next() (map[string]interface{}, error) {
	if len(r.rows) == 0 {
		return nil, io.EOF
	}
	if r.rowIdx == 0 {
		r.rowIdx = 1
		// First row is headers; we need at least one data row
		if len(r.rows) < 2 {
			return nil, io.EOF
		}
	}
	if r.rowIdx >= len(r.rows) {
		return nil, io.EOF
	}
	headers := r.rows[0]
	dataRow := r.rows[r.rowIdx]
	r.rowIdx++
	out := make(map[string]interface{}, len(headers))
	for i, h := range headers {
		key := strings.TrimSpace(h)
		if key == "" {
			key = "col_" + strconv.Itoa(i)
		}
		val := ""
		if i < len(dataRow) {
			val = strings.TrimSpace(dataRow[i])
		}
		out[key] = val
	}
	return out, nil
}

// Close releases the workbook file.
func (r *ExcelReader) Close() error {
	if r.f != nil {
		return r.f.Close()
	}
	return nil
}
