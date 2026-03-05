package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/internal/integrity"
	"github.com/spf13/cobra"
)

const violetANSI = "\033[35m"
const resetANSI = "\033[0m"

var (
	mapSchemaPath string
	mapFormat     string
	mapOutput     string
	mapDBURL      string
	mapQuery      string
	mapInputType  string
	mapDialect    string
	mapXMLBlock   string
	mapMask       string
)

var mapCmd = &cobra.Command{
	Use:   "map [input file]",
	Short: "Map input data using a YAML schema (streaming)",
	Long:  `Streams from a file (JSONL/CSV), or a SQL query when --db-url is set, applies the schema mapping, and writes JSONL or Parquet. Reports performance and Vericore seal to stderr.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runMap,
}

func init() {
	mapCmd.Flags().StringVar(&mapSchemaPath, "schema", "", "Path to the YAML mapping schema (required)")
	mapCmd.Flags().StringVar(&mapFormat, "format", "jsonl", "Output format: jsonl or parquet")
	mapCmd.Flags().StringVar(&mapOutput, "output", "", "Output file path (required for parquet; default stdout for jsonl)")
	mapCmd.Flags().StringVar(&mapDBURL, "db-url", "", "Database URL (postgres://... or file:path.db) to query instead of file input")
	mapCmd.Flags().StringVar(&mapQuery, "query", "SELECT * FROM users", "SQL query to run when --db-url is set")
	mapCmd.Flags().StringVar(&mapInputType, "input-type", "", "Override input type: jsonl, csv, xml, edi, x12, or fhir (default: auto from file extension)")
	mapCmd.Flags().StringVar(&mapDialect, "dialect", "", "Path to dialect YAML (required when input-type is edi or x12)")
	mapCmd.Flags().StringVar(&mapXMLBlock, "xml-block", "Record", "Repeating XML element name for xml input-type")
	mapCmd.Flags().StringVar(&mapMask, "mask", "", "Enable semantic masking (e.g. pii to anonymize PII in the output stream)")
	_ = mapCmd.MarkFlagRequired("schema")
}

func runMap(cmd *cobra.Command, args []string) error {
	if mapFormat == "parquet" && mapOutput == "" {
		return fmt.Errorf("--output is required when --format=parquet")
	}
	if mapFormat == "fhir" || mapInputType == "fhir" {
		fmt.Fprintf(os.Stderr, "%s⚕️ SL1C3D-L4BS FHIR Adapter Active: Bridging HL7/JSON to FHIR Bundles.%s\n", violetANSI, resetANSI)
	}

	var in io.ReadCloser
	if mapDBURL != "" {
		sqlReader, err := engine.NewSQLReader(mapDBURL, mapQuery)
		if err != nil {
			return fmt.Errorf("sql source: %w", err)
		}
		defer sqlReader.Close()
		in = sqlReader
	} else {
		if len(args) < 1 {
			return fmt.Errorf("input file required when not using --db-url")
		}
		inputPath := args[0]
		f, err := os.Open(inputPath)
		if err != nil {
			return fmt.Errorf("open input: %w", err)
		}
		defer f.Close()
		typ := mapInputType
		if typ == "" {
			switch {
			case strings.HasSuffix(strings.ToLower(inputPath), ".csv"):
				typ = "csv"
			case strings.HasSuffix(strings.ToLower(inputPath), ".xml"):
				typ = "xml"
			case strings.HasSuffix(strings.ToLower(inputPath), ".edi"), strings.HasSuffix(strings.ToLower(inputPath), ".hl7"):
				typ = "edi"
			case strings.HasSuffix(strings.ToLower(inputPath), ".x12"):
				typ = "x12"
			case strings.HasSuffix(strings.ToLower(inputPath), ".json"):
				// Could be FHIR Bundle or plain JSON; default jsonl unless user says fhir
				if mapInputType == "fhir" {
					typ = "fhir"
				} else {
					typ = "jsonl"
				}
			default:
				typ = "jsonl"
			}
			if mapInputType == "fhir" {
				typ = "fhir"
			}
		}
		switch typ {
		case "csv":
			in = io.NopCloser(engine.NewCSVReader(f))
		case "xml":
			in = io.NopCloser(engine.NewXMLReader(f, mapXMLBlock))
		case "edi":
			if mapDialect == "" {
				return fmt.Errorf("--dialect is required when --input-type is edi")
			}
			dialect, err := dialects.LoadDialect(mapDialect)
			if err != nil {
				return fmt.Errorf("load dialect: %w", err)
			}
			in = io.NopCloser(engine.NewEDIReader(f, dialect))
		case "x12":
			if mapDialect == "" {
				return fmt.Errorf("--dialect is required when --input-type is x12")
			}
			dialect, err := dialects.LoadDialect(mapDialect)
			if err != nil {
				return fmt.Errorf("load dialect: %w", err)
			}
			fmt.Fprintf(os.Stderr, "%s🏥 X12 Stateful Parsing Active: Tracking hierarchical loops for billing reconciliation.%s\n", violetANSI, resetANSI)
			in = io.NopCloser(engine.NewX12Reader(f, dialect))
		case "fhir":
			in = io.NopCloser(engine.NewFHIRReader(f))
		default:
			in = f
		}
	}

	schema, err := engine.LoadSchema(mapSchemaPath)
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	var outPath string
	var sink engine.RowSink
	var closer interface{ Close() error }

	if mapFormat == "parquet" {
		outFile, err := os.Create(mapOutput)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		defer outFile.Close()
		outPath = mapOutput
		pw, err := engine.NewParquetWriter(outFile, schema)
		if err != nil {
			return fmt.Errorf("parquet writer: %w", err)
		}
		sink = pw
		closer = pw
	} else if mapFormat == "fhir" {
		if mapOutput != "" {
			outFile, err := os.Create(mapOutput)
			if err != nil {
				return fmt.Errorf("create output: %w", err)
			}
			defer outFile.Close()
			outPath = mapOutput
			fw := engine.NewFHIRWriter(outFile)
			sink = fw
			closer = fw
		} else {
			fw := engine.NewFHIRWriter(os.Stdout)
			sink = fw
			closer = fw
		}
	} else {
		if mapOutput != "" {
			outFile, err := os.Create(mapOutput)
			if err != nil {
				return fmt.Errorf("create output: %w", err)
			}
			defer outFile.Close()
			outPath = mapOutput
			sink = engine.JSONLWriter{W: outFile}
		} else {
			sink = engine.JSONLWriter{W: os.Stdout}
		}
	}

	if mapMask == "pii" {
		sink = &engine.MaskingSink{Underlying: sink}
		fmt.Fprintf(os.Stderr, "🛡️  Semantic Masking Enabled: PII will be anonymized in the output stream.\n")
	}

	start := time.Now()
	rows, err := engine.MapStream(in, schema, sink)
	if err != nil {
		return fmt.Errorf("map stream: %w", err)
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("close writer: %w", err)
		}
	}
	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "Mapped %d rows in %s\n", rows, elapsed.Round(time.Millisecond))

	if outPath != "" {
		seal, err := integrity.SignResult(outPath)
		if err != nil {
			return fmt.Errorf("sign result: %w", err)
		}
		fmt.Fprintf(os.Stderr, "%s%s%s\n", violetANSI, seal, resetANSI)
	}
	return nil
}
