package engine

import (
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		peek     []byte
		expected string
	}{
		{"jsonl object", []byte(`{"a":1}`), "jsonl"},
		{"jsonl array", []byte(`[{"a":1}]`), "jsonl"},
		{"xml declaration", []byte(`<?xml version="1.0"?>`), "xml"},
		{"xml element", []byte("<Record><id>1</id></Record>"), "xml"},
		{"edi HL7", []byte("MSH|SEND|HOSP"), "edi"},
		{"edi X12", []byte("ISA*00*0000000000*01*123456789"), "edi"},
		{"csv comma", []byte("id,name,role\n1,Alice,admin"), "csv"},
		{"csv pipe", []byte("id|name|role\n1|Alice|admin"), "csv"},
		{"empty", []byte(""), "unknown"},
		{"unknown", []byte("plain text"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.peek)
			if got != tt.expected {
				t.Errorf("DetectFormat() = %q, want %q", got, tt.expected)
			}
		})
	}
}
