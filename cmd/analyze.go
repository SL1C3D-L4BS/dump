package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/internal/inference"
	"github.com/spf13/cobra"
)

var (
	analyzeTarget  string
	analyzeModel   string
	analyzeDialect string
)

const analyzePeekSize = 1024
const analyzeSampleRows = 10

var analyzeCmd = &cobra.Command{
	Use:   "analyze [file]",
	Short: "Detect format and infer a mapping schema from a mystery file (zero-knowledge)",
	Long:  `Peek at the file, heuristically detect format (xml, jsonl, edi, csv), sample it, and use the local LLM to infer semantics and produce a DUMP YAML mapping.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&analyzeTarget, "target", "parquet", "Target schema format (e.g. parquet, protobuf)")
	analyzeCmd.Flags().StringVar(&analyzeModel, "model", "llama3", "Ollama model name")
	analyzeCmd.Flags().StringVar(&analyzeDialect, "dialect", "", "Path to dialect YAML (optional, for EDI)")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// 1. Peek first 1024 bytes for format detection
	br := bufio.NewReader(f)
	peek, err := br.Peek(analyzePeekSize)
	if err != nil && len(peek) == 0 {
		return fmt.Errorf("read file: %w", err)
	}
	format := engine.DetectFormat(peek)
	fmt.Fprintf(os.Stderr, "🔍 Format detected: %s\n", format)
	if format == "unknown" {
		fmt.Fprintf(os.Stderr, "   (sampling as JSONL; use --input-type with dump map if format is known)\n")
	}

	// 2. Reset and pass full file to sampler (re-open so we don't rely on unread buffer)
	if err := f.Close(); err != nil {
		return err
	}
	f2, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen file: %w", err)
	}
	defer f2.Close()

	sample, err := engine.ExtractSample(f2, format, analyzeDialect, analyzeSampleRows)
	if err != nil {
		return fmt.Errorf("extract sample: %w", err)
	}
	if sample == "[]" || len(sample) < 3 {
		return fmt.Errorf("no rows could be sampled (format %q may be wrong or file malformed)", format)
	}

	// 3. Call Ollama
	inf := inference.NewSchemaInferencer("", &http.Client{Timeout: 5 * time.Minute})
	yamlOut, err := inf.AnalyzeAndInfer(sample, analyzeTarget, analyzeModel)
	if err != nil {
		return err
	}

	fmt.Print(yamlOut)
	return nil
}
