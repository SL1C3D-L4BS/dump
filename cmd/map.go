package cmd

import (
	"fmt"
	"os"
	"time"

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
)

var mapCmd = &cobra.Command{
	Use:   "map [input file]",
	Short: "Map input data using a YAML schema (streaming)",
	Long:  `Streams JSONL from the input file, applies the schema mapping, and writes JSONL or Parquet to stdout/output. Reports performance and Vericore seal to stderr.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runMap,
}

func init() {
	mapCmd.Flags().StringVar(&mapSchemaPath, "schema", "", "Path to the YAML mapping schema (required)")
	mapCmd.Flags().StringVar(&mapFormat, "format", "jsonl", "Output format: jsonl or parquet")
	mapCmd.Flags().StringVar(&mapOutput, "output", "", "Output file path (required for parquet; default stdout for jsonl)")
	_ = mapCmd.MarkFlagRequired("schema")
}

func runMap(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	if mapFormat == "parquet" && mapOutput == "" {
		return fmt.Errorf("--output is required when --format=parquet")
	}

	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

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

	start := time.Now()
	rows, err := engine.MapStream(f, schema, sink)
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
