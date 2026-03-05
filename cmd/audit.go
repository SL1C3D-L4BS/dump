package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	vericrypto "github.com/SL1C3D-L4BS/dump/internal/crypto"
	"github.com/spf13/cobra"
)

var (
	auditLimit  int
	auditVerifyID  int64
	auditVerifyAll bool
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect the persistent Vericore MMR audit log",
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent audit entries (MMR roots, file hashes, PQC signatures)",
	Args:  cobra.NoArgs,
	RunE:  runAuditList,
}

var auditVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check integrity of files in the audit log",
	Long:  `Re-computes file hashes and verifies PQC signatures against MMR roots. Use --id to verify one entry or --all to verify every entry.`,
	Args:  cobra.NoArgs,
	RunE:  runAuditVerify,
}

func init() {
	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditVerifyCmd)
	auditListCmd.Flags().IntVar(&auditLimit, "limit", 100, "Number of recent entries to show")
	auditVerifyCmd.Flags().Int64Var(&auditVerifyID, "id", 0, "Verify a single audit entry by ID")
	auditVerifyCmd.Flags().BoolVar(&auditVerifyAll, "all", false, "Verify all audit entries")
}

func runAuditList(cmd *cobra.Command, args []string) error {
	entries, err := vericrypto.List(auditLimit)
	if err != nil {
		return fmt.Errorf("list audit log: %w", err)
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stdout, "No audit entries found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTIMESTAMP\tTOOL\tPATH\tMMR_ROOT\tFILE_HASH")
	for _, e := range entries {
		ts := e.Timestamp.Format(time.RFC3339)
		root := shortenHex(e.MMRRoot, 12)
		hash := shortenHex(e.FileHash, 12)
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", e.ID, ts, e.Tool, e.FilePath, root, hash)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

func shortenHex(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 4 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func runAuditVerify(cmd *cobra.Command, args []string) error {
	if !auditVerifyAll && auditVerifyID == 0 {
		return fmt.Errorf("specify --id <ID> to verify one entry or --all to verify all")
	}
	if auditVerifyAll && auditVerifyID != 0 {
		return fmt.Errorf("use either --id or --all, not both")
	}

	var results []vericrypto.VerifyResult
	if auditVerifyAll {
		var err error
		results, err = vericrypto.VerifyAll()
		if err != nil {
			return fmt.Errorf("verify all: %w", err)
		}
	} else {
		r, err := vericrypto.VerifyByID(auditVerifyID)
		if err != nil {
			return fmt.Errorf("verify: %w", err)
		}
		results = []vericrypto.VerifyResult{r}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPATH\tSTATUS\tREASON")
	for _, r := range results {
		reason := r.Reason
		if reason == "" {
			reason = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", r.ID, r.FilePath, r.Status, reason)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	for _, r := range results {
		if r.Status != "VERIFIED" {
			return fmt.Errorf("one or more entries failed verification")
		}
	}
	return nil
}

