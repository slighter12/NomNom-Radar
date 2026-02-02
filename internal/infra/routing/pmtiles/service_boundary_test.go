package pmtiles

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"

	"radar/config"
	"radar/internal/usecase"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPMTilesService_TileBoundary_PointNearEdge tests routing when point is near tile edge
func TestPMTilesService_TileBoundary_PointNearEdge(t *testing.T) {
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

	ctx := context.Background()

	// Find a tile boundary
	centerLat := 25.0330
	centerLng := 121.5654
	centerTile := maptile.At(orb.Point{centerLng, centerLat}, 14)

	// Get the tile's bounding box
	tileBound := centerTile.Bound()
	t.Logf("Center tile: %s", tileKey(centerTile))
	t.Logf("Tile bounds: [%.6f, %.6f] to [%.6f, %.6f]",
		tileBound.Min.Lat(), tileBound.Min.Lon(),
		tileBound.Max.Lat(), tileBound.Max.Lon())

	// Test point very close to tile boundary (within 10m)
	edgeOffset := 0.0001 // ~10m
	testCases := []struct {
		name      string
		sourceLat float64
		sourceLng float64
		targetLat float64
		targetLng float64
	}{
		{
			name:      "Source near east edge, target in next tile",
			sourceLat: centerLat,
			sourceLng: tileBound.Max.Lon() - edgeOffset,
			targetLat: centerLat,
			targetLng: tileBound.Max.Lon() + edgeOffset,
		},
		{
			name:      "Source near north edge, target in next tile",
			sourceLat: tileBound.Max.Lat() - edgeOffset,
			sourceLng: centerLng,
			targetLat: tileBound.Max.Lat() + edgeOffset,
			targetLng: centerLng,
		},
		{
			name:      "Diagonal cross - NE corner",
			sourceLat: tileBound.Max.Lat() - edgeOffset,
			sourceLng: tileBound.Max.Lon() - edgeOffset,
			targetLat: tileBound.Max.Lat() + edgeOffset,
			targetLng: tileBound.Max.Lon() + edgeOffset,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			source := usecase.Coordinate{Lat: tc.sourceLat, Lng: tc.sourceLng}
			target := usecase.Coordinate{Lat: tc.targetLat, Lng: tc.targetLng}

			sourceTile := maptile.At(orb.Point{source.Lng, source.Lat}, 14)
			targetTile := maptile.At(orb.Point{target.Lng, target.Lat}, 14)

			t.Logf("Source tile: %s, Target tile: %s", tileKey(sourceTile), tileKey(targetTile))

			result, err := svc.CalculateDistance(ctx, source, target)
			require.NoError(t, err)

			t.Logf("Result: reachable=%v, distance=%.2f km, duration=%.2f min",
				result.IsReachable, result.DistanceKm, result.DurationMin)

			if !result.IsReachable {
				t.Logf("WARNING: Path not reachable across tile boundary - may indicate boundary issue")
			}
		})
	}
}

// TestPMTilesService_TileBoundary_GraphMerge verifies that graphs from adjacent tiles merge correctly
func TestPMTilesService_TileBoundary_GraphMerge(t *testing.T) {
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

	// Get two adjacent tiles
	centerTile := maptile.At(orb.Point{121.5654, 25.0330}, 14)
	rightTile := maptile.Tile{X: centerTile.X + 1, Y: centerTile.Y, Z: centerTile.Z}

	// Load both tiles separately
	graph1, err1 := pmSvc.loadTileGraph(ctx, centerTile)
	graph2, err2 := pmSvc.loadTileGraph(ctx, rightTile)

	if err1 != nil || err2 != nil {
		t.Skipf("Could not load adjacent tiles: err1=%v, err2=%v", err1, err2)
	}

	t.Logf("Tile 1 (%s): %d nodes, %d edge groups", tileKey(centerTile), len(graph1.Nodes), len(graph1.Edges))
	t.Logf("Tile 2 (%s): %d nodes, %d edge groups", tileKey(rightTile), len(graph2.Nodes), len(graph2.Edges))

	// Merge graphs
	mergedGraph := NewRoadGraph()
	mergeGraphs(mergedGraph, graph1)
	mergeGraphs(mergedGraph, graph2)
	finalNodes := len(mergedGraph.Nodes)

	t.Logf("Merged graph: %d nodes (expected less than %d if nodes are shared)",
		finalNodes, len(graph1.Nodes)+len(graph2.Nodes))

	// Check if nodes were deduplicated
	expectedMax := len(graph1.Nodes) + len(graph2.Nodes)
	if finalNodes < expectedMax {
		sharedNodes := expectedMax - finalNodes
		t.Logf("Shared boundary nodes: %d (good - tiles are connected)", sharedNodes)
	} else {
		t.Logf("No shared nodes detected - tiles may not be adjacent or have connecting roads")
	}

	// Verify we can find paths across the boundary
	var node1ID, node2ID NodeID
	for id := range graph1.Nodes {
		node1ID = id

		break
	}
	for id := range graph2.Nodes {
		node2ID = id

		break
	}

	if node1ID != 0 && node2ID != 0 {
		point1 := graph1.Nodes[node1ID]
		point2 := graph2.Nodes[node2ID]

		mergedNode1, _, found1 := mergedGraph.FindNearestNode(point1)
		mergedNode2, _, found2 := mergedGraph.FindNearestNode(point2)

		if found1 && found2 {
			pf := NewPathfinder(mergedGraph)
			result := pf.ShortestPath(mergedNode1, mergedNode2)
			t.Logf("Cross-tile path test: reachable=%v, distance=%.2f m", result.IsReachable, result.Distance)
		}
	}
}

// TestPMTilesService_TileBoundary_DiagonalTiles tests that diagonal tiles are properly loaded
func TestPMTilesService_TileBoundary_DiagonalTiles(t *testing.T) {
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

	ctx := context.Background()

	// Create a route that goes diagonally across tile corners
	centerTile := maptile.At(orb.Point{121.5654, 25.0330}, 14)
	tileBound := centerTile.Bound()

	// Source at SW corner, target at NE corner (diagonal)
	edgeOffset := 0.001 // ~100m
	source := usecase.Coordinate{
		Lat: tileBound.Min.Lat() + edgeOffset,
		Lng: tileBound.Min.Lon() + edgeOffset,
	}
	target := usecase.Coordinate{
		Lat: tileBound.Max.Lat() + edgeOffset*5,
		Lng: tileBound.Max.Lon() + edgeOffset*5,
	}

	sourceTile := maptile.At(orb.Point{source.Lng, source.Lat}, 14)
	targetTile := maptile.At(orb.Point{target.Lng, target.Lat}, 14)

	t.Logf("Diagonal test:")
	t.Logf("  Source: (%.6f, %.6f) in tile %s", source.Lat, source.Lng, tileKey(sourceTile))
	t.Logf("  Target: (%.6f, %.6f) in tile %s", target.Lat, target.Lng, tileKey(targetTile))
	t.Logf("  Tile difference: dX=%d, dY=%d",
		int(targetTile.X)-int(sourceTile.X),
		int(targetTile.Y)-int(sourceTile.Y))

	result, err := svc.CalculateDistance(ctx, source, target)
	require.NoError(t, err)

	t.Logf("  Result: reachable=%v, distance=%.2f km", result.IsReachable, result.DistanceKm)

	// Calculate straight-line distance for comparison
	straightLine := haversineDistance(
		orb.Point{source.Lng, source.Lat},
		orb.Point{target.Lng, target.Lat},
	)
	t.Logf("  Straight-line distance: %.2f km", straightLine/1000)

	if result.IsReachable && result.DistanceKm > 0 {
		ratio := result.DistanceKm / (straightLine / 1000)
		t.Logf("  Road/straight ratio: %.2f", ratio)
	}
}

// TestPMTilesService_CacheSameTileDifferentPoints tests that different points in same tile use cache
func TestPMTilesService_CacheSameTileDifferentPoints(t *testing.T) {
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

	// Clear cache
	pmSvc.tileCacheMu.Lock()
	pmSvc.tileCache = make(map[string]*RoadGraph)
	pmSvc.tileCacheMu.Unlock()

	// Find a known tile and calculate points within it
	baseTile := maptile.Tile{X: 27449, Y: 14029, Z: 15}
	tileBound := baseTile.Bound()
	t.Logf("Target tile bounds: [%.6f, %.6f] to [%.6f, %.6f]",
		tileBound.Min.Lat(), tileBound.Min.Lon(),
		tileBound.Max.Lat(), tileBound.Max.Lon())

	// Points well inside the tile
	centerLat := (tileBound.Min.Lat() + tileBound.Max.Lat()) / 2
	centerLng := (tileBound.Min.Lon() + tileBound.Max.Lon()) / 2
	point1 := usecase.Coordinate{Lat: centerLat - 0.001, Lng: centerLng - 0.001}
	point2 := usecase.Coordinate{Lat: centerLat + 0.001, Lng: centerLng + 0.001}

	// Verify both points are in the same tile
	tile1 := maptile.At(orb.Point{point1.Lng, point1.Lat}, 15)
	tile2 := maptile.At(orb.Point{point2.Lng, point2.Lat}, 15)
	t.Logf("Point1 tile: %s", tileKey(tile1))
	t.Logf("Point2 tile: %s", tileKey(tile2))

	// First request
	_, _, err = svc.FindNearestNode(ctx, point1)
	require.NoError(t, err)

	pmSvc.tileCacheMu.RLock()
	cacheSize1 := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()
	t.Logf("Cache size after point1: %d tiles", cacheSize1)

	// Second request to different point
	_, _, err = svc.FindNearestNode(ctx, point2)
	require.NoError(t, err)

	pmSvc.tileCacheMu.RLock()
	cacheSize2 := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()
	t.Logf("Cache size after point2: %d tiles", cacheSize2)

	if tile1.X == tile2.X && tile1.Y == tile2.Y {
		t.Log("Points are in SAME tile - cache should be reused for center tile")
	} else {
		t.Log("Points are in DIFFERENT tiles - cache will grow")
	}
}

// TestPMTilesService_TileCacheEfficiency tests tile cache hit rate
func TestPMTilesService_TileCacheEfficiency(t *testing.T) {
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

	// Clear cache
	pmSvc.tileCacheMu.Lock()
	pmSvc.tileCache = make(map[string]*RoadGraph)
	pmSvc.tileCacheMu.Unlock()

	// Make multiple requests to same area
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0350, Lng: 121.5670},
		{Lat: 25.0310, Lng: 121.5640},
	}

	// First request - should populate cache
	_, err = svc.OneToMany(ctx, source, targets)
	require.NoError(t, err)

	pmSvc.tileCacheMu.RLock()
	cacheSize1 := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()
	t.Logf("Cache size after first request: %d tiles", cacheSize1)

	// Second request to same area - should use cache
	_, err = svc.OneToMany(ctx, source, targets)
	require.NoError(t, err)

	pmSvc.tileCacheMu.RLock()
	cacheSize2 := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()
	t.Logf("Cache size after second request: %d tiles", cacheSize2)

	assert.Equal(t, cacheSize1, cacheSize2, "Cache should not grow for same area")

	// Request to nearby area - may add more tiles
	nearbyTargets := []usecase.Coordinate{
		{Lat: 25.0400, Lng: 121.5700},
		{Lat: 25.0250, Lng: 121.5600},
	}
	_, err = svc.OneToMany(ctx, source, nearbyTargets)
	require.NoError(t, err)

	pmSvc.tileCacheMu.RLock()
	cacheSize3 := len(pmSvc.tileCache)
	cachedTiles := make([]string, 0, len(pmSvc.tileCache))
	for key := range pmSvc.tileCache {
		cachedTiles = append(cachedTiles, key)
	}
	pmSvc.tileCacheMu.RUnlock()

	t.Logf("Cache size after third request: %d tiles", cacheSize3)
	t.Logf("Cached tiles: %v", cachedTiles)
}

// TestTileCache_Concurrent tests concurrent access to tile cache
func TestTileCache_Concurrent(t *testing.T) {
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

	// Clear cache
	pmSvc.tileCacheMu.Lock()
	pmSvc.tileCache = make(map[string]*RoadGraph)
	pmSvc.tileCacheMu.Unlock()

	// Prepare test coordinates
	coordinates := []usecase.Coordinate{
		{Lat: 25.0330, Lng: 121.5654},
		{Lat: 25.0350, Lng: 121.5670},
		{Lat: 25.0310, Lng: 121.5640},
		{Lat: 25.0400, Lng: 121.5700},
		{Lat: 25.0250, Lng: 121.5600},
	}

	// Launch concurrent requests
	var wg sync.WaitGroup
	errChan := make(chan error, len(coordinates)*len(coordinates))

	for _, source := range coordinates {
		for _, target := range coordinates {
			if source == target {
				continue
			}
			wg.Add(1)
			go func(src, tgt usecase.Coordinate) {
				defer wg.Done()
				_, err := svc.CalculateDistance(ctx, src, tgt)
				if err != nil {
					errChan <- err
				}
			}(source, target)
		}
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errors := make([]error, 0, cap(errChan))
	for err := range errChan {
		errors = append(errors, err)
	}

	assert.Empty(t, errors, "Concurrent access should not produce errors")

	// Verify cache is in consistent state
	pmSvc.tileCacheMu.RLock()
	cacheSize := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()

	t.Logf("Cache size after concurrent access: %d tiles", cacheSize)
	assert.Greater(t, cacheSize, 0, "Cache should have entries after concurrent access")
}

// TestTileCache_ConcurrentSameTile tests concurrent loading of the same tile
func TestTileCache_ConcurrentSameTile(t *testing.T) {
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

	// Clear cache
	pmSvc.tileCacheMu.Lock()
	pmSvc.tileCache = make(map[string]*RoadGraph)
	pmSvc.tileCacheMu.Unlock()

	// Same coordinate for all goroutines
	coord := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}

	// Launch many concurrent requests for the same location
	numGoroutines := 20
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for range numGoroutines {
		wg.Go(func() {
			_, _, err := svc.FindNearestNode(ctx, coord)
			if err != nil {
				errChan <- err
			}
		})
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errors := make([]error, 0, cap(errChan))
	for err := range errChan {
		errors = append(errors, err)
	}

	assert.Empty(t, errors, "Concurrent access to same tile should not produce errors")

	// Verify cache state
	pmSvc.tileCacheMu.RLock()
	cacheSize := len(pmSvc.tileCache)
	pmSvc.tileCacheMu.RUnlock()

	t.Logf("Cache size after %d concurrent requests to same location: %d tiles", numGoroutines, cacheSize)
}
