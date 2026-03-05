package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SL1C3D-L4BS/dump/internal/config"
	"github.com/SL1C3D-L4BS/dump/internal/dialects"
	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/spf13/cobra"
)

var (
	fanoutConfigPath string
	fanoutMask       string
)

var fanoutCmd = &cobra.Command{
	Use:   "fanout",
	Short: "Multiplex a single stream to multiple destinations (local, S3, Prometheus, Elasticsearch)",
	Long:  `Reads fan-out config (source, schema, targets), initializes all sinks, and pipes the stream through MapStream to each destination. Supports --mask=pii.`,
	Args:  cobra.NoArgs,
	RunE:  runFanout,
}

func init() {
	fanoutCmd.Flags().StringVar(&fanoutConfigPath, "config", "", "Path to fanout.yaml (required)")
	fanoutCmd.Flags().StringVar(&fanoutMask, "mask", "", "Enable semantic masking (e.g. pii) before fan-out")
	_ = fanoutCmd.MarkFlagRequired("config")
}

func runFanout(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadFanOutConfig(fanoutConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Open source
	f, err := os.Open(cfg.Source.Path)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer f.Close()

	var in io.Reader = f
	switch strings.ToLower(cfg.Source.Type) {
	case "csv":
		in = engine.NewCSVReader(f)
	case "xml":
		block := cfg.Source.XMLBlock
		if block == "" {
			block = "Record"
		}
		in = engine.NewXMLReader(f, block)
	case "edi":
		var dialect *dialects.Dialect
		if cfg.Source.Dialect != "" {
			dialect, err = dialects.LoadDialect(cfg.Source.Dialect)
			if err != nil {
				return fmt.Errorf("load dialect: %w", err)
			}
		} else {
			d := &dialects.Dialect{MessageStartSegment: "MSH", Segments: nil}
			d.Delimiters.Segment = "\n"
			d.Delimiters.Field = "|"
			d.Delimiters.Component = "^"
			dialect = d
		}
		in = engine.NewEDIReader(f, dialect)
	default:
		// jsonl: use file as-is
	}

	schema, err := engine.LoadSchema(cfg.Schema)
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	// Build sinks and closers from targets
	var sinks []engine.RowSink
	var closers []func() error

	for _, t := range cfg.Targets {
		s, closeFn, err := targetToSink(&t, schema)
		if err != nil {
			return fmt.Errorf("target %s: %w", t.Type, err)
		}
		sinks = append(sinks, s)
		if closeFn != nil {
			closers = append(closers, closeFn)
		}
	}

	if len(sinks) == 0 {
		return fmt.Errorf("no targets defined")
	}

	sink := engine.RowSink(&engine.MultiSink{Sinks: sinks})
	if fanoutMask == "pii" {
		sink = &engine.MaskingSink{Underlying: sink}
		fmt.Fprintf(os.Stderr, "🛡️  Semantic Masking Enabled: PII will be anonymized in the output stream.\n")
	}

	fmt.Fprintf(os.Stderr, "%s🌪️  Fan-Out initiated: Multiplexing stream to %d destinations.%s\n", violetANSI, len(cfg.Targets), resetANSI)

	start := time.Now()
	rows, err := engine.MapStream(io.NopCloser(in), schema, sink)
	if err != nil {
		return fmt.Errorf("map stream: %w", err)
	}
	for _, closeFn := range closers {
		if err := closeFn(); err != nil {
			return fmt.Errorf("close sink: %w", err)
		}
	}
	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "Mapped %d rows in %s\n", rows, elapsed.Round(time.Millisecond))
	return nil
}

func targetToSink(t *config.TargetConfig, schema *engine.Schema) (engine.RowSink, func() error, error) {
	switch strings.ToLower(t.Type) {
	case "local":
		if t.Path == "" {
			return nil, nil, fmt.Errorf("local target requires path")
		}
		out, err := os.Create(t.Path)
		if err != nil {
			return nil, nil, err
		}
		format := strings.ToLower(t.Format)
		if format == "" {
			format = "jsonl"
		}
		if format == "parquet" {
			pw, err := engine.NewParquetWriter(out, schema)
			if err != nil {
				out.Close()
				return nil, nil, err
			}
			return pw, func() error { return pw.Close() }, nil
		}
		return engine.JSONLWriter{W: out}, func() error { return out.Close() }, nil
	case "s3":
		if t.Bucket == "" || t.Key == "" {
			return nil, nil, fmt.Errorf("s3 target requires bucket and key")
		}
		s3s, err := engine.NewS3Sink(t.Bucket, t.Key)
		if err != nil {
			return nil, nil, err
		}
		return s3s, s3s.Close, nil
	case "prometheus":
		if t.URL == "" {
			return nil, nil, fmt.Errorf("prometheus target requires url")
		}
		ps := engine.NewPrometheusSink(t.URL, "dump")
		return ps, ps.Close, nil
	case "elasticsearch":
		if t.URL == "" || t.Index == "" {
			return nil, nil, fmt.Errorf("elasticsearch target requires url and index")
		}
		es, err := engine.NewElasticsearchSink(t.URL, t.Index)
		if err != nil {
			return nil, nil, err
		}
		return es, es.Close, nil
	default:
		return nil, nil, fmt.Errorf("unknown target type %q", t.Type)
	}
}
