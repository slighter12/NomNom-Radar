package main

import (
	"context"
	"crypto/md5" // #nosec G501
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// httpClient is initialized once and reused to allow TCP connection reuse.
//
//nolint:gochecknoglobals // shared client is intentional and safe for concurrent use
var (
	httpClient *http.Client
	clientOnce sync.Once
)

func getHTTPClient() *http.Client {
	clientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: 30 * time.Minute,
		}
	})

	return httpClient
}

// DownloadConfig holds download configuration
type DownloadConfig struct {
	Region    string
	OutputDir string
	URL       string
	Filename  string
	Resume    bool
	Verify    bool
}

func runDownload(ctx context.Context, region, outputDir string) error {
	// Get region configuration
	regionConfig, exists := GetRegionConfig(region)
	if !exists {
		return errors.Errorf("unsupported region '%s'. Supported regions: %s", region, strings.Join(ListRegions(), ", "))
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
	if err := downloadFile(ctx, config); err != nil {
		return errors.Wrap(err, "failed to download file")
	}

	fmt.Printf("\nDownload completed successfully!\n")

	return nil
}

// downloadFile handles the actual download with progress tracking and resume support
func downloadFile(ctx context.Context, config DownloadConfig) error {
	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	outputPath := filepath.Join(config.OutputDir, config.Filename)
	existingSize := getExistingSize(config.Resume, outputPath)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	// Make request
	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to make request")
	}
	defer resp.Body.Close()

	if err := checkResponseStatus(resp, config); err != nil {
		return err
	}

	return performDownload(ctx, resp, outputPath, existingSize, config)
}

func getExistingSize(resume bool, path string) int64 {
	if resume {
		if info, err := os.Stat(path); err == nil {
			size := info.Size()
			fmt.Printf("Resuming download from %d bytes\n", size)

			return size
		}
	}

	return 0
}

func checkResponseStatus(resp *http.Response, config DownloadConfig) error {
	switch resp.StatusCode {
	case http.StatusRequestedRangeNotSatisfiable:
		fmt.Println("File is already complete")

		return verifyFileIntegrity(config)
	case http.StatusOK, http.StatusPartialContent:
		return nil
	default:
		return errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

func performDownload(ctx context.Context, resp *http.Response, path string, existingSize int64, config DownloadConfig) error {
	file, err := openOutputFile(path, existingSize, resp.StatusCode)
	if err != nil {
		return err
	}
	defer file.Close()

	totalSize := calculateTotalSize(resp)
	progress := &DownloadProgress{
		Total:      totalSize,
		Downloaded: existingSize,
		StartTime:  time.Now(),
	}

	// Use a context-aware reader to support cancellation during download
	reader := &contextReader{
		ctx: ctx,
		r:   resp.Body,
	}

	if _, err := io.Copy(io.MultiWriter(file, progress), reader); err != nil {
		return errors.Wrap(err, "failed to download file")
	}

	fmt.Println()

	if config.Verify {
		return verifyFileIntegrity(config)
	}

	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, errors.Wrap(err, "context canceled during read")
	}

	bytesRead, err := cr.r.Read(p)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return bytesRead, io.EOF
		}

		return bytesRead, errors.Wrap(err, "read failed")
	}

	return bytesRead, nil
}

func openOutputFile(path string, existingSize int64, statusCode int) (*os.File, error) {
	if existingSize > 0 && statusCode == http.StatusPartialContent {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open file for appending")
		}

		return file, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file")
	}

	return file, nil
}

func calculateTotalSize(resp *http.Response) int64 {
	if resp.StatusCode == http.StatusOK {
		if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
			if size, err := parseContentLength(contentLength); err == nil {
				return size
			}
		}
	}

	return -1 // Unknown or partial
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

// formatDuration formats duration into human readable format (e.g., "1h30m", "5m10s", "45s")
func formatDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}

	if duration < time.Hour {
		m := int(duration.Minutes())
		s := int(duration.Seconds()) % 60

		return fmt.Sprintf("%dm%ds", m, s)
	}

	h := int(duration.Hours())
	m := int(duration.Minutes()) % 60

	return fmt.Sprintf("%dh%dm", h, m)
}

// parseContentLength parses Content-Length header
func parseContentLength(length string) (int64, error) {
	size, err := strconv.ParseInt(length, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse content length")
	}

	return size, nil
}

// verifyFileIntegrity verifies the downloaded file integrity
func verifyFileIntegrity(config DownloadConfig) error {
	outputPath := filepath.Join(config.OutputDir, config.Filename)

	fmt.Printf("Verifying file integrity: %s\n", outputPath)

	file, err := os.Open(outputPath)
	if err != nil {
		return errors.Wrap(err, "failed to open file for verification")
	}
	defer file.Close()

	// Calculate MD5 and SHA256
	// #nosec G401
	md5Hash := md5.New()
	sha256Hash := sha256.New()

	if _, err := io.Copy(io.MultiWriter(md5Hash, sha256Hash), file); err != nil {
		return errors.Wrap(err, "failed to calculate checksums")
	}

	md5Sum := fmt.Sprintf("%x", md5Hash.Sum(nil))
	sha256Sum := fmt.Sprintf("%x", sha256Hash.Sum(nil))

	fmt.Printf("MD5:    %s\n", md5Sum)
	fmt.Printf("SHA256: %s\n", sha256Sum)

	// Note: In production, you would compare against expected checksums
	// For now, we just display them

	return nil
}
