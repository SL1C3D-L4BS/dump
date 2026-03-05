package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SL1C3D-L4BS/dump/internal/inference"
	"github.com/spf13/cobra"
)

var (
	inferTarget     string
	inferSampleSize int
	inferModel      string
)

const ollamaHealthURL = "http://localhost:11434"

var inferCmd = &cobra.Command{
	Use:   "infer [input file]",
	Short: "Infer a YAML mapping schema from sample data using local AI (Ollama)",
	Long:  `Reads the input file (or first N lines / a small chunk for JSON), sends it to Ollama, and streams the generated YAML mapping to stdout.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runInfer,
}

func init() {
	inferCmd.Flags().StringVar(&inferTarget, "target", "protobuf", "Target schema format (e.g. protobuf, parquet)")
	inferCmd.Flags().IntVar(&inferSampleSize, "sample-size", 10, "Number of lines to sample, or approximate chunk size for JSON")
	inferCmd.Flags().StringVar(&inferModel, "model", "llama3", "Ollama model name (e.g. llama3)")
}

func runInfer(cmd *cobra.Command, args []string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(ollamaHealthURL)
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return fmt.Errorf("Ollama is not running. Please install Ollama and run 'ollama run %s'", inferModel)
	}
	resp.Body.Close()

	inputPath := args[0]
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	sample, err := readSample(f, inferSampleSize)
	if err != nil {
		return fmt.Errorf("read sample: %w", err)
	}

	inf := inference.NewSchemaInferencer("", nil)
	yamlOut, err := inf.InferMapping(sample, inferTarget, inferModel)
	if err != nil {
		return err
	}

	fmt.Print(yamlOut)
	return nil
}

// readSample reads up to sampleSize lines, or for JSON a single array/object chunk.
func readSample(r *os.File, sampleSize int) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	var lines [][]byte
	for scanner.Scan() && len(lines) < sampleSize {
		lines = append(lines, scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	raw := bytes.Join(lines, []byte("\n"))
	// If it looks like JSON, try to use a valid subset (first N array elements or the object)
	if json.Valid(raw) {
		return raw, nil
	}
	// If not valid JSON, try to build a minimal JSON array from lines
	var arr []interface{}
	for _, line := range lines {
		var v interface{}
		if err := json.Unmarshal(line, &v); err == nil {
			arr = append(arr, v)
		}
	}
	if len(arr) > 0 {
		return json.Marshal(arr)
	}
	return raw, nil
}
