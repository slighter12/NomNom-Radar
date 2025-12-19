package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"radar/internal/util"

	"github.com/pkg/errors"
)

func runValidate(dir string) error {
	fmt.Printf("Validating routing data in directory: %s\n", dir)

	if err := validateRoutingData(dir); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	fmt.Println("✅ Validation passed!")

	return nil
}

func validateRoutingData(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return errors.Wrapf(err, "directory does not exist: %s", dir)
	}

	// Load and validate metadata
	metadata, err := loadAndValidateMetadata(dir)
	if err != nil {
		return err
	}

	// Validate output files match metadata
	if err := validateFilesAgainstMetadata(dir, metadata); err != nil {
		return err
	}

	// Validate CSV file formats
	if err := validateCSVFormats(dir, metadata); err != nil {
		return err
	}

	// Check data consistency
	logConsistencyStats(metadata)

	return nil
}

func loadAndValidateMetadata(dir string) (*RoutingMetadata, error) {
	fmt.Println("\nValidating metadata...")

	metadataPath := filepath.Join(dir, "metadata.json")

	metadata, err := LoadMetadataFromFile(metadataPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load metadata")
	}

	// Validate metadata structure
	if metadata.Version != "1.0" {
		return nil, errors.Errorf("unsupported metadata version: %s", metadata.Version)
	}

	fmt.Printf("  ✅ Version: %s\n", metadata.Version)
	fmt.Printf("  ✅ Source: %s (%s)\n", metadata.Source.Filename, util.FormatBytes(metadata.Source.SizeBytes))
	fmt.Printf("  ✅ Generated: %s\n", metadata.Processing.GeneratedAt.Format("2006-01-02 15:04:05"))

	return metadata, nil
}

func validateFilesAgainstMetadata(dir string, metadata *RoutingMetadata) error {
	fmt.Println("\nValidating output files...")

	for filename, fileMeta := range metadata.Output.Files {
		filePath := filepath.Join(dir, filename)

		info, err := os.Stat(filePath)
		if err != nil {
			return errors.Wrapf(err, "output file not found: %s", filename)
		}

		if info.Size() != fileMeta.SizeBytes {
			return errors.Errorf("file size mismatch for %s: expected %d, got %d",
				filename, fileMeta.SizeBytes, info.Size())
		}

		// Validate checksum when available
		if fileMeta.SHA256 != "" {
			sha256Hash, err := util.CalculateFileChecksum(filePath)
			if err != nil {
				return errors.Wrapf(err, "failed to calculate checksum for %s", filename)
			}

			if sha256Hash != fileMeta.SHA256 {
				return errors.Errorf("checksum mismatch for %s: expected %s, got %s",
					filename, fileMeta.SHA256, sha256Hash)
			}
		}

		fmt.Printf("  ✅ %s (%s)\n", filename, util.FormatBytes(info.Size()))
	}

	return nil
}

func validateCSVFormats(dir string, metadata *RoutingMetadata) error {
	fmt.Println("\nValidating CSV formats...")

	csvChecks := map[string]func(string) error{
		"vertices.csv":  validateVerticesCSV,
		"edges.csv":     validateEdgesCSV,
		"shortcuts.csv": validateShortcutsCSV,
	}

	for filename, validator := range csvChecks {
		// Only validate if the file is present in metadata
		if _, exists := metadata.Output.Files[filename]; !exists {
			continue
		}

		filePath := filepath.Join(dir, filename)
		if err := validator(filePath); err != nil {
			return errors.Wrapf(err, "CSV validation failed for %s", filename)
		}

		fmt.Printf("  ✅ %s format\n", filename)
	}

	return nil
}

func logConsistencyStats(metadata *RoutingMetadata) {
	fmt.Println("\nChecking data consistency...")

	if metadata.Output.VerticesCount == 0 {
		fmt.Println("  ⚠️  Warning: No vertices count in metadata")
	} else {
		fmt.Printf("  ✅ Vertices: %d\n", metadata.Output.VerticesCount)
	}

	if metadata.Output.EdgesCount == 0 {
		fmt.Println("  ⚠️  Warning: No edges count in metadata")
	} else {
		fmt.Printf("  ✅ Edges: %d\n", metadata.Output.EdgesCount)
	}

	if metadata.Output.ShortcutsCount == 0 {
		fmt.Println("  ⚠️  Warning: No shortcuts count in metadata")
	} else {
		fmt.Printf("  ✅ Shortcuts: %d\n", metadata.Output.ShortcutsCount)
	}
}

// validateVerticesCSV performs basic validation of vertices.csv
func validateVerticesCSV(filePath string) error {
	return validateCSVHasColumns(filePath, []string{"id", "lat", "lng"})
}

// validateEdgesCSV performs basic validation of edges.csv
func validateEdgesCSV(filePath string) error {
	return validateCSVHasColumns(filePath, []string{"from", "to", "weight"})
}

// validateShortcutsCSV performs basic validation of shortcuts.csv
func validateShortcutsCSV(filePath string) error {
	return validateCSVHasColumns(filePath, []string{"from", "to", "weight"})
}

// validateCSVHasColumns checks if CSV file has the expected columns
func validateCSVHasColumns(filePath string, expectedColumns []string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}
	defer file.Close()

	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("file is empty or has no header")
		}

		return errors.Wrap(err, "failed to read CSV header")
	}

	headerSet := make(map[string]struct{}, len(header))
	for _, h := range header {
		headerSet[h] = struct{}{}
	}

	for _, expected := range expectedColumns {
		if _, ok := headerSet[expected]; !ok {
			return errors.Errorf("missing required column '%s' in header. Got: %v", expected, header)
		}
	}

	return nil
}

// formatBytes is defined in download.go
