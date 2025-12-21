package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

// RoutingMetadata represents the metadata for routing data files
// This tracks the provenance and version of the preprocessed OSM data
type RoutingMetadata struct {
	Version    string         `json:"version"`
	Source     SourceInfo     `json:"source"`
	Processing ProcessingInfo `json:"processing"`
	Output     OutputInfo     `json:"output"`
}

// SourceInfo contains information about the source OSM data
type SourceInfo struct {
	Region       string    `json:"region"`
	URL          string    `json:"url"`
	Filename     string    `json:"filename"`
	SizeBytes    int64     `json:"size_bytes"`
	MD5          string    `json:"md5,omitempty"`
	SHA256       string    `json:"sha256,omitempty"`
	OSMTimestamp time.Time `json:"osm_timestamp,omitzero"`
}

// ProcessingInfo contains information about the preprocessing run
type ProcessingInfo struct {
	GeneratedAt        time.Time `json:"generated_at"`
	OSM2CHVersion      string    `json:"osm2ch_version"`
	CLIVersion         string    `json:"cli_version"`
	Profile            string    `json:"profile"`
	ContractionEnabled bool      `json:"contraction_enabled"`
	TagsIncluded       []string  `json:"tags_included,omitempty"`
}

// OutputInfo contains information about the generated output files
type OutputInfo struct {
	VerticesCount  int64                `json:"vertices_count"`
	EdgesCount     int64                `json:"edges_count"`
	ShortcutsCount int64                `json:"shortcuts_count"`
	Files          map[string]*FileInfo `json:"files,omitempty"`
}

// FileInfo contains checksum information for a single output file
type FileInfo struct {
	SizeBytes int64  `json:"size_bytes"`
	MD5       string `json:"md5,omitempty"`
}

// LoadMetadata loads and parses the metadata.json file from the given directory
func LoadMetadata(dataDir string) (*RoutingMetadata, error) {
	metadataPath := filepath.Join(dataDir, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(err, "metadata.json not found in routing data directory")
		}

		return nil, errors.Wrap(err, "failed to read metadata.json")
	}

	var metadata RoutingMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, errors.Wrap(err, "failed to parse metadata.json")
	}

	return &metadata, nil
}

// Validate checks if the metadata is valid and complete
func (m *RoutingMetadata) Validate() error {
	if m.Version == "" {
		return errors.New("metadata version is required")
	}

	if m.Source.Region == "" {
		return errors.New("source region is required")
	}

	if m.Processing.GeneratedAt.IsZero() {
		return errors.New("processing generated_at timestamp is required")
	}

	if m.Output.VerticesCount <= 0 {
		return errors.New("output vertices_count must be positive")
	}

	if m.Output.EdgesCount <= 0 {
		return errors.New("output edges_count must be positive")
	}

	return nil
}

// GetAge returns the age of the routing data since generation
func (m *RoutingMetadata) GetAge() time.Duration {
	return time.Since(m.Processing.GeneratedAt)
}

// Summary returns a brief summary of the metadata for logging
func (m *RoutingMetadata) Summary() map[string]any {
	return map[string]any{
		"region":          m.Source.Region,
		"osm_timestamp":   m.Source.OSMTimestamp,
		"generated_at":    m.Processing.GeneratedAt,
		"profile":         m.Processing.Profile,
		"vertices_count":  m.Output.VerticesCount,
		"edges_count":     m.Output.EdgesCount,
		"shortcuts_count": m.Output.ShortcutsCount,
	}
}
