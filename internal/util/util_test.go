package util

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{name: "zero bytes", bytes: 0, expected: "0 B"},
		{name: "bytes under kilobyte", bytes: 512, expected: "512 B"},
		{name: "exact kilobyte", bytes: 1024, expected: "1.0 KB"},
		{name: "fractional kilobyte", bytes: 1536, expected: "1.5 KB"},
		{name: "megabyte", bytes: 1024 * 1024, expected: "1.0 MB"},
		{name: "gigabyte", bytes: 5 * 1024 * 1024 * 1024, expected: "5.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatBytes(tt.bytes); got != tt.expected {
				t.Fatalf("FormatBytes(%d) = %s, want %s", tt.bytes, got, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{name: "under one minute", duration: 45 * time.Second, expected: "45s"},
		{name: "rounded second to minute", duration: 59*time.Second + 500*time.Millisecond, expected: "1m0s"},
		{name: "minutes and seconds", duration: 2*time.Minute + 30*time.Second, expected: "2m30s"},
		{name: "hours and minutes", duration: time.Hour + 30*time.Minute, expected: "1h30m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatDuration(tt.duration); got != tt.expected {
				t.Fatalf("FormatDuration(%s) = %s, want %s", tt.duration, got, tt.expected)
			}
		})
	}
}
