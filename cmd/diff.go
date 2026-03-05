package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/spf13/cobra"
)

var (
	diffS1       string
	diffS2       string
	diffOn       string
	diffIgnore   string
	diffFormat   string
	diffS1Sheet  string
	diffS2Sheet  string
	diffS1Query  string
	diffS2Query  string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Heterogeneous diff: compare two data sources (XLSX, JSON, SQL) on a common primary key",
	Long:  `Compares --s1 and --s2 (file paths or DB connection strings) using --on as the primary key. Reports rows only in S1, only in S2, and field-level discrepancies. Use --s1-query/--s2-query for SQL sources.`,
	Args:  cobra.NoArgs,
	RunE:  runDiff,
}

func init() {
	diffCmd.Flags().StringVar(&diffS1, "s1", "", "First source: path to file (XLSX/JSON/CSV) or DB connection string (required)")
	diffCmd.Flags().StringVar(&diffS2, "s2", "", "Second source: path or connection string (required)")
	diffCmd.Flags().StringVar(&diffOn, "on", "", "Primary key column name to join on (required)")
	diffCmd.Flags().StringVar(&diffIgnore, "ignore", "", "Comma-separated fields to ignore in comparison (e.g. updated_at)")
	diffCmd.Flags().StringVar(&diffFormat, "format", "table", "Output format: table or json")
	diffCmd.Flags().StringVar(&diffS1Sheet, "s1-sheet", "", "Sheet name for S1 when Excel (default: first sheet)")
	diffCmd.Flags().StringVar(&diffS2Sheet, "s2-sheet", "", "Sheet name for S2 when Excel (default: first sheet)")
	diffCmd.Flags().StringVar(&diffS1Query, "s1-query", "", "SQL query for S1 when S1 is a connection string (required for SQL)")
	diffCmd.Flags().StringVar(&diffS2Query, "s2-query", "", "SQL query for S2 when S2 is a connection string (required for SQL)")
	_ = diffCmd.MarkFlagRequired("s1")
	_ = diffCmd.MarkFlagRequired("s2")
	_ = diffCmd.MarkFlagRequired("on")
}

func runDiff(cmd *cobra.Command, args []string) error {
	ignoreSet := make(map[string]bool)
	for _, f := range strings.Split(diffIgnore, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			ignoreSet[f] = true
		}
	}

	r1, err := engine.NewRowReaderFromSource(diffS1, diffS1Sheet, diffS1Query)
	if err != nil {
		return fmt.Errorf("s1: %w", err)
	}
	defer engine.CloseRowReader(r1)

	r2, err := engine.NewRowReaderFromSource(diffS2, diffS2Sheet, diffS2Query)
	if err != nil {
		return fmt.Errorf("s2: %w", err)
	}
	defer engine.CloseRowReader(r2)

	msg := fmt.Sprintf("🔍 Cross-Format Diff Active: Reconciling [%s] and [%s] on [%s].", diffS1, diffS2, diffOn)
	fmt.Fprintf(os.Stderr, "%s%s%s\n", violetANSIPrefix, msg, violetANSIReset)

	report, err := engine.CompareSources(r1, r2, diffOn, ignoreSet)
	if err != nil {
		return err
	}

	switch strings.ToLower(diffFormat) {
	case "json":
		out, err := engine.DiffReportJSON(report)
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	default:
		printDiffTable(report)
	}
	return nil
}

func printDiffTable(r *engine.DiffReport) {
	fmt.Println("=== Only in S1 ===")
	fmt.Printf("Count: %d\n", len(r.OnlyInS1))
	for i, row := range r.OnlyInS1 {
		if i >= 5 {
			fmt.Printf("... and %d more\n", len(r.OnlyInS1)-5)
			break
		}
		b, _ := json.Marshal(row)
		fmt.Println("  ", string(b))
	}
	fmt.Println()
	fmt.Println("=== Only in S2 ===")
	fmt.Printf("Count: %d\n", len(r.OnlyInS2))
	for i, row := range r.OnlyInS2 {
		if i >= 5 {
			fmt.Printf("... and %d more\n", len(r.OnlyInS2)-5)
			break
		}
		b, _ := json.Marshal(row)
		fmt.Println("  ", string(b))
	}
	fmt.Println()
	fmt.Println("=== Discrepancies (same key, different fields) ===")
	fmt.Printf("Count: %d\n", len(r.Discrepancies))
	for id, diffs := range r.Discrepancies {
		fmt.Printf("  ID %s:\n", id)
		for _, d := range diffs {
			fmt.Printf("    %s: %v vs %v\n", d.Key, d.Val1, d.Val2)
		}
	}
}
