// Package ingest provides IoT/edge ingestors that decode binary payloads and forward to sinks.

package ingest

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/SL1C3D-L4BS/dump/internal/engine"
)

// RowSink writes decoded rows (JSON bytes). Implemented by engine.PrometheusSink and others.
type RowSink interface {
	WriteRow(row []byte) error
}

// MQTTIngestor connects to an MQTT broker, subscribes to topics, decodes binary payloads
// with a bit-level schema, and forwards the decoded map to a RowSink (e.g. Prometheus).
type MQTTIngestor struct {
	Broker   string
	Topic    string
	SchemaPath string
	Sink     RowSink
	Specs    []engine.FieldSpec
	client   mqtt.Client
	done    chan struct{}
	mu      sync.Mutex
}

// NewMQTTIngestor builds an ingestor. Schema is loaded from schemaPath (YAML with source: binary).
func NewMQTTIngestor(broker, topic, schemaPath string, sink RowSink) (*MQTTIngestor, error) {
	specs, err := engine.LoadBinaryMappingSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("load binary schema: %w", err)
	}
	return &MQTTIngestor{
		Broker:      broker,
		Topic:       topic,
		SchemaPath:  schemaPath,
		Sink:        sink,
		Specs:       specs,
		done:        make(chan struct{}),
	}, nil
}

// Run connects to the broker, subscribes to the topic, and processes messages until Stop is called.
func (m *MQTTIngestor) Run() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(m.Broker)
	opts.SetClientID(fmt.Sprintf("dump-ingest-%d", time.Now().UnixNano()))
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		token := c.Subscribe(m.Topic, 1, m.onMessage)
		token.Wait()
		if token.Error() != nil {
			// log but don't exit; reconnection will retry
			_ = token.Error()
		}
	})

	m.client = mqtt.NewClient(opts)
	token := m.client.Connect()
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("mqtt connect: %w", token.Error())
	}
	<-m.done
	return nil
}

// Stop signals Run to exit and disconnects the client.
func (m *MQTTIngestor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.done:
		return
	default:
		close(m.done)
		if m.client != nil {
			m.client.Disconnect(250)
		}
	}
}

func (m *MQTTIngestor) onMessage(_ mqtt.Client, msg mqtt.Message) {
	decoded, err := engine.DecodeBinary(msg.Payload(), m.Specs)
	if err != nil {
		return
	}
	row, err := json.Marshal(decoded)
	if err != nil {
		return
	}
	_ = m.Sink.WriteRow(row)
}
