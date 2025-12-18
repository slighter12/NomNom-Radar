package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runValidate(dir string) {
	fmt.Printf("Validating routing data in directory: %s\n", dir)

	if err := validateRoutingData(dir); err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Validation passed!")
}

func validateRoutingData(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	// Check for required files
	requiredFiles := []string{
		"metadata.json",
		"vertices.csv",
		"edges.csv",
		"shortcuts.csv",
	}

	fmt.Println("Checking required files...")
	for _, filename := range requiredFiles {
		filePath := filepath.Join(dir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("required file missing: %s", filename)
		}
		fmt.Printf("  ✅ %s\n", filename)
	}

	// Load and validate metadata
	fmt.Println("\nValidating metadata...")
	metadataPath := filepath.Join(dir, "metadata.json")
	metadata, err := LoadMetadataFromFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Validate metadata structure
	if metadata.Version != "1.0" {
		return fmt.Errorf("unsupported metadata version: %s", metadata.Version)
	}

	fmt.Printf("  ✅ Version: %s\n", metadata.Version)
	fmt.Printf("  ✅ Source: %s (%s)\n", metadata.Source.Filename, formatBytes(metadata.Source.SizeBytes))
	fmt.Printf("  ✅ Generated: %s\n", metadata.Processing.GeneratedAt.Format("2006-01-02 15:04:05"))

	// Validate output files exist and match metadata
	fmt.Println("\nValidating output files...")
	for filename, fileMeta := range metadata.Output.Files {
		filePath := filepath.Join(dir, filename)

		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("output file not found: %s", filename)
		}

		if info.Size() != fileMeta.SizeBytes {
			return fmt.Errorf("file size mismatch for %s: expected %d, got %d",
				filename, fileMeta.SizeBytes, info.Size())
		}

		fmt.Printf("  ✅ %s (%s)\n", filename, formatBytes(info.Size()))
	}

	// Validate CSV file formats (basic check)
	fmt.Println("\nValidating CSV formats...")
	csvChecks := map[string]func(string) error{
		"vertices.csv":  validateVerticesCSV,
		"edges.csv":     validateEdgesCSV,
		"shortcuts.csv": validateShortcutsCSV,
	}

	for filename, validator := range csvChecks {
		filePath := filepath.Join(dir, filename)
		if err := validator(filePath); err != nil {
			return fmt.Errorf("CSV validation failed for %s: %w", filename, err)
		}
		fmt.Printf("  ✅ %s format\n", filename)
	}

	// Check data consistency
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

	return nil
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
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read first line (header)
	header, err := readCSVLine(file)
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Check if all expected columns are present
	headerMap := make(map[string]bool)
	for _, col := range header {
		headerMap[col] = true
	}

	for _, expected := range expectedColumns {
		if !headerMap[expected] {
			return fmt.Errorf("missing required column: %s", expected)
		}
	}

	return nil
}

// readCSVLine reads a single line from CSV file and splits by comma
func readCSVLine(file *os.File) ([]string, error) {
	// This is a simple implementation - in production you'd use a proper CSV reader
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}

	line := string(buf[:n])
	// Find first newline
	for i, b := range buf {
		if b == '\n' {
			line = string(buf[:i])
			break
		}
	}

	// Simple CSV parsing (doesn't handle quotes, escapes, etc.)
	// In production, use encoding/csv package
	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				fields = append(fields, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields, nil
}

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
