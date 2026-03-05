package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/pkg/generators"
	"github.com/SL1C3D-L4BS/dump/pkg/healthcare"
	"github.com/spf13/cobra"
)

var (
	generateCsharpOutput   string
	generateCsharpNamespace string
	generateCsharpClass     string
	generateCsharpDialect   string
	generateCsharpModel     string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from HL7/X12 or other sources (TypeGen)",
	Long:  `Subcommands that produce strictly-typed code (e.g. C# POCOs) from EDI/HL7 samples.`,
}

var generateCsharpCmd = &cobra.Command{
	Use:   "csharp [file]",
	Short: "Generate C# POCOs from an HL7 or X12 sample",
	Long:  `Reads an HL7 or X12 sample (file or stdin), infers the schema from the data, and outputs a strictly-typed C# class file. Nested loops become List<T> properties.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGenerateCsharp,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.AddCommand(generateCsharpCmd)
	generateCsharpCmd.Flags().StringVar(&generateCsharpOutput, "output", "", "Write .cs file here (default: stdout)")
	generateCsharpCmd.Flags().StringVar(&generateCsharpNamespace, "namespace", "Generated.Hl7", "C# namespace for generated types")
	generateCsharpCmd.Flags().StringVar(&generateCsharpClass, "class", "Hl7Message", "Root class name")
	generateCsharpCmd.Flags().StringVar(&generateCsharpDialect, "dialect", "", "Path to dialect YAML (optional; with --industry healthcare uses standard)")
	generateCsharpCmd.Flags().StringVar(&generateCsharpModel, "model", "llama3", "Ollama model (for optional LLM refinement)")
}

func runGenerateCsharp(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "%s⚙️ TypeGen Active: Transpiling HL7/X12 to C# POCOs.%s\n", violetANSI, resetANSI)

	var peek []byte
	var format string
	var dialect *dialects.Dialect

	if len(args) >= 1 {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		peek = make([]byte, 1024)
		n, _ := f.Read(peek)
		f.Close()
		peek = peek[:n]
		format = engine.DetectFormat(peek)
		if format != "edi" {
			format = "edi"
		}
		dialect, _ = dialectForGenerate(peek)
	} else {
		peek = make([]byte, 1024)
		n, _ := os.Stdin.Read(peek)
		peek = peek[:n]
		format = engine.DetectFormat(peek)
		if format != "edi" {
			format = "edi"
		}
		dialect, _ = dialectForGenerate(peek)
	}

	var sample string
	var err error
	if len(args) >= 1 {
		f2, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("reopen: %w", err)
		}
		defer f2.Close()
		sample, err = engine.ExtractSampleWithDialect(f2, format, generateCsharpDialect, 20, dialect)
		if err != nil {
			return fmt.Errorf("extract sample: %w", err)
		}
	} else {
		rest, _ := io.ReadAll(os.Stdin)
		combined := bytes.NewReader(append(peek, rest...))
		sample, err = engine.ExtractSampleWithDialect(combined, format, generateCsharpDialect, 20, dialect)
		if err != nil {
			return fmt.Errorf("extract sample: %w", err)
		}
	}

	if sample == "[]" || len(sample) < 3 {
		return fmt.Errorf("no sample data could be extracted (format %q); provide an HL7 or X12 file", format)
	}

	cs, err := generators.InferAndGenerate(sample, generateCsharpNamespace, generateCsharpClass)
	if err != nil {
		return fmt.Errorf("generate C#: %w", err)
	}

	if generateCsharpOutput != "" {
		if err := os.WriteFile(generateCsharpOutput, []byte(cs), 0644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s\n", generateCsharpOutput)
	} else {
		fmt.Print(cs)
	}
	return nil
}

func dialectForGenerate(peek []byte) (*dialects.Dialect, error) {
	if industryFlag == "healthcare" || generateCsharpDialect == "" {
		standardName := "hl7_v25"
		if bytes.Contains(peek, []byte("ISA*")) {
			standardName = "x12_837"
		}
		return healthcare.LoadStandardDialect(standardName)
	}
	return dialects.LoadDialect(generateCsharpDialect)
}
