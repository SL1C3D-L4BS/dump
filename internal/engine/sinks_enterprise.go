// Package engine: enterprise RowSink implementations (S3, Prometheus, Elasticsearch).
// Each implements RowSink and provides Close() for flushing/uploading.

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// S3Sink buffers JSONL rows to a temp file and uploads to S3 on Close().
type S3Sink struct {
	Bucket   string
	Key      string
	tmpFile  *os.File
	uploaded bool
}

// NewS3Sink creates an S3Sink that will upload to the given bucket/key on Close().
func NewS3Sink(bucket, key string) (*S3Sink, error) {
	f, err := os.CreateTemp("", "dump-s3-*.jsonl")
	if err != nil {
		return nil, fmt.Errorf("s3 sink temp file: %w", err)
	}
	return &S3Sink{Bucket: bucket, Key: key, tmpFile: f}, nil
}

// WriteRow implements RowSink. Appends row as a single JSONL line to the temp file.
func (s *S3Sink) WriteRow(row []byte) error {
	_, err := s.tmpFile.Write(append(row, '\n'))
	return err
}

// Close uploads the temp file to S3 and removes it. Idempotent after first success.
func (s *S3Sink) Close() error {
	if s.uploaded || s.tmpFile == nil {
		return nil
	}
	path := s.tmpFile.Name()
	if err := s.tmpFile.Sync(); err != nil {
		return err
	}
	if _, err := s.tmpFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("s3 config: %w", err)
	}
	client := s3.NewFromConfig(cfg)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.Key),
		Body:   s.tmpFile,
	})
	_ = s.tmpFile.Close()
	_ = os.Remove(path)
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	s.uploaded = true
	return nil
}

// PrometheusSink increments a counter per row and pushes to Pushgateway on Close().
type PrometheusSink struct {
	URL     string
	Job     string
	counter prometheus.Counter
	pushed  bool
}

// NewPrometheusSink creates a sink that counts rows and pushes dump_rows_processed_total to the given Pushgateway URL on Close().
func NewPrometheusSink(pushgatewayURL, job string) *PrometheusSink {
	if job == "" {
		job = "dump"
	}
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dump_rows_processed_total",
		Help: "Total number of rows processed by DUMP fan-out.",
	})
	return &PrometheusSink{URL: strings.TrimSuffix(pushgatewayURL, "/"), Job: job, counter: c}
}

// WriteRow implements RowSink. Increments the counter.
func (p *PrometheusSink) WriteRow(row []byte) error {
	p.counter.Inc()
	return nil
}

// Close pushes the counter to the Pushgateway. Idempotent after first success.
func (p *PrometheusSink) Close() error {
	if p.pushed {
		return nil
	}
	if err := push.New(p.URL, p.Job).Collector(p.counter).Push(); err != nil {
		return fmt.Errorf("prometheus push: %w", err)
	}
	p.pushed = true
	return nil
}

// ElasticsearchSink buffers rows and bulk-indexes them on Close().
type ElasticsearchSink struct {
	URL    string
	Index  string
	client *es.Client
	buf    [][]byte
}

// NewElasticsearchSink creates a sink that bulk-indexes rows to the given ES URL and index on Close().
func NewElasticsearchSink(esURL, index string) (*ElasticsearchSink, error) {
	cfg := es.Config{Addresses: []string{esURL}}
	client, err := es.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch client: %w", err)
	}
	return &ElasticsearchSink{URL: esURL, Index: index, client: client}, nil
}

// WriteRow implements RowSink. Buffers the row for bulk index.
func (e *ElasticsearchSink) WriteRow(row []byte) error {
	e.buf = append(e.buf, append([]byte(nil), row...))
	return nil
}

// Close sends the buffered documents to ES Bulk API. Idempotent (clears buffer after first success).
func (e *ElasticsearchSink) Close() error {
	if len(e.buf) == 0 {
		return nil
	}
	var body bytes.Buffer
	for _, doc := range e.buf {
		meta := map[string]interface{}{"index": map[string]interface{}{"_index": e.Index}}
		metaJSON, _ := json.Marshal(meta)
		body.Write(metaJSON)
		body.WriteByte('\n')
		body.Write(doc)
		body.WriteByte('\n')
	}
	req := esapi.BulkRequest{Body: bytes.NewReader(body.Bytes())}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("elasticsearch bulk: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("elasticsearch bulk response: %s", res.String())
	}
	e.buf = nil
	return nil
}
