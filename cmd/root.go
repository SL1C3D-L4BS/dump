package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	industryFlag string
)

var rootCmd = &cobra.Command{
	Use:   "dump",
	Short: "Data Universal Mapping Platform — AI-assisted schema inference and hyperspeed data mapping",
	Long: `DUMP is a high-performance CLI that maps disparate data formats (JSON, Parquet, SQL, Protobuf)
using AI-inferred schemas. Infer schemas from samples, then map data at hyperspeed.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&industryFlag, "industry", "", "Industry mode (e.g. healthcare) for protocol defaults and standard dialects")
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(cryptoCmd)
	rootCmd.AddCommand(decodeCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(fanoutCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(inferCmd)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(mapCmd)
	rootCmd.AddCommand(mirrorCmd)
	rootCmd.AddCommand(nl2sCmd)
	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(stressCmd)
	rootCmd.AddCommand(verifyCmd)
}
