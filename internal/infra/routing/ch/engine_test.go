package ch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"radar/internal/infra/routing/loader"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDataDir(t *testing.T) string {
	tmpDir := t.TempDir()

	// Create vertices.csv
	verticesCSV := `id,lat,lng,order_pos,importance
0,25.0330,121.5654,0,1
1,25.0478,121.5170,1,2
2,25.0400,121.5400,2,3
3,23.5711,119.5793,3,4
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vertices.csv"), []byte(verticesCSV), 0644))

	// Create edges.csv - connect the Taiwan vertices, but NOT Penghu
	edgesCSV := `from,to,weight
0,1,2000
1,2,1500
0,2,2500
2,0,2500
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "edges.csv"), []byte(edgesCSV), 0644))

	// Create shortcuts.csv (empty for simplicity)
	shortcutsCSV := `from,to,weight,via_node
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "shortcuts.csv"), []byte(shortcutsCSV), 0644))

	// Create metadata.json
	metadata := loader.RoutingMetadata{
		Version: "1.0",
		Source: loader.SourceInfo{
			Region: "taiwan",
		},
		Processing: loader.ProcessingInfo{
			GeneratedAt: time.Now(),
			Profile:     "scooter",
		},
		Output: loader.OutputInfo{
			VerticesCount:  4,
			EdgesCount:     4,
			ShortcutsCount: 0,
		},
	}
	metaBytes, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "metadata.json"), metaBytes, 0644))

	return tmpDir
}

func TestEngine_LoadData(t *testing.T) {
	dataDir := setupTestDataDir(t)

	engine := NewEngine(DefaultEngineConfig(), nil)
	err := engine.LoadData(dataDir)
	require.NoError(t, err)

	assert.True(t, engine.IsReady())
	assert.NotNil(t, engine.GetMetadata())
	assert.Equal(t, "taiwan", engine.GetMetadata().Source.Region)
}

func TestEngine_LoadData_MissingDirectory(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil)
	err := engine.LoadData("/nonexistent/path")
	assert.Error(t, err)
	assert.False(t, engine.IsReady())
}

func TestEngine_FindNearestNode(t *testing.T) {
	dataDir := setupTestDataDir(t)

	engine := NewEngine(DefaultEngineConfig(), nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	// Find nearest to Taipei Station (vertex 0)
	result, err := engine.FindNearestNode(ctx, Coordinate{Lat: 25.0335, Lng: 121.5660})
	require.NoError(t, err)
	assert.True(t, result.IsValid)
	assert.Equal(t, 0, result.NodeID)
	assert.InDelta(t, 25.0330, result.NodeLat, 0.0001)
}

func TestEngine_FindNearestNode_SnapDistanceExceeded(t *testing.T) {
	dataDir := setupTestDataDir(t)

	config := DefaultEngineConfig()
	config.MaxSnapDistanceMeters = 100 // Very strict snap distance

	engine := NewEngine(config, nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	// Query from middle of Taiwan Strait (far from any road)
	result, err := engine.FindNearestNode(ctx, Coordinate{Lat: 24.5, Lng: 119.5})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSnapDistanceExceeded)
	assert.False(t, result.IsValid)
}

func TestEngine_ShortestPath(t *testing.T) {
	dataDir := setupTestDataDir(t)

	engine := NewEngine(DefaultEngineConfig(), nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	// Route between two connected Taipei vertices
	from := Coordinate{Lat: 25.0330, Lng: 121.5654} // Near vertex 0
	to := Coordinate{Lat: 25.0478, Lng: 121.5170}   // Near vertex 1

	result, err := engine.ShortestPath(ctx, from, to)
	require.NoError(t, err)
	assert.True(t, result.IsReachable)
	assert.Greater(t, result.Distance, 0.0)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestEngine_ShortestPath_SamePoint(t *testing.T) {
	dataDir := setupTestDataDir(t)

	engine := NewEngine(DefaultEngineConfig(), nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	// Same point should return 0 distance
	point := Coordinate{Lat: 25.0330, Lng: 121.5654}
	result, err := engine.ShortestPath(ctx, point, point)
	require.NoError(t, err)
	assert.True(t, result.IsReachable)
	assert.Equal(t, 0.0, result.Distance)
}

func TestEngine_ShortestPath_IslandUnreachable(t *testing.T) {
	dataDir := setupTestDataDir(t)

	engine := NewEngine(DefaultEngineConfig(), nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	// Taipei (vertex 0) to Penghu (vertex 3) - should be unreachable
	// because there's no edge connecting Penghu
	taipei := Coordinate{Lat: 25.0330, Lng: 121.5654}
	penghu := Coordinate{Lat: 23.5711, Lng: 119.5793}

	result, err := engine.ShortestPath(ctx, taipei, penghu)
	require.NoError(t, err)
	assert.False(t, result.IsReachable, "Penghu should be unreachable from Taiwan main island via road network")
}

func TestEngine_OneToMany(t *testing.T) {
	dataDir := setupTestDataDir(t)

	config := DefaultEngineConfig()
	config.OneToManyWorkers = 2 // Limit workers for testing

	engine := NewEngine(config, nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	source := Coordinate{Lat: 25.0330, Lng: 121.5654} // Near vertex 0
	targets := []Coordinate{
		{Lat: 25.0478, Lng: 121.5170}, // Near vertex 1 (reachable)
		{Lat: 25.0400, Lng: 121.5400}, // Near vertex 2 (reachable)
		{Lat: 23.5711, Lng: 119.5793}, // Penghu (unreachable)
	}

	results, err := engine.OneToMany(ctx, source, targets)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Check that Taipei targets are reachable
	assert.True(t, results[0].IsReachable, "Target 0 should be reachable")
	assert.True(t, results[1].IsReachable, "Target 1 should be reachable")

	// Penghu should be unreachable (no road connection)
	assert.False(t, results[2].IsReachable, "Target 2 (Penghu) should be unreachable")
}

func TestEngine_OneToMany_Empty(t *testing.T) {
	dataDir := setupTestDataDir(t)

	engine := NewEngine(DefaultEngineConfig(), nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	source := Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []Coordinate{}

	results, err := engine.OneToMany(ctx, source, targets)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestEngine_NotReady(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil)
	// Don't load any data

	assert.False(t, engine.IsReady())

	ctx := context.Background()

	_, err := engine.FindNearestNode(ctx, Coordinate{Lat: 25.0, Lng: 121.0})
	assert.ErrorIs(t, err, ErrEngineNotReady)

	_, err = engine.ShortestPath(ctx, Coordinate{Lat: 25.0, Lng: 121.0}, Coordinate{Lat: 25.1, Lng: 121.1})
	assert.ErrorIs(t, err, ErrEngineNotReady)

	_, err = engine.OneToMany(ctx, Coordinate{Lat: 25.0, Lng: 121.0}, []Coordinate{{Lat: 25.1, Lng: 121.1}})
	assert.ErrorIs(t, err, ErrEngineNotReady)
}

func TestHaversineMeters(t *testing.T) {
	// Taipei Station to Taipei Main Station
	// Approximately 5.6 km apart
	dist := haversineMeters(25.0330, 121.5654, 25.0478, 121.5170)
	assert.InDelta(t, 5500, dist, 500) // Allow 500m tolerance

	// Same point should be 0
	dist = haversineMeters(25.0, 121.0, 25.0, 121.0)
	assert.Equal(t, 0.0, dist)

	// Taipei to Penghu (roughly 250 km)
	dist = haversineMeters(25.0330, 121.5654, 23.5711, 119.5793)
	assert.InDelta(t, 255000, dist, 10000) // Allow 10km tolerance
}

func TestEngine_OneToMany_Concurrent(t *testing.T) {
	dataDir := setupTestDataDir(t)

	config := DefaultEngineConfig()
	config.OneToManyWorkers = 4

	engine := NewEngine(config, nil)
	require.NoError(t, engine.LoadData(dataDir))

	ctx := context.Background()

	// Run multiple concurrent OneToMany calls to verify thread safety
	const numGoroutines = 10
	const numTargets = 5

	source := Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := make([]Coordinate, numTargets)
	for i := range targets {
		targets[i] = Coordinate{
			Lat: 25.0 + float64(i)*0.01,
			Lng: 121.5 + float64(i)*0.01,
		}
	}

	errCh := make(chan error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			results, err := engine.OneToMany(ctx, source, targets)
			if err != nil {
				errCh <- err

				return
			}
			if len(results) != numTargets {
				errCh <- errors.Errorf("unexpected result count: got %d, want %d", len(results), numTargets)

				return
			}
			errCh <- nil
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}
}

func BenchmarkEngine_OneToMany(b *testing.B) {
	// Create a larger test dataset
	tmpDir := b.TempDir()

	// Create grid of vertices
	var verticesLines []string
	verticesLines = append(verticesLines, "id,lat,lng,order_pos,importance")
	var edgesLines []string
	edgesLines = append(edgesLines, "from,to,weight")

	id := 0
	for lat := 22.0; lat <= 25.5; lat += 0.05 {
		for lng := 120.0; lng <= 122.0; lng += 0.05 {
			verticesLines = append(verticesLines, formatVertex(id, lat, lng))
			// Connect to neighbors
			if id > 0 {
				edgesLines = append(edgesLines, formatEdge(id-1, id, 1000))
			}
			id++
		}
	}

	writeLines(b, filepath.Join(tmpDir, "vertices.csv"), verticesLines)
	writeLines(b, filepath.Join(tmpDir, "edges.csv"), edgesLines)
	os.WriteFile(filepath.Join(tmpDir, "shortcuts.csv"), []byte("from,to,weight,via_node\n"), 0644)

	config := DefaultEngineConfig()
	config.OneToManyWorkers = 10

	engine := NewEngine(config, nil)
	if err := engine.LoadData(tmpDir); err != nil {
		b.Fatalf("Failed to load data: %v", err)
	}

	source := Coordinate{Lat: 24.0, Lng: 121.0}
	targets := make([]Coordinate, 100)
	for i := range targets {
		targets[i] = Coordinate{
			Lat: 23.5 + float64(i%10)*0.1,
			Lng: 120.5 + float64(i/10)*0.1,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.OneToMany(ctx, source, targets)
	}
}

func formatVertex(id int, lat, lng float64) string {
	return strconv.Itoa(id) + "," +
		floatToString(lat) + "," + floatToString(lng) + ",0,1"
}

func formatEdge(from, to int, weight int) string {
	return strconv.Itoa(from) + "," +
		strconv.Itoa(to) + "," + strconv.Itoa(weight)
}

func floatToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func writeLines(b *testing.B, path string, lines []string) {
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		b.Fatalf("Failed to write %s: %v", path, err)
	}
}

func BenchmarkEngine_ShortestPath(b *testing.B) {
	// Reuse the larger test dataset setup from BenchmarkEngine_OneToMany
	tmpDir := b.TempDir()

	var verticesLines []string
	verticesLines = append(verticesLines, "id,lat,lng,order_pos,importance")
	var edgesLines []string
	edgesLines = append(edgesLines, "from,to,weight")

	id := 0
	for lat := 22.0; lat <= 25.5; lat += 0.05 {
		for lng := 120.0; lng <= 122.0; lng += 0.05 {
			verticesLines = append(verticesLines, formatVertex(id, lat, lng))
			if id > 0 {
				edgesLines = append(edgesLines, formatEdge(id-1, id, 1000))
			}
			id++
		}
	}

	writeLines(b, filepath.Join(tmpDir, "vertices.csv"), verticesLines)
	writeLines(b, filepath.Join(tmpDir, "edges.csv"), edgesLines)
	_ = os.WriteFile(filepath.Join(tmpDir, "shortcuts.csv"), []byte("from,to,weight,via_node\n"), 0644)

	engine := NewEngine(DefaultEngineConfig(), nil)
	if err := engine.LoadData(tmpDir); err != nil {
		b.Fatalf("Failed to load data: %v", err)
	}

	from := Coordinate{Lat: 23.0, Lng: 121.0}
	to := Coordinate{Lat: 24.5, Lng: 121.5}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ShortestPath(ctx, from, to)
	}
}

// BenchmarkEngine_OneToMany_WithPreFilter benchmarks OneToMany with tight pre-filter radius
// to measure the effectiveness of Haversine pre-filtering
func BenchmarkEngine_OneToMany_WithPreFilter(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test data
	var verticesLines, edgesLines []string
	verticesLines = append(verticesLines, "id,lat,lng,order_pos,importance")
	edgesLines = append(edgesLines, "from,to,weight")

	id := 0
	for lat := 22.0; lat <= 25.5; lat += 0.1 {
		for lng := 120.0; lng <= 122.0; lng += 0.1 {
			verticesLines = append(verticesLines, formatVertex(id, lat, lng))
			if id > 0 {
				edgesLines = append(edgesLines, formatEdge(id-1, id, 1000))
			}
			id++
		}
	}

	writeLines(b, filepath.Join(tmpDir, "vertices.csv"), verticesLines)
	writeLines(b, filepath.Join(tmpDir, "edges.csv"), edgesLines)
	_ = os.WriteFile(filepath.Join(tmpDir, "shortcuts.csv"), []byte("from,to,weight,via_node\n"), 0644)

	// Tight pre-filter radius (5km) - should filter out most targets
	configTight := DefaultEngineConfig()
	configTight.MaxQueryRadiusMeters = 5000
	configTight.PreFilterRadiusMultiplier = 1.3 // 6.5km effective

	engine := NewEngine(configTight, nil)
	if err := engine.LoadData(tmpDir); err != nil {
		b.Fatalf("Failed to load data: %v", err)
	}

	source := Coordinate{Lat: 24.0, Lng: 121.0}
	// Create targets spread across Taiwan - most will be filtered
	targets := make([]Coordinate, 50)
	for i := range targets {
		targets[i] = Coordinate{
			Lat: 22.5 + float64(i%10)*0.3,
			Lng: 120.0 + float64(i/10)*0.4,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.OneToMany(ctx, source, targets)
	}
}
