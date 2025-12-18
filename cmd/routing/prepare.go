package main

import (
	"context"
	"fmt"
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
	tempDir := "/tmp"
	downloadConfig := DownloadConfig{
		Region:    region,
		OutputDir: tempDir,
		Resume:    true,
		Verify:    true,
	}

	regionConfig, exists := GetRegionConfig(region)
	if !exists {
		return errors.Errorf("unsupported region '%s'", region)
	}

	downloadConfig.URL = regionConfig.URL
	downloadConfig.Filename = regionConfig.Filename

	if err := downloadFile(ctx, downloadConfig); err != nil {
		return errors.Wrap(err, "download failed")
	}

	inputFile := filepath.Join(tempDir, regionConfig.Filename)

	// Step 2: Convert OSM data
	fmt.Println("\n=== Step 2: Converting OSM data ===")
	if err := runOSM2CHConversion(ctx, inputFile, output, true); err != nil {
		return errors.Wrap(err, "conversion failed")
	}

	// Step 3: Generate metadata
	fmt.Println("\n=== Step 3: Generating metadata ===")
	if err := generateMetadata(inputFile, output, region, true); err != nil {
		return errors.Wrap(err, "failed to generate metadata")
	}

	metadataPath := filepath.Join(output, "metadata.json")

	// Step 4: Validate results
	fmt.Println("\n=== Step 4: Validating results ===")
	if err := validateRoutingData(output); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✅ Preparation completed successfully in %v!\n", duration)
	fmt.Printf("Output directory: %s\n", output)
	fmt.Printf("Metadata: %s\n", metadataPath)

	return nil
}
