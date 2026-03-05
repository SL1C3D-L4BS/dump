package cmd

import (
	"fmt"
	"os"

	"github.com/SL1C3D-L4BS/dump/internal/tools"
	"github.com/spf13/cobra"
)

var (
	stressOutput string
	stressSize   int64
)

var stressCmd = &cobra.Command{
	Use:   "stress [output path]",
	Short: "Generate 1.2GB X12 837 stress file for SL1C3D-L4BS validation",
	Long:  `Writes a synthetic X12 837 (Healthcare Claims) file with ~2M CLM loops. Used for memory and throughput benchmarks. Default size 1.2GB.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStress,
}

func init() {
	rootCmd.AddCommand(stressCmd)
	stressCmd.Flags().StringVarP(&stressOutput, "output", "o", "stress_837.x12", "Output file path")
	stressCmd.Flags().Int64VarP(&stressSize, "size-mb", "s", 1200, "Target size in MB (default 1200 = 1.2GB)")
}

func runStress(cmd *cobra.Command, args []string) error {
	outPath := stressOutput
	if len(args) > 0 {
		outPath = args[0]
	}
	targetBytes := stressSize * 1024 * 1024
	fmt.Fprintf(os.Stderr, "Generating X12 837 stress file: %s (~%d MB)\n", outPath, stressSize)
	written, claims, err := tools.WriteStressFile(outPath, targetBytes)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote %d bytes (%d claims)\n", written, claims)
	return nil
}
