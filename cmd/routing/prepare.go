package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

func runPrepare(ctx context.Context, region, output string) error {
	fmt.Printf("Preparing routing data for region: %s\n", region)
	fmt.Printf("Output directory: %s\n", output)
	fmt.Println()

	startTime := time.Now()

	// Step 1: Download OSM data
	fmt.Println("=== Step 1: Downloading OSM data ===")
	tempDir := os.TempDir()
	if err := runDownload(ctx, region, tempDir); err != nil {
		return errors.Wrap(err, "download failed")
	}

	regionConfig, _ := GetRegionConfig(region) // validated inside runDownload
	inputFile := filepath.Join(tempDir, regionConfig.Filename)

	// Step 1.5: Cache check to potentially skip conversion
	if ok, reason := canSkipConversion(inputFile, output, true); ok {
		fmt.Printf("\nCache hit: %s\n", reason)
		fmt.Println("✅ Existing outputs match source checksum; skipping conversion and metadata generation.")

		metadataPath := filepath.Join(output, "metadata.json")
		duration := time.Since(startTime)
		fmt.Printf("\n✅ Preparation completed successfully in %v!\n", duration)
		fmt.Printf("Output directory: %s\n", output)
		fmt.Printf("Metadata: %s\n", metadataPath)

		return nil
	}

	// Step 2: Convert OSM data and generate metadata
	fmt.Println("\n=== Step 2: Converting OSM data and generating metadata ===")
	if err := runConvert(ctx, inputFile, output, region, true); err != nil {
		return errors.Wrap(err, "conversion failed")
	}

	metadataPath := filepath.Join(output, "metadata.json")

	// Step 3: Validate results
	fmt.Println("\n=== Step 3: Validating results ===")
	if err := validateRoutingData(output); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✅ Preparation completed successfully in %v!\n", duration)
	fmt.Printf("Output directory: %s\n", output)
	fmt.Printf("Metadata: %s\n", metadataPath)

	return nil
}

// canSkipConversion checks whether existing outputs already match the current source file.
// It returns (true, reason) if conversion can be skipped.
func canSkipConversion(inputFile, outputDir string, contract bool) (bool, string) {
	metadataPath := filepath.Join(outputDir, "metadata.json")

	metadata, err := LoadMetadataFromFile(metadataPath)
	if err != nil {
		return false, fmt.Sprintf("metadata not usable (%v)", err)
	}

	if metadata.Source.SHA256 == "" {
		return false, "metadata missing source checksum"
	}

	currentSHA, err := calculateFileChecksums(inputFile)
	if err != nil {
		return false, fmt.Sprintf("failed to checksum source: %v", err)
	}

	if currentSHA != metadata.Source.SHA256 {
		return false, "source checksum differs from metadata"
	}

	if metadata.Processing.ContractionEnabled != contract {
		return false, "contraction flag differs from metadata"
	}

	if err := validateFilesAgainstMetadata(outputDir, metadata); err != nil {
		return false, fmt.Sprintf("output files failed validation: %v", err)
	}

	return true, "source and outputs already validated against metadata"
}
