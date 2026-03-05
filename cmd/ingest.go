package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/SL1C3D-L4BS/dump/internal/engine"
	"github.com/SL1C3D-L4BS/dump/pkg/ingest"
	"github.com/spf13/cobra"
)

var (
	ingestBroker       string
	ingestTopic        string
	ingestSchema       string
	ingestPushgateway  string
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "IoT/edge ingest: decode binary payloads and push to Prometheus or other sinks",
	Long:  `Subcommands for MQTT and other ingestors that decode bit-level binary and forward to Pushgateway.`,
}

var ingestMqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "MQTT ingestor: subscribe to topics, decode binary by schema, push to Prometheus",
	Long:  `Connects to an MQTT broker, subscribes to --topic, decodes each message with --schema (YAML bit-level mapping), and forwards to --pushgateway.`,
	Args:  cobra.NoArgs,
	RunE:  runIngestMQTT,
}

func init() {
	ingestCmd.AddCommand(ingestMqttCmd)
	ingestMqttCmd.Flags().StringVar(&ingestBroker, "broker", "tcp://localhost:1883", "MQTT broker URL (e.g. tcp://localhost:1883 or ssl://host:8883)")
	ingestMqttCmd.Flags().StringVar(&ingestTopic, "topic", "telemetry/sensors/#", "Topic to subscribe to (e.g. telemetry/sensors/#)")
	ingestMqttCmd.Flags().StringVar(&ingestSchema, "schema", "", "Path to YAML file with source: binary and bit-level field specs (required)")
	ingestMqttCmd.Flags().StringVar(&ingestPushgateway, "pushgateway", "http://localhost:9091", "Prometheus Pushgateway URL")
	_ = ingestMqttCmd.MarkFlagRequired("schema")
}

func runIngestMQTT(cmd *cobra.Command, args []string) error {
	sink := engine.NewPrometheusSink(ingestPushgateway, "dump")
	ing, err := ingest.NewMQTTIngestor(ingestBroker, ingestTopic, ingestSchema, sink)
	if err != nil {
		return err
	}

	msg := "📡 IoT Ingest Active: Squashing binary payloads into Prometheus metrics."
	fmt.Fprintf(os.Stderr, "%s%s%s\n", violetANSIPrefix, msg, violetANSIReset)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		_ = ing.Run()
	}()

	<-sigCh
	ing.Stop()
	_ = sink.Close()
	return nil
}
