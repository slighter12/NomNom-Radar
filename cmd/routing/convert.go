package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

func runConvert(ctx context.Context, input, output, region string, contract bool) error {
	fmt.Printf("Converting OSM data from %s to %s\n", input, output)
	fmt.Printf("Region: %s\n", region)
	fmt.Printf("Contraction enabled: %v\n", contract)
	fmt.Println()

	// Validate input file exists
	if _, err := os.Stat(input); os.IsNotExist(err) {
		return errors.Errorf("input file does not exist: %s", input)
	}

	// Create output directory
	if err := os.MkdirAll(output, 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	// Run osm2ch conversion
	if err := runOSM2CHConversion(ctx, input, output, contract); err != nil {
		return errors.Wrap(err, "conversion failed")
	}

	// Generate metadata
	if err := generateMetadata(input, output, region, contract); err != nil {
		return errors.Wrap(err, "failed to generate metadata")
	}

	fmt.Println("Conversion completed successfully!")

	return nil
}

// runOSM2CHConversion executes the osm2ch command
func runOSM2CHConversion(ctx context.Context, input, output string, contract bool) error {
	fmt.Println("Running osm2ch conversion...")

	// Check if osm2ch is available
	if _, err := exec.LookPath("osm2ch"); err != nil {
		return errors.New("osm2ch command not found in PATH. Please install osm2ch first: go install github.com/LdDl/osm2ch/cmd/osm2ch@latest")
	}

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
	cmd := exec.CommandContext(ctx, "osm2ch", args...)

	// Set working directory if needed
	cmd.Dir = "."

	// Capture output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		return errors.Wrapf(err, "osm2ch execution failed after %v", duration)
	}

	fmt.Printf("Conversion completed in %v\n", duration)

	return nil
}

// generateMetadata creates metadata.json for the converted data
func generateMetadata(input, output, region string, contract bool) error {
	fmt.Println("Generating metadata...")

	// Generate metadata using the new structure
	metadata, err := GenerateMetadata(input, output, region, contract)
	if err != nil {
		return errors.Wrap(err, "failed to generate metadata")
	}

	// Write metadata file
	metadataPath := filepath.Join(output, "metadata.json")
	if err := WriteMetadataToFile(metadata, metadataPath); err != nil {
		return errors.Wrap(err, "failed to write metadata file")
	}

	fmt.Printf("Metadata written to: %s\n", metadataPath)

	return nil
}
