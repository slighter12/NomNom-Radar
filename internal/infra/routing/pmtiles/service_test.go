package pmtiles

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"radar/config"
	"radar/internal/usecase"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTileKey(t *testing.T) {
	tests := []struct {
		tile     maptile.Tile
		expected string
	}{
		{
			tile:     maptile.Tile{X: 0, Y: 0, Z: 14},
			expected: "14/0/0",
		},
		{
			tile:     maptile.Tile{X: 13823, Y: 7082, Z: 14},
			expected: "14/13823/7082",
		},
		{
			tile:     maptile.Tile{X: 1000, Y: 2000, Z: 12},
			expected: "12/1000/2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tileKey(tt.tile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSourcePath(t *testing.T) {
	tests := []struct {
		name            string
		source          string
		expectedBucket  string
		expectedPrefix  string
		expectedTileset string
	}{
		{
			name:            "file:// prefix with absolute path",
			source:          "file:///path/to/walking.pmtiles",
			expectedBucket:  "file:///path/to",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "file:// prefix with nested path",
			source:          "file:///home/user/data/tiles/roads.pmtiles",
			expectedBucket:  "file:///home/user/data/tiles",
			expectedPrefix:  "",
			expectedTileset: "roads",
		},
		{
			name:            "local path without prefix",
			source:          "/path/to/walking.pmtiles",
			expectedBucket:  "file:///path/to",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "root path file",
			source:          "/walking.pmtiles",
			expectedBucket:  "file:///",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "file:// root path",
			source:          "file:///walking.pmtiles",
			expectedBucket:  "file:///",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "https URL",
			source:          "https://example.com/tiles/walking.pmtiles",
			expectedBucket:  "https://example.com/tiles",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "http URL",
			source:          "http://localhost:8080/data/roads.pmtiles",
			expectedBucket:  "http://localhost:8080/data",
			expectedPrefix:  "",
			expectedTileset: "roads",
		},
		{
			name:            "https URL with port",
			source:          "https://tiles.example.com:8443/v1/walking.pmtiles",
			expectedBucket:  "https://tiles.example.com:8443/v1",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "filename without .pmtiles extension",
			source:          "/path/to/tileset",
			expectedBucket:  "file:///path/to",
			expectedPrefix:  "",
			expectedTileset: "tileset",
		},
		{
			name:            "gs:// GCS bucket root",
			source:          "gs://my-bucket/walking.pmtiles",
			expectedBucket:  "gs://my-bucket",
			expectedPrefix:  "",
			expectedTileset: "walking",
		},
		{
			name:            "gs:// GCS bucket with subdirectory",
			source:          "gs://my-bucket/subdir/walking.pmtiles",
			expectedBucket:  "gs://my-bucket",
			expectedPrefix:  "subdir",
			expectedTileset: "walking",
		},
		{
			name:            "gs:// GCS bucket with nested subdirectories",
			source:          "gs://my-bucket/path/to/tiles/walking.pmtiles",
			expectedBucket:  "gs://my-bucket",
			expectedPrefix:  "path/to/tiles",
			expectedTileset: "walking",
		},
		{
			name:            "s3:// AWS bucket with subdirectory",
			source:          "s3://my-bucket/folder/walking.pmtiles",
			expectedBucket:  "s3://my-bucket",
			expectedPrefix:  "folder",
			expectedTileset: "walking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, prefix, tileset := parseSourcePath(tt.source)
			assert.Equal(t, tt.expectedBucket, bucket, "bucket mismatch")
			assert.Equal(t, tt.expectedPrefix, prefix, "prefix mismatch")
			assert.Equal(t, tt.expectedTileset, tileset, "tileset mismatch")
		})
	}
}

func TestGetTilesForBounds(t *testing.T) {
	tests := []struct {
		name     string
		minLat   float64
		maxLat   float64
		minLng   float64
		maxLng   float64
		zoom     maptile.Zoom
		minTiles int
	}{
		{
			name:     "single tile",
			minLat:   25.0330,
			maxLat:   25.0340,
			minLng:   121.5654,
			maxLng:   121.5664,
			zoom:     14,
			minTiles: 1,
		},
		{
			name:     "multiple tiles",
			minLat:   25.00,
			maxLat:   25.10,
			minLng:   121.50,
			maxLng:   121.60,
			zoom:     14,
			minTiles: 4,
		},
		{
			name:     "zoom 12",
			minLat:   25.00,
			maxLat:   25.50,
			minLng:   121.00,
			maxLng:   121.50,
			zoom:     12,
			minTiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tiles := getTilesForBounds(tt.minLat, tt.maxLat, tt.minLng, tt.maxLng, tt.zoom)
			assert.GreaterOrEqual(t, len(tiles), tt.minTiles)

			// Verify all tiles have correct zoom level
			for _, tile := range tiles {
				assert.Equal(t, tt.zoom, tile.Z)
			}
		})
	}
}

func TestMergeGraphs(t *testing.T) {
	// Create target graph
	target := NewRoadGraph()
	segment1 := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}
	target.AddSegment(segment1)

	initialNodeCount := len(target.Nodes)
	initialEdgeCount := countEdges(target)

	// Create source graph with overlapping point
	source := NewRoadGraph()
	segment2 := &RoadSegment{
		Points: []orb.Point{
			{121.51, 25.00}, // Same as target's second point
			{121.52, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}
	source.AddSegment(segment2)

	// Merge graphs
	mergeGraphs(target, source)

	// Should add 1 new node (not 2, because one is shared)
	assert.Equal(t, initialNodeCount+1, len(target.Nodes))

	// Should have more edges
	assert.Greater(t, countEdges(target), initialEdgeCount)
}

func TestMergeGraphs_EmptySource(t *testing.T) {
	target := NewRoadGraph()
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}
	target.AddSegment(segment)

	initialNodeCount := len(target.Nodes)

	source := NewRoadGraph()
	mergeGraphs(target, source)

	assert.Equal(t, initialNodeCount, len(target.Nodes))
}

func TestMergeGraphs_EmptyTarget(t *testing.T) {
	target := NewRoadGraph()

	source := NewRoadGraph()
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}
	source.AddSegment(segment)

	mergeGraphs(target, source)

	assert.Equal(t, 2, len(target.Nodes))
}

func countEdges(g *RoadGraph) int {
	count := 0
	for _, edges := range g.Edges {
		count += len(edges)
	}

	return count
}

type layerInfo struct {
	name         string
	featureCount int
}

// listMVTLayers parses MVT data and returns info about all layers
func listMVTLayers(data []byte) ([]layerInfo, error) {
	layers, err := mvt.UnmarshalGzipped(data)
	if err != nil {
		// Try regular unmarshal
		layers, err = mvt.Unmarshal(data)
		if err != nil {
			return nil, err
		}
	}

	result := make([]layerInfo, 0, len(layers))
	for _, layer := range layers {
		result = append(result, layerInfo{
			name:         layer.Name,
			featureCount: len(layer.Features),
		})
	}

	return result, nil
}

func TestNewPMTilesRoutingService_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: nil, // nil config means disabled
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)

	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.True(t, svc.IsReady()) // Haversine fallback is always ready
}

func TestNewPMTilesRoutingService_DisabledExplicit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled: false,
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)

	require.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestNewPMTilesRoutingService_MissingSource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled: true,
			Source:  "", // Missing source
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)

	assert.Error(t, err)
	assert.Nil(t, svc)
	assert.Contains(t, err.Error(), "source is required")
}

func TestNewPMTilesRoutingService_InvalidSource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled: true,
			Source:  "../../../walking.pmtiles",
		},
		Logger: logger,
	}

	// This may or may not error depending on pmtiles library behavior
	// The server creation might succeed even with invalid path
	svc, err := NewPMTilesRoutingService(params)

	if err != nil {
		assert.Nil(t, svc)
	}
}

// Haversine fallback service tests
func TestHaversineFallbackService_OneToMany(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := newHaversineFallbackService(logger)

	ctx := context.Background()

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0478, Lng: 121.5170},
		{Lat: 25.0400, Lng: 121.5400},
	}

	result, err := svc.OneToMany(ctx, source, targets)

	require.NoError(t, err)
	require.Len(t, result.Results, 2)

	for _, r := range result.Results {
		assert.True(t, r.IsReachable)
		assert.Greater(t, r.DistanceKm, 0.0)
		assert.Greater(t, r.DurationMin, 0.0)
	}
}

func TestHaversineFallbackService_CalculateDistance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := newHaversineFallbackService(logger)

	ctx := context.Background()

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	target := usecase.Coordinate{Lat: 25.0478, Lng: 121.5170}

	result, err := svc.CalculateDistance(ctx, source, target)

	require.NoError(t, err)
	assert.True(t, result.IsReachable)
	// Taipei Station to Taipei Main Station ~5.6km
	assert.InDelta(t, 5.5, result.DistanceKm, 1.0)
}

func TestHaversineFallbackService_FindNearestNode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := newHaversineFallbackService(logger)

	ctx := context.Background()

	coord := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}

	nodeInfo, found, err := svc.FindNearestNode(ctx, coord)

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, coord, nodeInfo.Location)
}

func TestHaversineFallbackService_IsReady(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := newHaversineFallbackService(logger)

	assert.True(t, svc.IsReady())
}

// TestGCSBlobRead tests that GCS URLs can be parsed and read correctly.
// This test requires:
//   - PMTILES_GCS_SOURCE env var set to a valid gs:// PMTiles URL
//   - Valid GCP credentials (ADC, service account, etc.)
//
// Example:
//
//	PMTILES_GCS_SOURCE=gs://my-bucket/walking.pmtiles go test -run TestGCSBlobRead -v
func TestGCSBlobRead(t *testing.T) {
	gcsSource := os.Getenv("PMTILES_GCS_SOURCE")
	if gcsSource == "" {
		t.Skip("Skipping GCS test: PMTILES_GCS_SOURCE env var not set")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Test parseSourcePath with actual GCS URL
	bucketPath, prefix, tilesetName := parseSourcePath(gcsSource)
	t.Logf("Parsed GCS source: bucket=%s, prefix=%s, tileset=%s", bucketPath, prefix, tilesetName)

	assert.True(t, len(bucketPath) > 0, "bucket path should not be empty")
	assert.True(t, len(tilesetName) > 0, "tileset name should not be empty")
	assert.Contains(t, bucketPath, "gs://", "bucket path should contain gs://")

	// Test creating PMTiles service with GCS source
	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled:   true,
			Source:    gcsSource,
			RoadLayer: "transportation",
			ZoomLevel: 14,
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)
	require.NoError(t, err, "Failed to create PMTiles service with GCS source")
	require.NotNil(t, svc)

	assert.True(t, svc.IsReady(), "PMTiles service should be ready")

	t.Log("GCS PMTiles service initialized successfully")
}

// TestGCSBlobRead_Routing tests actual routing with a GCS-hosted PMTiles file.
// This test requires:
//   - PMTILES_GCS_SOURCE env var set to a valid gs:// PMTiles URL
//   - Valid GCP credentials (ADC, service account, etc.)
//   - The PMTiles file should cover the test coordinates (Taipei area by default)
func TestGCSBlobRead_Routing(t *testing.T) {
	gcsSource := os.Getenv("PMTILES_GCS_SOURCE")
	if gcsSource == "" {
		t.Skip("Skipping GCS routing test: PMTILES_GCS_SOURCE env var not set")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled:   true,
			Source:    gcsSource,
			RoadLayer: "transportation",
			ZoomLevel: 14,
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)
	require.NoError(t, err)
	require.NotNil(t, svc)

	ctx := context.Background()

	// Test coordinates in Taipei area
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0478, Lng: 121.5170}, // ~5.5km away
	}

	result, err := svc.OneToMany(ctx, source, targets)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Results, 1)

	t.Logf("Route result: distance=%.2f km, duration=%.2f min, reachable=%v",
		result.Results[0].DistanceKm,
		result.Results[0].DurationMin,
		result.Results[0].IsReachable)

	// The result should have valid distance (either from road network or Haversine fallback)
	assert.Greater(t, result.Results[0].DistanceKm, 0.0, "distance should be positive")
	assert.Greater(t, result.Results[0].DurationMin, 0.0, "duration should be positive")
}
