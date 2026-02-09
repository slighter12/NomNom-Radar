package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMetadata_Success(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create test metadata
	metadata := RoutingMetadata{
		Version: "1.0",
		Source: SourceInfo{
			Region:       "taiwan",
			URL:          "https://example.com/taiwan.osm.pbf",
			Filename:     "taiwan-latest.osm.pbf",
			SizeBytes:    325058560,
			OSMTimestamp: time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
		},
		Processing: ProcessingInfo{
			GeneratedAt:        time.Date(2025, 12, 18, 10, 30, 0, 0, time.UTC),
			OSM2CHVersion:      "1.7.6",
			CLIVersion:         "0.1.0",
			Profile:            "scooter",
			ContractionEnabled: true,
		},
		Output: OutputInfo{
			VerticesCount:  1234567,
			EdgesCount:     2345678,
			ShortcutsCount: 3456789,
		},
	}

	// Write to file
	data, err := json.MarshalIndent(metadata, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "metadata.json"), data, 0644)
	require.NoError(t, err)

	// Load and verify
	loaded, err := LoadMetadata(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "1.0", loaded.Version)
	assert.Equal(t, "taiwan", loaded.Source.Region)
	assert.Equal(t, "scooter", loaded.Processing.Profile)
	assert.Equal(t, int64(1234567), loaded.Output.VerticesCount)
}

func TestLoadMetadata_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadMetadata(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestLoadMetadata_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = LoadMetadata(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestRoutingMetadata_Validate(t *testing.T) {
	tests := []struct {
		name      string
		metadata  RoutingMetadata
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid metadata",
			metadata: RoutingMetadata{
				Version: "1.0",
				Source:  SourceInfo{Region: "taiwan"},
				Processing: ProcessingInfo{
					GeneratedAt: time.Now(),
				},
				Output: OutputInfo{
					VerticesCount: 1000,
					EdgesCount:    2000,
				},
			},
			expectErr: false,
		},
		{
			name: "missing version",
			metadata: RoutingMetadata{
				Source: SourceInfo{Region: "taiwan"},
				Processing: ProcessingInfo{
					GeneratedAt: time.Now(),
				},
				Output: OutputInfo{
					VerticesCount: 1000,
					EdgesCount:    2000,
				},
			},
			expectErr: true,
			errMsg:    "version",
		},
		{
			name: "missing region",
			metadata: RoutingMetadata{
				Version: "1.0",
				Processing: ProcessingInfo{
					GeneratedAt: time.Now(),
				},
				Output: OutputInfo{
					VerticesCount: 1000,
					EdgesCount:    2000,
				},
			},
			expectErr: true,
			errMsg:    "region",
		},
		{
			name: "missing generated_at",
			metadata: RoutingMetadata{
				Version: "1.0",
				Source:  SourceInfo{Region: "taiwan"},
				Output: OutputInfo{
					VerticesCount: 1000,
					EdgesCount:    2000,
				},
			},
			expectErr: true,
			errMsg:    "generated_at",
		},
		{
			name: "zero vertices",
			metadata: RoutingMetadata{
				Version: "1.0",
				Source:  SourceInfo{Region: "taiwan"},
				Processing: ProcessingInfo{
					GeneratedAt: time.Now(),
				},
				Output: OutputInfo{
					VerticesCount: 0,
					EdgesCount:    2000,
				},
			},
			expectErr: true,
			errMsg:    "vertices_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRoutingMetadata_GetAge(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	metadata := RoutingMetadata{
		Processing: ProcessingInfo{
			GeneratedAt: past,
		},
	}

	age := metadata.GetAge()
	assert.True(t, age >= 23*time.Hour && age <= 25*time.Hour)
}

func TestRoutingMetadata_Summary(t *testing.T) {
	metadata := RoutingMetadata{
		Source: SourceInfo{
			Region: "taiwan",
		},
		Processing: ProcessingInfo{
			Profile: "scooter",
		},
		Output: OutputInfo{
			VerticesCount:  1000,
			EdgesCount:     2000,
			ShortcutsCount: 3000,
		},
	}

	summary := metadata.Summary()
	assert.Equal(t, "taiwan", summary["region"])
	assert.Equal(t, "scooter", summary["profile"])
	assert.Equal(t, int64(1000), summary["vertices_count"])
}
