// Performance suite for SL1C3D-L4BS: O(1) memory stability and throughput validation.

package engine

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SL1C3D-L4BS/dump/internal/tools"
	"github.com/SL1C3D-L4BS/dump/pkg/healthcare"
)

const (
	maxHeapMB       = 150
	sampleEveryRows = 50_000
)

// TestX12LargeStreamMemory verifies that HeapAlloc remains under 150MB (constant)
// while processing the full X12 837 stress stream. Uses runtime.ReadMemStats.
// Consumes the stream via Read() to drive the X12Reader state machine.
func TestX12LargeStreamMemory(t *testing.T) {
	var targetSize int64 = tools.TargetStressSize
	if testing.Short() {
		targetSize = 50 * 1024 * 1024 // 50MB for short mode
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "stress_837.x12")
	t.Logf("Generating X12 837 stress file (~%d MB)...", targetSize/(1024*1024))
	written, _, err := tools.WriteStressFile(path, targetSize)
	if err != nil {
		t.Fatalf("generate stress file: %v", err)
	}
	t.Logf("Generated %d bytes", written)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stress file: %v", err)
	}
	defer f.Close()

	dialect, err := healthcare.LoadStandardDialect("x12_837")
	if err != nil {
		t.Fatalf("load dialect: %v", err)
	}
	if dialect.YieldTrigger.Segment == "" {
		dialect.YieldTrigger.Segment = "CLM"
	}

	reader := healthcare.NewX12Reader(f, dialect)

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	heapStartMB := m1.HeapAlloc / (1024 * 1024)

	rows := int64(0)
	sampleRow := int64(sampleEveryRows)
	buf := make([]byte, 64*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' {
					rows++
					if rows%sampleRow == 0 {
						var m runtime.MemStats
						runtime.ReadMemStats(&m)
						heapMB := m.HeapAlloc / (1024 * 1024)
						if heapMB > maxHeapMB {
							t.Errorf("HeapAlloc %d MB exceeds %d MB at row %d (O(1) memory violation)", heapMB, maxHeapMB, rows)
						}
						t.Logf("Row %d: HeapAlloc %d MB", rows, heapMB)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	heapEndMB := m2.HeapAlloc / (1024 * 1024)
	t.Logf("Processed %d rows; Heap start %d MB, end %d MB", rows, heapStartMB, heapEndMB)
	if heapEndMB > maxHeapMB {
		t.Errorf("Final HeapAlloc %d MB exceeds %d MB", heapEndMB, maxHeapMB)
	}
}

// consumeX12Reader fully drains the X12 reader (via Read) and returns rows read and any error.
func consumeX12Reader(r io.Reader) (rows int64, err error) {
	buf := make([]byte, 64*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			// Count newlines as approximate row count
			for _, b := range buf[:n] {
				if b == '\n' {
					rows++
				}
			}
		}
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
	}
}

// Minimal schema for pipeline benchmark: one pass-through rule.
var benchSchema = &Schema{
	Rules: []MappingRule{
		{SourcePath: "CLM.ClaimSubmitterIdentifier", TargetField: "id", Type: "string"},
		{SourcePath: "CLM.MonetaryAmount", TargetField: "amount", Type: "string"},
	},
}

// discardSink is a RowSink that discards rows (for throughput measurement).
type discardSink struct{}

func (d discardSink) WriteRow(row []byte) error { return nil }

// BenchmarkMapPipeline measures rows per second for JSONL -> MapStream -> sink.
func BenchmarkMapPipeline(b *testing.B) {
	// Generate small JSONL in memory (one row per claim-like structure)
	row := []byte(`{"CLM":{"ClaimSubmitterIdentifier":"CLM001","MonetaryAmount":"100.00"}}` + "\n")
	const numRows = 100_000
	var buf bytes.Buffer
	for i := 0; i < numRows; i++ {
		buf.Write(row)
	}
	jsonl := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in := bytes.NewReader(jsonl)
		sink := discardSink{}
		n, err := MapStream(in, benchSchema, sink)
		if err != nil {
			b.Fatal(err)
		}
		if n != numRows {
			b.Fatalf("expected %d rows, got %d", numRows, n)
		}
	}
}

// BenchmarkMapPipelineParquet measures rows/sec for JSONL -> MapStream -> Parquet (no disk).
func BenchmarkMapPipelineParquet(b *testing.B) {
	row := []byte(`{"CLM":{"ClaimSubmitterIdentifier":"CLM001","MonetaryAmount":"100.00"}}` + "\n")
	const numRows = 10_000
	var buf bytes.Buffer
	for i := 0; i < numRows; i++ {
		buf.Write(row)
	}
	jsonl := buf.Bytes()

	for i := 0; i < b.N; i++ {
		in := bytes.NewReader(jsonl)
		out := bytes.NewBuffer(nil)
		pw, err := NewParquetWriter(out, benchSchema)
		if err != nil {
			b.Fatal(err)
		}
		n, err := MapStream(in, benchSchema, pw)
		if err != nil {
			b.Fatal(err)
		}
		if err := pw.Close(); err != nil {
			b.Fatal(err)
		}
		if n != numRows {
			b.Fatalf("expected %d rows, got %d", numRows, n)
		}
	}
}

// BenchmarkX12ReaderThroughput measures segments (yields) per second for X12Reader.
// Uses an in-memory stress buffer of fixed size (~600 KB per b.N iteration).
func BenchmarkX12ReaderThroughput(b *testing.B) {
	// One interchange ~600 bytes; 1024 interchanges ~600 KB for fast benchmark.
	var buf bytes.Buffer
	_, _, err := tools.GenStress837(&buf, 600*1024)
	if err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()

	dialect, err := healthcare.LoadStandardDialect("x12_837")
	if err != nil {
		b.Fatal(err)
	}
	if dialect.YieldTrigger.Segment == "" {
		dialect.YieldTrigger.Segment = "CLM"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		reader := healthcare.NewX12Reader(r, dialect)
		n, err := consumeX12Reader(reader)
		if err != nil {
			b.Fatal(err)
		}
		if n <= 0 {
			b.Fatal("expected positive row count")
		}
	}
}
