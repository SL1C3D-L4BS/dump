package cmd

import (
	"fmt"
	"os"

	vericrypto "github.com/SL1C3D-L4BS/dump/internal/crypto"
	"github.com/spf13/cobra"
)

var cryptoCmd = &cobra.Command{
	Use:   "crypto",
	Short: "Cryptographic key lifecycle (Dilithium2)",
	Long:  `Manage Vericore PQC keys: rotate generates a new keypair, archives the old keys, and logs the event.`,
}

var cryptoRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate the Dilithium2 keypair and archive old keys",
	Long:  `Generates a new Dilithium2 keypair, archives the current keys to ~/.vericore/keys/archive/, writes the new keys to the default Vericore keys path, and logs the rotation in the audit_rotations table.`,
	Args:  cobra.NoArgs,
	RunE:  runCryptoRotate,
}

func init() {
	rootCmd.AddCommand(cryptoCmd)
	cryptoCmd.AddCommand(cryptoRotateCmd)
}

func runCryptoRotate(cmd *cobra.Command, args []string) error {
	if err := vericrypto.Rotate(); err != nil {
		return fmt.Errorf("rotate keys: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Dilithium2 keypair rotated. Old keys archived; rotation logged in audit_rotations.")
	return nil
}
