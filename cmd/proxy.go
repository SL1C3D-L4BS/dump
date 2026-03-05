package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/SL1C3D-L4BS/dump/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	proxyUpstream string
	proxySchema   string
	proxyXMLBlock string
	proxyPort     int
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the JIT sidecar proxy (translate upstream XML to JSONL on the fly)",
	Long:  `Starts an HTTP server that forwards requests to an upstream URL and streams the response through the DUMP mapping engine, returning translated JSONL (NDJSON).`,
	Args:  cobra.NoArgs,
	RunE:  runProxy,
}

func init() {
	proxyCmd.Flags().StringVar(&proxyUpstream, "upstream", "", "Upstream base URL (e.g. http://legacy-system.local/api)")
	proxyCmd.Flags().StringVar(&proxySchema, "schema", "", "Path to the DUMP YAML mapping schema")
	proxyCmd.Flags().StringVar(&proxyXMLBlock, "xml-block", "Record", "Repeating XML element to split on for streaming")
	proxyCmd.Flags().IntVar(&proxyPort, "port", 8081, "Port to listen on")
	_ = proxyCmd.MarkFlagRequired("upstream")
	_ = proxyCmd.MarkFlagRequired("schema")
}

func runProxy(cmd *cobra.Command, args []string) error {
	jit, err := proxy.NewJITProxy(proxyUpstream, proxySchema, proxyXMLBlock)
	if err != nil {
		return fmt.Errorf("init proxy: %w", err)
	}

	addr := fmt.Sprintf(":%d", proxyPort)
	fmt.Fprintf(os.Stderr, "%s🚀 DUMP Sidecar Proxy running on port %d. Translating %s on the fly.%s\n", violetANSI, proxyPort, proxyUpstream, resetANSI)

	return http.ListenAndServe(addr, jit)
}
