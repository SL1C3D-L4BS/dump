// Package config parses fan-out configuration (fanout.yaml).

package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// FanOutConfig is the root structure for fanout.yaml.
type FanOutConfig struct {
	Source  SourceConfig    `yaml:"source"`
	Schema  string          `yaml:"schema"`
	Targets []TargetConfig  `yaml:"targets"`
}

// SourceConfig describes the streaming source (type and path).
type SourceConfig struct {
	Type    string `yaml:"type"`    // jsonl, csv, xml, edi
	Path    string `yaml:"path"`
	Dialect string `yaml:"dialect"` // optional, for edi
	XMLBlock string `yaml:"xml_block"` // optional, for xml (default Record)
}

// TargetConfig describes a single fan-out destination.
type TargetConfig struct {
	Type   string `yaml:"type"`   // local, s3, prometheus, elasticsearch
	Format string `yaml:"format"` // for local: jsonl or parquet
	Path   string `yaml:"path"`   // for local file

	// S3
	Bucket string `yaml:"bucket"`
	Key    string `yaml:"key"`

	// Prometheus Pushgateway
	URL string `yaml:"url"`

	// Elasticsearch
	Index string `yaml:"index"`
}

// LoadFanOutConfig reads and parses a fanout YAML file from path.
func LoadFanOutConfig(path string) (*FanOutConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c FanOutConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
