package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runConvert(input, output string, contract bool) {
	fmt.Printf("Converting OSM data from %s to %s\n", input, output)
	fmt.Printf("Contraction enabled: %v\n", contract)
	fmt.Println()

	// Validate input file exists
	if _, err := os.Stat(input); os.IsNotExist(err) {
		fmt.Printf("Error: Input file does not exist: %s\n", input)
		os.Exit(1)
	}

	// Create output directory
	if err := os.MkdirAll(output, 0755); err != nil {
		fmt.Printf("Error: Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Run osm2ch conversion
	if err := runOSM2CHConversion(input, output, contract); err != nil {
		fmt.Printf("Error: Conversion failed: %v\n", err)
		os.Exit(1)
	}

	// Generate metadata
	if err := generateMetadata(input, output, contract); err != nil {
		fmt.Printf("Error: Failed to generate metadata: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Conversion completed successfully!")
}

// runOSM2CHConversion executes the osm2ch command
func runOSM2CHConversion(input, output string, contract bool) error {
	fmt.Println("Running osm2ch conversion...")

	// Build osm2ch command arguments
	args := []string{
		"--file", input,
		"--out", output,
		"--tags", "motorway,trunk,primary,secondary,tertiary,residential,service",
		"--geom-precision", "6",
	}

	if contract {
		args = append(args, "--contract=true")
	}

	fmt.Printf("Executing: osm2ch %v\n", args)

	// Execute osm2ch command
	cmd := exec.Command("osm2ch", args...)

	// Set working directory if needed
	cmd.Dir = "."

	// Capture output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("osm2ch execution failed after %v: %w", duration, err)
	}

	fmt.Printf("Conversion completed in %v\n", duration)
	return nil
}

// generateMetadata creates metadata.json for the converted data
func generateMetadata(input, output string, contract bool) error {
	fmt.Println("Generating metadata...")

	// Determine region from input filename (simple heuristic)
	region := "unknown"
	if strings.Contains(filepath.Base(input), "taiwan") {
		region = "taiwan"
	} else if strings.Contains(filepath.Base(input), "japan") {
		region = "japan"
	}

	// Generate metadata using the new structure
	metadata, err := GenerateMetadata(input, output, region, contract)
	if err != nil {
		return fmt.Errorf("failed to generate metadata: %w", err)
	}

	// Write metadata file
	metadataPath := filepath.Join(output, "metadata.json")
	if err := WriteMetadataToFile(metadata, metadataPath); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	fmt.Printf("Metadata written to: %s\n", metadataPath)
	return nil
}
