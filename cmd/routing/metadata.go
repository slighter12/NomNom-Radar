package main

import (
	"crypto/md5" // #nosec G501
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

// RoutingMetadata represents the metadata for routing data
type RoutingMetadata struct {
	Version    string             `json:"version"`
	Source     SourceMetadata     `json:"source"`
	Processing ProcessingMetadata `json:"processing"`
	Output     OutputMetadata     `json:"output"`
}

// SourceMetadata contains information about the source OSM data
type SourceMetadata struct {
	Region       string    `json:"region,omitempty"`
	URL          string    `json:"url,omitempty"`
	Filename     string    `json:"filename"`
	SizeBytes    int64     `json:"size_bytes"`
	MD5          string    `json:"md5"`
	SHA256       string    `json:"sha256"`
	OSMTimestamp time.Time `json:"osm_timestamp"`
}

// ProcessingMetadata contains information about the processing
type ProcessingMetadata struct {
	GeneratedAt        time.Time `json:"generated_at"`
	OSM2CHVersion      string    `json:"osm2ch_version,omitempty"`
	CLIVersion         string    `json:"cli_version,omitempty"`
	Profile            string    `json:"profile"`
	ContractionEnabled bool      `json:"contraction_enabled"`
	TagsIncluded       []string  `json:"tags_included"`
	GeomPrecision      int       `json:"geom_precision"`
}

// OutputMetadata contains information about the output files
type OutputMetadata struct {
	VerticesCount  int                     `json:"vertices_count"`
	EdgesCount     int                     `json:"edges_count"`
	ShortcutsCount int                     `json:"shortcuts_count"`
	Files          map[string]FileMetadata `json:"files"`
}

// FileMetadata contains metadata for individual output files
type FileMetadata struct {
	SizeBytes int64  `json:"size_bytes"`
	MD5       string `json:"md5,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

// GenerateMetadata creates metadata for the routing data
func GenerateMetadata(inputFile, outputDir string, region string, contract bool) (*RoutingMetadata, error) {
	// Get source information
	source, err := getSourceMetadata(inputFile, region)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get source metadata")
	}

	// Get processing information
	processing := ProcessingMetadata{
		GeneratedAt:        time.Now(),
		Profile:            "scooter",
		ContractionEnabled: contract,
		TagsIncluded:       []string{"motorway", "trunk", "primary", "secondary", "tertiary", "residential", "service"},
		GeomPrecision:      6,
	}

	// Get output information
	output, err := getOutputMetadata(outputDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get output metadata")
	}

	metadata := &RoutingMetadata{
		Version:    "1.0",
		Source:     *source,
		Processing: processing,
		Output:     *output,
	}

	return metadata, nil
}

// getSourceMetadata extracts metadata from the source OSM file
func getSourceMetadata(inputFile, region string) (*SourceMetadata, error) {
	info, err := os.Stat(inputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to stat input file")
	}

	// Calculate checksums
	md5Hash, sha256Hash, err := calculateFileChecksums(inputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate checksums")
	}

	// Get region config for URL
	var url string
	if regionConfig, exists := GetRegionConfig(region); exists {
		url = regionConfig.URL
	}

	source := &SourceMetadata{
		Region:       region,
		URL:          url,
		Filename:     filepath.Base(inputFile),
		SizeBytes:    info.Size(),
		MD5:          md5Hash,
		SHA256:       sha256Hash,
		OSMTimestamp: info.ModTime(),
	}

	return source, nil
}

// getOutputMetadata scans the output directory and collects file information
func getOutputMetadata(outputDir string) (*OutputMetadata, error) {
	output := &OutputMetadata{
		Files: make(map[string]FileMetadata),
	}

	expectedFiles := []string{"vertices.csv", "edges.csv", "shortcuts.csv"}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(outputDir, filename)

		info, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Warning: Expected output file not found: %s\n", filePath)

				continue
			}

			return nil, errors.Wrapf(err, "failed to stat file %s", filePath)
		}

		// Calculate checksums
		fileMeta := FileMetadata{
			SizeBytes: info.Size(),
		}
		md5Hash, sha256Hash, err := calculateFileChecksums(filePath)
		if err != nil {
			fmt.Printf("Warning: Failed to calculate checksum for %s: %v\n", filename, err)
		} else {
			fileMeta.MD5 = md5Hash
			fileMeta.SHA256 = sha256Hash
		}

		output.Files[filename] = fileMeta

		// Count lines for statistics (approximate)
		switch filename {
		case "vertices.csv":
			output.VerticesCount = estimateLineCount(filePath)
		case "edges.csv":
			output.EdgesCount = estimateLineCount(filePath)
		case "shortcuts.csv":
			output.ShortcutsCount = estimateLineCount(filePath)
		}
	}

	return output, nil
}

// calculateFileChecksums calculates MD5 and SHA256 checksums for a file
func calculateFileChecksums(filePath string) (string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to open file")
	}
	defer file.Close()

	// #nosec G401
	md5Hash := md5.New()
	sha256Hash := sha256.New()

	if _, err := io.Copy(io.MultiWriter(md5Hash, sha256Hash), file); err != nil {
		return "", "", errors.Wrap(err, "failed to calculate checksums")
	}

	md5Sum := fmt.Sprintf("%x", md5Hash.Sum(nil))
	sha256Sum := fmt.Sprintf("%x", sha256Hash.Sum(nil))

	return md5Sum, sha256Sum, nil
}

// estimateLineCount gives a rough estimate of line count in a file
func estimateLineCount(filePath string) int {
	file, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer file.Close()

	buf := make([]byte, 64*1024) // 64KB buffer
	count := 0

	for {
		n, err := file.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			break
		}

		for i := range n {
			if buf[i] == '\n' {
				count++
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return count
}

// WriteMetadataToFile writes metadata to a JSON file
func WriteMetadataToFile(metadata *RoutingMetadata, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create metadata file")
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(metadata); err != nil {
		return errors.Wrap(err, "failed to encode metadata")
	}

	return nil
}

// LoadMetadataFromFile loads metadata from a JSON file
func LoadMetadataFromFile(filePath string) (*RoutingMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open metadata file")
	}
	defer file.Close()

	var metadata RoutingMetadata
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&metadata); err != nil {
		return nil, errors.Wrap(err, "failed to decode metadata")
	}

	return &metadata, nil
}
