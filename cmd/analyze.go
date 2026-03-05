package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/internal/inference"
	"github.com/SL1C3D-L4BS/dump/pkg/healthcare"
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
	if industryFlag == "healthcare" {
		fmt.Fprintf(os.Stderr, "%s🏥 Industry Mode: Healthcare. Standard HL7/X12 protocols engaged.%s\n", violetANSI, resetANSI)
	}
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
	if industryFlag == "healthcare" && format == "unknown" {
		format = "edi"
	}
	fmt.Fprintf(os.Stderr, "🔍 Format detected: %s\n", format)
	if format == "unknown" {
		fmt.Fprintf(os.Stderr, "   (sampling as JSONL; use --input-type with dump map if format is known)\n")
	}

	// 2. EDI/X12: check for custom (Z-) segments and run Acronym Resolver if needed
	var mergedDialect *dialects.Dialect
	if format == "edi" {
		standardName := "hl7_v25"
		if bytes.Contains(peek, []byte("ISA*")) {
			standardName = "x12_837"
		}
		baseDialect, err := healthcare.LoadStandardDialect(standardName)
		if err != nil {
			return fmt.Errorf("load standard dialect: %w", err)
		}
		if err := f.Close(); err != nil {
			return err
		}
		fRaw, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("reopen file for EDI scan: %w", err)
		}
		rawLines, err := engine.ExtractRawEDILines(fRaw, 500)
		fRaw.Close()
		if err != nil {
			return fmt.Errorf("extract raw EDI lines: %w", err)
		}
		segmentIDs := engine.SegmentIDsFromRawEDILines(rawLines)
		var unknown []string
		for id := range segmentIDs {
			if baseDialect.Segments == nil || baseDialect.Segments[id] == nil {
				unknown = append(unknown, id)
			}
		}
		// Route to Acronym Resolver when any segment is not in the standard (e.g. Z-segments).
		if len(unknown) > 0 {
			fmt.Fprintf(os.Stderr, "%s🧠 SL1C3D-L4BS Acronym Resolver Active: Auto-generating custom dialect for undocumented legacy segments.%s\n", violetANSI, resetANSI)
			inf := inference.NewSchemaInferencer("", &http.Client{Timeout: 5 * time.Minute})
			rawSample := strings.Join(rawLines, "\n")
			customYAML, err := inference.InferCustomDialect(inf, rawSample, baseDialect, analyzeModel)
			if err != nil {
				return fmt.Errorf("infer custom dialect: %w", err)
			}
			if err := os.WriteFile("custom_dialect.yaml", []byte(customYAML), 0644); err != nil {
				return fmt.Errorf("write custom_dialect.yaml: %w", err)
			}
			customDialect, err := dialects.ParseDialect([]byte(customYAML))
			if err != nil {
				return fmt.Errorf("parse generated custom dialect: %w", err)
			}
			mergedDialect = &dialects.Dialect{
				Name:                baseDialect.Name,
				MessageStartSegment: baseDialect.MessageStartSegment,
				Segments:            make(map[string][]string),
				Delimiters:          baseDialect.Delimiters,
				TransactionBoundary: baseDialect.TransactionBoundary,
				LoopTriggers:        baseDialect.LoopTriggers,
				YieldTrigger:        baseDialect.YieldTrigger,
			}
			if baseDialect.Segments != nil {
				for k, v := range baseDialect.Segments {
					mergedDialect.Segments[k] = v
				}
			}
			mergedDialect.MergeDialect(customDialect)
		} else if mergedDialect == nil {
			mergedDialect = baseDialect
		}
	}

	// 3. Reset and pass full file to sampler (re-open so we don't rely on unread buffer)
	if mergedDialect == nil {
		if err := f.Close(); err != nil {
			return err
		}
	}
	f2, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen file: %w", err)
	}
	defer f2.Close()

	dialectPath := analyzeDialect
	if industryFlag == "healthcare" && dialectPath == "" && mergedDialect == nil && format == "edi" {
		standardName := "hl7_v25"
		if bytes.Contains(peek, []byte("ISA*")) {
			standardName = "x12_837"
		}
		stdDialect, err := healthcare.LoadStandardDialect(standardName)
		if err == nil {
			mergedDialect = stdDialect
		}
	}
	sample, err := engine.ExtractSampleWithDialect(f2, format, dialectPath, analyzeSampleRows, mergedDialect)
	if err != nil {
		return fmt.Errorf("extract sample: %w", err)
	}
	if sample == "[]" || len(sample) < 3 {
		return fmt.Errorf("no rows could be sampled (format %q may be wrong or file malformed)", format)
	}

	// 4. Call Ollama
	inf := inference.NewSchemaInferencer("", &http.Client{Timeout: 5 * time.Minute})
	yamlOut, err := inf.AnalyzeAndInfer(sample, analyzeTarget, analyzeModel)
	if err != nil {
		return err
	}

	fmt.Print(yamlOut)
	return nil
}
