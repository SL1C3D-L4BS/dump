package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/integrity"
	"github.com/spf13/cobra"
)

var (
	verifySeal     string
	verifySealFile  string
)

var verifyCmd = &cobra.Command{
	Use:   "verify [file]",
	Short: "Verify a file against its Vericore Seal",
	Long:  `Reads the seal from --seal, --seal-file, or <file>.vericore-seal sidecar; verifies the file's hash against the PQC signature. Outputs VERIFIED or TAMPERED.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runVerify,
}

func init() {
	verifyCmd.Flags().StringVar(&verifySeal, "seal", "", "Seal string (e.g. from X-Vericore-Seal header)")
	verifyCmd.Flags().StringVar(&verifySealFile, "seal-file", "", "Path to file containing the seal (default: <file>.vericore-seal)")
}

func runVerify(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	var seal string
	switch {
	case verifySeal != "":
		seal = strings.ReplaceAll(verifySeal, "\\n", "\n")
	case verifySealFile != "":
		data, err := os.ReadFile(verifySealFile)
		if err != nil {
			return fmt.Errorf("read seal file: %w", err)
		}
		seal = string(data)
	default:
		sidecar := filePath + ".vericore-seal"
		data, err := os.ReadFile(sidecar)
		if err != nil {
			return fmt.Errorf("no seal provided; use --seal or --seal-file, or create %s: %w", sidecar, err)
		}
		seal = string(data)
	}
	if strings.TrimSpace(seal) == "" {
		return fmt.Errorf("seal is empty")
	}
	status, err := integrity.VerifyResult(filePath, seal)
	if err != nil {
		return err
	}
	switch status {
	case "VERIFIED":
		fmt.Fprintln(os.Stdout, "VERIFIED")
		return nil
	default:
		fmt.Fprintln(os.Stdout, "TAMPERED")
		return nil
	}
}
