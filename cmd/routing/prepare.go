package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func runPrepare(ctx context.Context, region, output string) {
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
		fmt.Printf("Error: Unsupported region '%s'\n", region)
		os.Exit(1)
	}

	downloadConfig.URL = regionConfig.URL
	downloadConfig.Filename = regionConfig.Filename

	if err := downloadFile(ctx, downloadConfig); err != nil {
		fmt.Printf("Error: Download failed: %v\n", err)
		os.Exit(1)
	}

	inputFile := filepath.Join(tempDir, regionConfig.Filename)

	// Step 2: Convert OSM data
	fmt.Println("\n=== Step 2: Converting OSM data ===")
	if err := runOSM2CHConversion(ctx, inputFile, output, true); err != nil {
		fmt.Printf("Error: Conversion failed: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Generate metadata
	fmt.Println("\n=== Step 3: Generating metadata ===")
	if err := generateMetadata(inputFile, output, true); err != nil {
		fmt.Printf("Error: Failed to generate metadata: %v\n", err)
		os.Exit(1)
	}

	metadataPath := filepath.Join(output, "metadata.json")

	// Step 4: Validate results
	fmt.Println("\n=== Step 4: Validating results ===")
	if err := validateRoutingData(output); err != nil {
		fmt.Printf("Error: Validation failed: %v\n", err)
		os.Exit(1)
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✅ Preparation completed successfully in %v!\n", duration)
	fmt.Printf("Output directory: %s\n", output)
	fmt.Printf("Metadata: %s\n", metadataPath)
}
