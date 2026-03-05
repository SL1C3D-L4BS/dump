package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/spf13/cobra"
)

var (
	scanSource       string
	scanPath         string
	scanOutputFormat string
	scanVericoreDir  string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Shadow IT discovery: find and profile untracked data assets",
	Long:  `Scans a directory for data files (.csv, .xlsx, .jsonl, .edi), runs a silent analyze (row count, PII density, schema complexity), and suggests DUMP mapping commands to migrate into the Vericore central store.`,
	RunE:  runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringVar(&scanSource, "source", "filesystem", "Scan source (filesystem)")
	scanCmd.Flags().StringVar(&scanPath, "path", ".", "Directory path to walk (default: current directory)")
	scanCmd.Flags().StringVar(&scanOutputFormat, "format", "table", "Report format: table or json")
	scanCmd.Flags().StringVar(&scanVericoreDir, "vericore-store", "./vericore_ingest", "Suggested output directory for Vericore central store migration")
}

func runScan(cmd *cobra.Command, args []string) error {
	if scanSource != "filesystem" {
		return fmt.Errorf("only --source filesystem is supported")
	}
	fmt.Fprintf(os.Stderr, "%s🔍 Shadow IT Scan: Identifying untracked data assets.%s\n", violetANSI, resetANSI)

	root := scanPath
	if root == "" {
		root = "."
	}
	var results []*engine.ShadowFileResult
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".csv", ".xlsx", ".jsonl", ".json", ".edi", ".hl7", ".x12":
			// skip
		default:
			return nil
		}
		res, err := engine.ScanDataFile(path, scanVericoreDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", path, err)
			return nil
		}
		results = append(results, res)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	if scanOutputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"shadow_it_footprint": results,
			"summary": map[string]interface{}{
				"total_files":   len(results),
				"vericore_store": scanVericoreDir,
			},
		})
	}

	// Table
	fmt.Println("Path | Format | Rows | PII Density | Complexity | Suggested Command")
	fmt.Println("-----|--------|------|-------------|------------|-------------------")
	for _, r := range results {
		piiPct := ""
		if r.PIIDensity > 0 {
			piiPct = fmt.Sprintf("%.0f%%", r.PIIDensity*100)
		} else {
			piiPct = "0%"
		}
		fmt.Printf("%s | %s | %d | %s | %d | %s\n", r.Path, r.Format, r.RowCount, piiPct, r.SchemaComplexity, r.SuggestedCommand)
	}
	fmt.Fprintf(os.Stderr, "\nTotal untracked data files: %d. Migrate to Vericore with the suggested dump map commands above.\n", len(results))
	return nil
}
