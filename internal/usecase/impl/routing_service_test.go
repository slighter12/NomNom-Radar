package impl

import (
	"context"
	"fmt"
	"math"
	"testing"

	"radar/config"
	"radar/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRoutingService(t *testing.T) {
	cfg := &config.RoutingConfig{
		MaxSnapDistanceKm: 1.5,
		DefaultSpeedKmh:   35.0,
		DataPath:          "./data/routing",
		Enabled:           false, // Disabled to avoid loading non-existent data
	}

	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	assert.NotNil(t, service)
	assert.False(t, service.IsReady()) // Should start as not ready
}

func TestNewRoutingService_ZeroConfig(t *testing.T) {
	// Test with zero values - should use defaults without panic
	cfg := &config.RoutingConfig{}

	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})
	impl := service.(*routingService)

	assert.NotNil(t, service)
	assert.Equal(t, defaultSnapDistanceKm, impl.maxSnapDistanceKm)
	assert.Equal(t, defaultSpeedKmh, impl.defaultSpeedKmh)
}

func TestRoutingService_IsReady(t *testing.T) {
	cfg := &config.RoutingConfig{}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil}).(*routingService)

	// Should start not ready (no engine loaded)
	assert.False(t, service.IsReady())
}

func TestRoutingService_CalculateDistance(t *testing.T) {
	cfg := &config.RoutingConfig{
		DefaultSpeedKmh: 30.0, // 30 km/h
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()

	// Test with known coordinates (approximately 1km apart)
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654} // Taipei 101
	target := usecase.Coordinate{Lat: 25.0425, Lng: 121.5649} // Nearby location

	result, err := service.CalculateDistance(ctx, source, target)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result structure
	assert.Equal(t, source, result.Source)
	assert.Equal(t, target, result.Target)
	assert.True(t, result.DistanceKm > 0)
	assert.True(t, result.DurationMin > 0)
	assert.True(t, result.IsReachable)

	// Distance should be reasonable (less than 2km for these coordinates)
	assert.True(t, result.DistanceKm < 2.0, "Distance should be less than 2km")
}

func TestRoutingService_FindNearestNode(t *testing.T) {
	cfg := &config.RoutingConfig{
		MaxSnapDistanceKm: 1.0,
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()

	// Test with valid Taiwan coordinates
	coord := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}

	nodeInfo, withinRange, err := service.FindNearestNode(ctx, coord)
	require.NoError(t, err)
	require.NotNil(t, nodeInfo)
	assert.True(t, withinRange) // Should be within range for valid coordinates

	// Node should have the same coordinates (mock implementation)
	assert.Equal(t, coord.Lat, nodeInfo.Location.Lat)
	assert.Equal(t, coord.Lng, nodeInfo.Location.Lng)
}

func TestRoutingService_FindNearestNode_InvalidCoordinates(t *testing.T) {
	cfg := &config.RoutingConfig{
		MaxSnapDistanceKm: 1.0,
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()

	invalidCoords := []usecase.Coordinate{
		{Lat: 95.0, Lng: 0.0},  // beyond north pole
		{Lat: -95.0, Lng: 0.0}, // beyond south pole
		{Lat: 0.0, Lng: 195.0}, // beyond valid longitude
		{Lat: 0.0, Lng: -195.0},
		{Lat: math.NaN(), Lng: 0.0},
	}

	for _, coord := range invalidCoords {
		_, _, err := service.FindNearestNode(ctx, coord)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside valid bounds")
	}
}

func TestRoutingService_OneToMany_EmptyTargets(t *testing.T) {
	cfg := &config.RoutingConfig{}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{}

	result, err := service.OneToMany(ctx, source, targets)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, source, result.Source)
	assert.Empty(t, result.Results)
	assert.True(t, result.Duration >= 0)
}

func TestRoutingService_OneToMany_SingleTarget(t *testing.T) {
	cfg := &config.RoutingConfig{
		DefaultSpeedKmh: 30.0,
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0425, Lng: 121.5649},
	}

	result, err := service.OneToMany(ctx, source, targets)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, source, result.Source)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Duration > 0)

	routeResult := result.Results[0]
	assert.Equal(t, source, routeResult.Source)
	assert.Equal(t, targets[0], routeResult.Target)
	assert.True(t, routeResult.DistanceKm > 0)
	assert.True(t, routeResult.IsReachable)
}

func TestRoutingService_OneToMany_MultipleTargets(t *testing.T) {
	cfg := &config.RoutingConfig{
		DefaultSpeedKmh: 30.0,
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0425, Lng: 121.5649}, // ~1km
		{Lat: 25.0520, Lng: 121.5640}, // ~2km
		{Lat: 25.0615, Lng: 121.5630}, // ~3km
	}

	result, err := service.OneToMany(ctx, source, targets)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, source, result.Source)
	assert.Len(t, result.Results, 3)
	assert.True(t, result.Duration > 0)

	// Verify all results
	for i, routeResult := range result.Results {
		assert.Equal(t, source, routeResult.Source)
		assert.Equal(t, targets[i], routeResult.Target)
		assert.True(t, routeResult.DistanceKm > 0)
		assert.True(t, routeResult.IsReachable)

		// Distance should increase with each target
		if i > 0 {
			assert.True(t, routeResult.DistanceKm > result.Results[i-1].DistanceKm)
		}
	}
}

func TestRoutingService_OneToMany_ContextCancellation(t *testing.T) {
	cfg := &config.RoutingConfig{}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	targets := []usecase.Coordinate{
		{Lat: 25.0425, Lng: 121.5649},
	}

	_, err := service.OneToMany(ctx, source, targets)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}

func TestRoutingService_HaversineDistance(t *testing.T) {
	cfg := &config.RoutingConfig{}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil}).(*routingService)

	// Test known distance: Taipei 101 to nearby location (~1km)
	lat1, lng1 := 25.0330, 121.5654
	lat2, lng2 := 25.0425, 121.5649

	distance := service.haversineDistance(lat1, lng1, lat2, lng2)

	// Should be approximately 1km (allowing some margin for floating point precision)
	assert.True(t, distance > 0.8, "Distance should be greater than 0.8km")
	assert.True(t, distance < 1.5, "Distance should be less than 1.5km")
}

func TestRoutingService_IsValidCoordinate(t *testing.T) {
	cfg := &config.RoutingConfig{}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil}).(*routingService)

	// Valid coordinates on Earth
	validCoords := []usecase.Coordinate{
		{Lat: 25.0330, Lng: 121.5654},  // Taiwan
		{Lat: -33.8688, Lng: 151.2093}, // Sydney
		{Lat: 0, Lng: 0},               // Gulf of Guinea
	}
	for _, coord := range validCoords {
		assert.True(t, service.isValidCoordinate(coord))
	}

	// Invalid coordinates (outside Taiwan bounds)
	invalidCoords := []usecase.Coordinate{
		{Lat: 91.0, Lng: 0.0},   // Too far north
		{Lat: -91.0, Lng: 0.0},  // Too far south
		{Lat: 0.0, Lng: 181.0},  // Too far east
		{Lat: 0.0, Lng: -181.0}, // Too far west
		{Lat: math.NaN(), Lng: 0},
	}

	for _, coord := range invalidCoords {
		assert.False(t, service.isValidCoordinate(coord), "Coordinate should be invalid: %+v", coord)
	}
}

func TestRoutingService_CalculateDistance_InvalidCoordinates(t *testing.T) {
	cfg := &config.RoutingConfig{
		DefaultSpeedKmh: 30.0,
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()

	// Test with NaN coordinate
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}
	invalidTarget := usecase.Coordinate{Lat: math.NaN(), Lng: 121.5649}

	result, err := service.CalculateDistance(ctx, source, invalidTarget)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsReachable, "Invalid coordinates should be marked as unreachable")

	// Test with invalid source
	invalidSource := usecase.Coordinate{Lat: 95.0, Lng: 0.0}
	validTarget := usecase.Coordinate{Lat: 25.0425, Lng: 121.5649}

	result, err = service.CalculateDistance(ctx, invalidSource, validTarget)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsReachable, "Invalid coordinates should be marked as unreachable")
}

func BenchmarkRoutingService_OneToMany(b *testing.B) {
	cfg := &config.RoutingConfig{
		DefaultSpeedKmh: 30.0,
	}
	service := NewRoutingService(RoutingServiceParams{Config: cfg, Logger: nil})

	ctx := context.Background()
	source := usecase.Coordinate{Lat: 25.0330, Lng: 121.5654}

	// Benchmark with different numbers of targets
	targetCounts := []int{1, 10, 50, 100}

	for _, count := range targetCounts {
		b.Run(fmt.Sprintf("targets_%d", count), func(b *testing.B) {
			// Generate targets in a grid pattern
			targets := make([]usecase.Coordinate, count)
			for i := range count {
				lat := source.Lat + float64(i%10)*0.001
				lng := source.Lng + float64(i/10)*0.001
				targets[i] = usecase.Coordinate{Lat: lat, Lng: lng}
			}

			b.ResetTimer()
			for b.Loop() {
				_, err := service.OneToMany(ctx, source, targets)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
