package cmd

import (
	"os"

	"github.com/spf13/cobra"
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
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(decodeCmd)
	rootCmd.AddCommand(fanoutCmd)
	rootCmd.AddCommand(inferCmd)
	rootCmd.AddCommand(mapCmd)
	rootCmd.AddCommand(mirrorCmd)
	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(verifyCmd)
}
