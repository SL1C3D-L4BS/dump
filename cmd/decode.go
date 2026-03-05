package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/spf13/cobra"
)

const violetStatus = "\033[35m🔓 Protobuf heuristic decoding complete. Proceeding without .proto schema.\033[0m"

var (
	decodeType string
)

var decodeCmd = &cobra.Command{
	Use:   "decode [file]",
	Short: "Decode binary payloads to JSON (e.g. Protobuf without schema)",
	Long:  `Reads a binary file and decodes it to JSON. Use --type protobuf for heuristic Protobuf decoding without a .proto schema.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDecode,
}

func init() {
	decodeCmd.Flags().StringVar(&decodeType, "type", "protobuf", "Decode type (protobuf)")
}

func runDecode(cmd *cobra.Command, args []string) error {
	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	switch decodeType {
	case "protobuf":
		m, err := engine.DecodeProtobuf(data)
		if err != nil {
			return fmt.Errorf("decode protobuf: %w", err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		fmt.Fprintln(os.Stderr, violetStatus)
	default:
		return fmt.Errorf("unsupported decode type: %q (use: protobuf)", decodeType)
	}
	return nil
}
