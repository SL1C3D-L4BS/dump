package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SL1C3D-L4BS/dump/internal/inference"
	"github.com/spf13/cobra"
)

const violetANSIPrefix = "\033[35m"
const violetANSIReset = "\033[0m"

var (
	nl2sTemplate string
	nl2sModel    string
	nl2sStrict   bool
)

var nl2sCmd = &cobra.Command{
	Use:   "nl2s",
	Short: "AI Prompt-to-Schema (NL2S): force unstructured text into a strict JSON contract via Ollama",
	Long:  `Reads unstructured text from stdin, sends it to local Ollama with a JSON template, and prints sanitized JSON to stdout. Use --template with a file path or raw JSON.`,
	Args:  cobra.NoArgs,
	RunE:  runNL2S,
}

func init() {
	nl2sCmd.Flags().StringVar(&nl2sTemplate, "template", "", "Path to a JSON template file or raw JSON string (required)")
	nl2sCmd.Flags().StringVar(&nl2sModel, "model", "llama3", "Ollama model name")
	nl2sCmd.Flags().BoolVar(&nl2sStrict, "strict", true, "Remove keys not present in template to prevent AI hallucinations")
	_ = nl2sCmd.MarkFlagRequired("template")
}

func runNL2S(cmd *cobra.Command, args []string) error {
	templateJSON, templateName, err := resolveTemplate(nl2sTemplate)
	if err != nil {
		return err
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	inputText := string(input)

	// Status to stderr in violet
	msg := "🎯 NL2S Extraction Active: Forcing unstructured text into " + templateName + " contract."
	fmt.Fprintf(os.Stderr, "%s%s%s\n", violetANSIPrefix, msg, violetANSIReset)

	out, err := inference.ExtractToTemplate(inputText, templateJSON, nl2sModel, nl2sStrict)
	if err != nil {
		return err
	}
	fmt.Print(out)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

// resolveTemplate returns (templateJSON, templateName, error).
// If nl2sTemplate is a path to an existing file, read it; otherwise use as raw JSON.
func resolveTemplate(flag string) (string, string, error) {
	if flag == "" {
		return "", "", nil
	}
	if _, err := os.Stat(flag); err == nil {
		b, err := os.ReadFile(flag)
		if err != nil {
			return "", "", err
		}
		name := filepath.Base(flag)
		return string(b), name, nil
	}
	return flag, "inline", nil
}
