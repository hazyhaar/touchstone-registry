// CLAUDE:SUMMARY Manifest YAML schema defining dictionary metadata, CSV format spec, and pattern definitions.
package dict

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest describes a dictionary: its source, format, and how to interpret it.
type Manifest struct {
	ID           string           `yaml:"id" json:"id"`
	Version      string           `yaml:"version" json:"version"`
	Jurisdiction string           `yaml:"jurisdiction" json:"jurisdiction"`
	EntityType   string           `yaml:"entity_type" json:"entity_type"`
	Source       string           `yaml:"source" json:"source"`
	SourceURL    string           `yaml:"source_url" json:"source_url,omitempty"`
	License      string           `yaml:"license" json:"license"`
	DataFile     string           `yaml:"data_file" json:"data_file"`
	Method       string           `yaml:"method" json:"method,omitempty"`
	Format       FormatSpec       `yaml:"format" json:"-"`
	MetadataCols []MetadataColumn `yaml:"metadata_columns" json:"-"`
	Patterns     []PatternSpec    `yaml:"patterns" json:"patterns,omitempty"`
}

// PatternSpec defines a regex pattern with an optional checksum validator.
type PatternSpec struct {
	Name      string `yaml:"name" json:"name"`
	Regex     string `yaml:"regex" json:"regex"`
	Validator string `yaml:"validator,omitempty" json:"validator,omitempty"`
}

// FormatSpec describes the CSV layout.
type FormatSpec struct {
	Delimiter string `yaml:"delimiter"`
	Encoding  string `yaml:"encoding"`
	HasHeader bool   `yaml:"has_header"`
	KeyColumn string `yaml:"key_column"`
	Normalize string `yaml:"normalize"`
}

// MetadataColumn maps a logical name to a CSV column.
type MetadataColumn struct {
	Name   string `yaml:"name"`
	Column string `yaml:"column"`
}

// LoadManifest reads and parses a manifest.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if m.ID == "" {
		return nil, fmt.Errorf("manifest %s: missing id", path)
	}
	if m.DataFile == "" {
		m.DataFile = "data.csv"
	}
	return &m, nil
}
