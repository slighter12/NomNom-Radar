package util

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
)

// CalculateFileChecksum calculates the SHA256 checksum for a file.
func CalculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open file")
	}
	defer file.Close()

	sha256Hash := sha256.New()

	if _, err := io.Copy(sha256Hash, file); err != nil {
		return "", errors.Wrap(err, "failed to calculate checksum")
	}

	sha256Sum := fmt.Sprintf("%x", sha256Hash.Sum(nil))

	return sha256Sum, nil
}

// FormatBytes formats bytes into human readable format.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	const units = "KMGTPEZY"
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), units[exp])
}

// FormatDuration formats duration into human readable format (e.g., "1h30m", "5m10s", "45s").
func FormatDuration(duration time.Duration) string {
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
