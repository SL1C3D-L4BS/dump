package cmd

import (
	"fmt"
	"os"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/spf13/cobra"
)

const mirrorStatusViolet = "\033[35m💎 Statistical Mirroring Complete: 100%% Synthetic, 0%% Prod-Data Leakage.\033[0m"

var (
	mirrorFrom string
	mirrorTo   string
	mirrorRows int
)

var mirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "Generate a statistical clone of a production database (synthetic data only)",
	Long:  `Analyzes the source DB schema and stats, then generates synthetic rows with FK consistency and writes to the target DB. No real data leaves the source.`,
	Args:  cobra.NoArgs,
	RunE:  runMirror,
}

func init() {
	mirrorCmd.Flags().StringVar(&mirrorFrom, "from", "", "Source DB connection string (postgres://... or sqlite://...)")
	mirrorCmd.Flags().StringVar(&mirrorTo, "to", "", "Target DB connection string (e.g. sqlite://local.db)")
	mirrorCmd.Flags().IntVar(&mirrorRows, "rows", 1000, "Number of synthetic rows per table")
	_ = mirrorCmd.MarkFlagRequired("from")
	_ = mirrorCmd.MarkFlagRequired("to")
}

func runMirror(cmd *cobra.Command, args []string) error {
	if mirrorFrom == "" || mirrorTo == "" {
		return fmt.Errorf("both --from and --to are required")
	}
	if mirrorRows < 1 {
		mirrorRows = 1000
	}

	// 1) Analyze source -> build spec
	spec, err := engine.AnalyzeDatabase(mirrorFrom)
	if err != nil {
		return fmt.Errorf("analyze source: %w", err)
	}
	if len(spec.Tables) == 0 {
		return fmt.Errorf("no tables found in source database")
	}

	// 2) Generate synthetic data (topological order, FK-safe)
	data := engine.GenerateSyntheticData(spec, mirrorRows)

	// 3) Write to target: CREATE TABLEs then bulk insert
	writer, err := engine.NewSQLWriter(mirrorTo)
	if err != nil {
		return fmt.Errorf("open target: %w", err)
	}
	defer writer.Close()

	if err := writer.CreateTables(spec); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	if err := writer.InsertRows(spec, data); err != nil {
		return fmt.Errorf("insert rows: %w", err)
	}

	fmt.Fprintln(os.Stderr, mirrorStatusViolet)
	return nil
}
