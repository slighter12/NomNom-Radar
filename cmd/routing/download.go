package main

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadConfig holds download configuration
type DownloadConfig struct {
	Region    string
	OutputDir string
	URL       string
	Filename  string
	Resume    bool
	Verify    bool
}

func runDownload(region, outputDir string) {
	// Get region configuration
	regionConfig, exists := GetRegionConfig(region)
	if !exists {
		fmt.Printf("Error: Unsupported region '%s'\n", region)
		fmt.Printf("Supported regions: %s\n", strings.Join(ListRegions(), ", "))
		os.Exit(1)
	}

	// Create download config
	config := DownloadConfig{
		Region:    region,
		OutputDir: outputDir,
		URL:       regionConfig.URL,
		Filename:  regionConfig.Filename,
		Resume:    true, // Always enable resume
		Verify:    true, // Always verify after download
	}

	fmt.Printf("Downloading OSM data for %s\n", regionConfig.Name)
	fmt.Printf("Source: %s\n", config.URL)
	fmt.Printf("Output: %s/%s\n", config.OutputDir, config.Filename)
	fmt.Printf("Description: %s\n", regionConfig.Description)
	fmt.Println()

	// Download the file
	if err := downloadFile(config); err != nil {
		fmt.Printf("Error: Failed to download file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDownload completed successfully!\n")
}

// downloadFile handles the actual download with progress tracking and resume support
func downloadFile(config DownloadConfig) error {
	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(config.OutputDir, config.Filename)

	// Check if file already exists and get its size for resume
	var existingSize int64
	if config.Resume {
		if info, err := os.Stat(outputPath); err == nil {
			existingSize = info.Size()
			fmt.Printf("Resuming download from %d bytes\n", existingSize)
		}
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", config.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Range header for resume
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	// Make request
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == 416 {
		// 416 Range Not Satisfiable - file is already complete
		fmt.Println("File is already complete")
		return verifyFileIntegrity(config)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Open output file
	var file *os.File
	if existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
		file, err = os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		file, err = os.Create(outputPath)
		existingSize = 0 // Reset if not resuming
	}
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer file.Close()

	// Get total size
	totalSize := existingSize
	if resp.StatusCode == http.StatusOK {
		if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
			if size, err := parseContentLength(contentLength); err == nil {
				totalSize = size
			}
		}
	} else if resp.StatusCode == http.StatusPartialContent {
		// For partial content, we don't know the total size
		totalSize = -1
	}

	// Create progress reader
	progress := &DownloadProgress{
		Total:      totalSize,
		Downloaded: existingSize,
		StartTime:  time.Now(),
	}

	// Copy with progress tracking
	_, err = io.Copy(io.MultiWriter(file, progress), resp.Body)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	fmt.Println() // New line after progress bar

	// Verify file integrity if requested
	if config.Verify {
		return verifyFileIntegrity(config)
	}

	return nil
}

// DownloadProgress tracks download progress and displays a progress bar
type DownloadProgress struct {
	Total      int64
	Downloaded int64
	StartTime  time.Time
}

func (dp *DownloadProgress) Write(p []byte) (int, error) {
	n := len(p)
	dp.Downloaded += int64(n)

	dp.displayProgress()
	return n, nil
}

func (dp *DownloadProgress) displayProgress() {
	width := 50
	progress := float64(dp.Downloaded) / float64(dp.Total)

	if dp.Total <= 0 {
		// Unknown total size
		fmt.Printf("\rDownloaded: %s | ???%% complete", formatBytes(dp.Downloaded))
		return
	}

	// Calculate percentage
	percentage := int(progress * 100)

	// Create progress bar
	filled := int(progress * float64(width))
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)

	// Calculate speed and ETA
	elapsed := time.Since(dp.StartTime)
	speed := float64(dp.Downloaded) / elapsed.Seconds()
	eta := time.Duration(float64(dp.Total-dp.Downloaded)/speed) * time.Second

	fmt.Printf("\r[%s] %d%% | %s/%s | %s/s | ETA: %s",
		bar,
		percentage,
		formatBytes(dp.Downloaded),
		formatBytes(dp.Total),
		formatBytes(int64(speed)),
		formatDuration(eta),
	)
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

// formatDuration formats duration into human readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm%.0fs", d.Minutes(), d.Seconds()-d.Minutes()*60)
	}
	return fmt.Sprintf("%.0fh%.0fm", d.Hours(), d.Minutes()-d.Hours()*60)
}

// parseContentLength parses Content-Length header
func parseContentLength(length string) (int64, error) {
	var size int64
	_, err := fmt.Sscanf(length, "%d", &size)
	return size, err
}

// verifyFileIntegrity verifies the downloaded file integrity
func verifyFileIntegrity(config DownloadConfig) error {
	outputPath := filepath.Join(config.OutputDir, config.Filename)

	fmt.Printf("Verifying file integrity: %s\n", outputPath)

	file, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("failed to open file for verification: %w", err)
	}
	defer file.Close()

	// Calculate MD5 and SHA256
	md5Hash := md5.New()
	sha256Hash := sha256.New()

	if _, err := io.Copy(io.MultiWriter(md5Hash, sha256Hash), file); err != nil {
		return fmt.Errorf("failed to calculate checksums: %w", err)
	}

	md5Sum := fmt.Sprintf("%x", md5Hash.Sum(nil))
	sha256Sum := fmt.Sprintf("%x", sha256Hash.Sum(nil))

	fmt.Printf("MD5:    %s\n", md5Sum)
	fmt.Printf("SHA256: %s\n", sha256Sum)

	// Note: In production, you would compare against expected checksums
	// For now, we just display them

	return nil
}
