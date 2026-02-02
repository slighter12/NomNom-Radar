package pmtiles

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"radar/config"
	"radar/internal/usecase"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPMTilesPath returns the path to the test PMTiles file with file:// prefix
// The service will automatically extract the directory and tileset name from this path.
func testPMTilesPath() string {
	// Walk up from test directory to find the project root
	wd, _ := os.Getwd()
	for range 5 {
		path := filepath.Join(wd, "walking.pmtiles")
		if _, err := os.Stat(path); err == nil {
			// Return full file path - service will parse it
			return "file://" + path
		}
		wd = filepath.Dir(wd)
	}
	return ""
}

func skipIfNoTestFile(t *testing.T) string {
	path := testPMTilesPath()
	if path == "" {
		t.Skip("walking.pmtiles not found, skipping integration test")
	}
	return path
}

func createTestService(t *testing.T) usecase.RoutingUsecase {
	path := skipIfNoTestFile(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled:   true,
			Source:    path,
			RoadLayer: "roads",
			ZoomLevel: 15,
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)
	require.NoError(t, err)
	return svc
}

func TestNewPMTilesRoutingService_WithValidFile(t *testing.T) {
	path := skipIfNoTestFile(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled:   true,
			Source:    path,
			RoadLayer: "roads",
			ZoomLevel: 15,
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)

	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.True(t, svc.IsReady())
}

func TestNewPMTilesRoutingService_DefaultValues(t *testing.T) {
	path := skipIfNoTestFile(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled:   true,
			Source:    path,
			RoadLayer: "", // Should default to "transportation"
			ZoomLevel: 0,  // Should default to 14
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)

	require.NoError(t, err)
	require.NotNil(t, svc)

	// Verify defaults were applied
	pmSvc, ok := svc.(*pmtilesRoutingService)
	require.True(t, ok)
	assert.Equal(t, "transportation", pmSvc.roadLayer)
	assert.Equal(t, 14, pmSvc.zoomLevel)
}

func TestPMTilesRoutingService_IsReady(t *testing.T) {
	svc := createTestService(t)

	pmSvc, ok := svc.(*pmtilesRoutingService)
	require.True(t, ok)

	assert.True(t, pmSvc.IsReady())
	assert.NotNil(t, pmSvc.server)
}

func TestPMTilesService_OneToMany_Integration(t *testing.T) {
	svc := createTestService(t)
	ctx := context.Background()

	// Taipei area coordinates
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0350, Lng: 121.5670},
		{Lat: 25.0310, Lng: 121.5640},
	}

	result, err := svc.OneToMany(ctx, source, targets)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, source, result.Source)
	assert.Len(t, result.Results, 2)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}

func TestPMTilesService_OneToMany_EmptyTargets(t *testing.T) {
	svc := createTestService(t)
	ctx := context.Background()

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{}

	result, err := svc.OneToMany(ctx, source, targets)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Results)
}

func TestPMTilesService_CalculateDistance_Integration(t *testing.T) {
	svc := createTestService(t)
	ctx := context.Background()

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	target := usecase.Coordinate{Lat: 25.0350, Lng: 121.5670}

	result, err := svc.CalculateDistance(ctx, source, target)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, source, result.Source)
	assert.Equal(t, target, result.Target)
	// Distance should be reasonable (0-10km for nearby points)
	assert.GreaterOrEqual(t, result.DistanceKm, 0.0)
	assert.Less(t, result.DistanceKm, 10.0)
}

func TestPMTilesService_FindNearestNode_Integration(t *testing.T) {
	svc := createTestService(t)
	ctx := context.Background()

	coord := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}

	nodeInfo, found, err := svc.FindNearestNode(ctx, coord)

	require.NoError(t, err)
	// Node may or may not be found depending on coverage
	if found {
		assert.NotNil(t, nodeInfo)
		// Location should be reasonably close to query point
		assert.InDelta(t, coord.Lat, nodeInfo.Location.Lat, 0.01)
		assert.InDelta(t, coord.Lng, nodeInfo.Location.Lng, 0.01)
	}
}

func TestPMTilesService_LongDistanceRoute(t *testing.T) {
	svc := createTestService(t)

	pmSvc := svc.(*pmtilesRoutingService)
	ctx := context.Background()

	// Clear cache to measure fresh load
	pmSvc.tileCacheMu.Lock()
	pmSvc.tileCache = make(map[string]*RoadGraph)
	pmSvc.tileCacheMu.Unlock()

	// Long route across Taipei (~3km)
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5200}
	target := usecase.Coordinate{Lat: 25.0330, Lng: 121.5700}

	straightLine := haversineDistance(
		orb.Point{source.Lng, source.Lat},
		orb.Point{target.Lng, target.Lat},
	)
	t.Logf("Straight-line distance: %.2f km", straightLine/1000)

	result, err := svc.CalculateDistance(ctx, source, target)
	require.NoError(t, err)

	pmSvc.tileCacheMu.RLock()
	tilesLoaded := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()

	t.Logf("Tiles loaded for route: %d", tilesLoaded)
	t.Logf("Result: reachable=%v, distance=%.2f km, duration=%.2f min",
		result.IsReachable, result.DistanceKm, result.DurationMin)

	if result.IsReachable && result.DistanceKm > 0 {
		ratio := result.DistanceKm / (straightLine / 1000)
		t.Logf("Road/straight ratio: %.2f (typical urban: 1.2-1.5)", ratio)
	}
}

// TestPMTilesService_InspectFile inspects the PMTiles file metadata
func TestPMTilesService_InspectFile(t *testing.T) {
	path := skipIfNoTestFile(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	params := PMTilesServiceParams{
		Config: &config.PMTilesConfig{
			Enabled:   true,
			Source:    path,
			RoadLayer: "roads",
			ZoomLevel: 15,
		},
		Logger: logger,
	}

	svc, err := NewPMTilesRoutingService(params)
	require.NoError(t, err)

	pmSvc := svc.(*pmtilesRoutingService)
	ctx := context.Background()

	// Try different zoom levels to find available data
	t.Log("=== Testing different zoom levels ===")
	testCoord := orb.Point{121.5654, 25.0330} // Taipei

	for z := maptile.Zoom(10); z <= 16; z++ {
		tile := maptile.At(testCoord, z)
		data, err := pmSvc.fetchTile(ctx, tile)
		if err != nil {
			t.Logf("Zoom %d: tile %s - not found", z, tileKey(tile))
			continue
		}

		segments, parseErr := pmSvc.parser.ParseTile(data, tile)
		segCount := 0
		if parseErr == nil {
			segCount = len(segments)
		}

		t.Logf("Zoom %d: tile %s - %d bytes, %d segments", z, tileKey(tile), len(data), segCount)
	}

	// Try different layer names
	t.Log("\n=== Testing different layer names ===")
	tile := maptile.At(testCoord, 15)
	data, err := pmSvc.fetchTile(ctx, tile)
	if err == nil && len(data) > 0 {
		t.Logf("Tile %s loaded, %d bytes", tileKey(tile), len(data))

		layerNames := []string{"transportation", "road", "roads", "highway", "path", "footway", "walking", "route", "street", "network"}
		for _, layerName := range layerNames {
			parser := NewMVTParser(layerName)
			segments, _ := parser.ParseTile(data, tile)
			if len(segments) > 0 {
				t.Logf("  Layer '%s': %d segments found", layerName, len(segments))
			}
		}

		t.Log("  Listing all layers in tile:")
		layers, parseErr := listMVTLayers(data)
		if parseErr != nil {
			t.Logf("  Error parsing MVT: %v", parseErr)
		} else {
			for _, l := range layers {
				t.Logf("    - Layer '%s': %d features", l.name, l.featureCount)
			}
		}
	}
}

func TestPMTilesService_TileDataSize(t *testing.T) {
	svc := createTestService(t)

	pmSvc := svc.(*pmtilesRoutingService)
	ctx := context.Background()

	testPoints := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{"Taipei Main Station", 25.0478, 121.5170},
		{"Taipei 101", 25.0339, 121.5645},
		{"Xinyi District", 25.0276, 121.5649},
		{"Daan District", 25.0268, 121.5435},
		{"Zhongshan District", 25.0630, 121.5225},
	}

	testZoom := maptile.Zoom(15)

	var totalDataSize int64
	var tileCount int

	for _, tp := range testPoints {
		tile := maptile.At(orb.Point{tp.lng, tp.lat}, testZoom)
		data, err := pmSvc.fetchTile(ctx, tile)

		if err != nil {
			t.Logf("Tile %s (%s): fetch error - %v", tp.name, tileKey(tile), err)
			continue
		}

		dataSize := len(data)
		totalDataSize += int64(dataSize)
		tileCount++

		segments, parseErr := pmSvc.parser.ParseTile(data, tile)
		segmentCount := 0
		if parseErr == nil {
			segmentCount = len(segments)
		}

		t.Logf("Tile %s (%s): %d bytes, %d road segments",
			tp.name, tileKey(tile), dataSize, segmentCount)
	}

	if tileCount > 0 {
		avgSize := totalDataSize / int64(tileCount)
		t.Logf("\n=== Summary ===")
		t.Logf("Total tiles fetched: %d", tileCount)
		t.Logf("Total data size: %d bytes (%.2f KB)", totalDataSize, float64(totalDataSize)/1024)
		t.Logf("Average tile size: %d bytes (%.2f KB)", avgSize, float64(avgSize)/1024)
	}
}

func TestPMTilesService_MultiTileFetch(t *testing.T) {
	svc := createTestService(t)

	pmSvc := svc.(*pmtilesRoutingService)
	ctx := context.Background()

	// Test area: Taipei city center (~2km x 2km)
	minLat, maxLat := 25.02, 25.05
	minLng, maxLng := 121.51, 121.57

	tiles := getTilesForBounds(minLat, maxLat, minLng, maxLng, 15)

	t.Logf("Area bounds: [%.4f, %.4f] to [%.4f, %.4f]", minLat, minLng, maxLat, maxLng)
	t.Logf("Tiles needed: %d", len(tiles))

	var totalDataSize int64
	var successCount int
	var totalSegments int

	for _, tile := range tiles {
		data, err := pmSvc.fetchTile(ctx, tile)
		if err != nil {
			continue
		}

		successCount++
		totalDataSize += int64(len(data))

		segments, parseErr := pmSvc.parser.ParseTile(data, tile)
		if parseErr == nil {
			totalSegments += len(segments)
		}
	}

	t.Logf("\n=== Multi-Tile Fetch Summary ===")
	t.Logf("Tiles requested: %d", len(tiles))
	t.Logf("Tiles successful: %d", successCount)
	t.Logf("Total data fetched: %d bytes (%.2f KB, %.2f MB)",
		totalDataSize, float64(totalDataSize)/1024, float64(totalDataSize)/1024/1024)
	t.Logf("Total road segments: %d", totalSegments)

	if successCount > 0 {
		avgSize := totalDataSize / int64(successCount)
		t.Logf("Average tile size: %d bytes (%.2f KB)", avgSize, float64(avgSize)/1024)
	}
}
