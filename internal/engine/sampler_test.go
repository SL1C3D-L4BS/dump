package engine

import (
	"strings"
	"testing"
)

func TestExtractSample_JSONL(t *testing.T) {
	r := strings.NewReader(`{"a":1}
{"a":2}
{"a":3}`)
	out, err := ExtractSample(r, "jsonl", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" || out == "[]" {
		t.Errorf("expected non-empty JSON array, got %q", out)
	}
	if !strings.Contains(out, `"a":1`) || !strings.Contains(out, `"a":2`) {
		t.Errorf("expected two sampled rows in output, got %q", out)
	}
}

func TestExtractSample_CSV(t *testing.T) {
	r := strings.NewReader("k,v\n1,a\n2,b\n3,c")
	out, err := ExtractSample(r, "csv", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" || out == "[]" {
		t.Errorf("expected non-empty JSON array, got %q", out)
	}
	if !strings.Contains(out, "k") || !strings.Contains(out, "v") {
		t.Errorf("expected CSV headers as keys, got %q", out)
	}
}

func TestExtractSample_Unknown(t *testing.T) {
	r := strings.NewReader(`{"x":1}`)
	out, err := ExtractSample(r, "unknown", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	// unknown is treated as jsonl, so we should get one row
	if out == "" {
		t.Errorf("expected best-effort sample, got %q", out)
	}
}
