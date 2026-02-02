package pmtiles

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"radar/config"
	"radar/internal/usecase"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
)

func BenchmarkTileKey(b *testing.B) {
	tile := maptile.Tile{X: 13823, Y: 7082, Z: 14}

	for b.Loop() {
		tileKey(tile)
	}
}

func BenchmarkGetTilesForBounds(b *testing.B) {
	for b.Loop() {
		getTilesForBounds(25.00, 25.10, 121.50, 121.60, 14)
	}
}

func BenchmarkMergeGraphs(b *testing.B) {
	sourceSegment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.01},
			{121.52, 25.02},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	for b.Loop() {
		target := NewRoadGraph()
		source := NewRoadGraph()
		source.AddSegment(sourceSegment)
		mergeGraphs(target, source)
	}
}

func BenchmarkHaversineFallback_OneToMany(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := newHaversineFallbackService(logger)

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := make([]usecase.Coordinate, 100)
	for i := range targets {
		targets[i] = usecase.Coordinate{
			Lat: 25.0 + float64(i%10)*0.01,
			Lng: 121.5 + float64(i/10)*0.01,
		}
	}

	ctx := context.Background()

	for b.Loop() {
		_, _ = svc.OneToMany(ctx, source, targets)
	}
}

func BenchmarkPMTilesService_OneToMany(b *testing.B) {
	path := testPMTilesPath()
	if path == "" {
		b.Skip("walking.pmtiles not found")
	}

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
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := make([]usecase.Coordinate, 10)
	for i := range targets {
		targets[i] = usecase.Coordinate{
			Lat: 25.03 + float64(i)*0.002,
			Lng: 121.56 + float64(i)*0.002,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		_, _ = svc.OneToMany(ctx, source, targets)
	}
}

func BenchmarkPMTilesService_CalculateDistance(b *testing.B) {
	path := testPMTilesPath()
	if path == "" {
		b.Skip("walking.pmtiles not found")
	}

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
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	target := usecase.Coordinate{Lat: 25.0350, Lng: 121.5670}

	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		_, _ = svc.CalculateDistance(ctx, source, target)
	}
}

func BenchmarkPMTilesService_FindNearestNode(b *testing.B) {
	path := testPMTilesPath()
	if path == "" {
		b.Skip("walking.pmtiles not found")
	}

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
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	coord := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		_, _, _ = svc.FindNearestNode(ctx, coord)
	}
}

func BenchmarkParseSourcePath(b *testing.B) {
	sources := []string{
		"file:///path/to/walking.pmtiles",
		"/path/to/walking.pmtiles",
		"https://example.com/tiles/walking.pmtiles",
	}

	for b.Loop() {
		for _, source := range sources {
			parseSourcePath(source)
		}
	}
}
