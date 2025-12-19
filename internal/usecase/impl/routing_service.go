package impl

import (
	"context"
	"math"
	"sync"
	"time"

	"radar/config"
	"radar/internal/usecase"

	"github.com/pkg/errors"
)

// routingService implements the RoutingUsecase interface
type routingService struct {
	// Configuration
	maxSnapDistanceKm float64 // Maximum distance for GPS coordinate snapping
	defaultSpeedKmh   float64 // Default speed for duration estimation

	// Engine state
	isReady bool
	mu      sync.RWMutex

	// TODO: Add CH graph and query pool when dependencies are available
	// chGraph    *routing.CHGraph
	// queryPool  *routing.QueryPool
	// spatialIdx *routing.SpatialIndex
}

// NewRoutingService creates a new routing service instance
func NewRoutingService(config *config.RoutingConfig) usecase.RoutingUsecase {
	return &routingService{
		maxSnapDistanceKm: config.MaxSnapDistanceKm,
		defaultSpeedKmh:   config.DefaultSpeedKmh,
		isReady:           false, // Will be set to true when CH data is loaded
	}
}

// OneToMany calculates routes from one source coordinate to multiple target coordinates
func (s *routingService) OneToMany(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) (*usecase.OneToManyResult, error) {
	startTime := time.Now()

	if len(targets) == 0 {
		return &usecase.OneToManyResult{
			Source:   source,
			Targets:  targets,
			Results:  []usecase.RouteResult{},
			Duration: time.Since(startTime),
		}, nil
	}

	// Check if CH engine is ready, otherwise use Haversine fallback
	if s.IsReady() {
		// TODO: Implement CH-based OneToMany routing
		// For now, fall back to Haversine
		return s.oneToManyHaversine(ctx, source, targets)
	}

	// Use Haversine fallback
	return s.oneToManyHaversine(ctx, source, targets)
}

// oneToManyHaversine implements OneToMany using Haversine distance (straight-line)
func (s *routingService) oneToManyHaversine(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) (*usecase.OneToManyResult, error) {
	startTime := time.Now()
	results := make([]usecase.RouteResult, len(targets))

	// Process targets concurrently using worker pool pattern
	const numWorkers = 10
	targetCh := make(chan int, len(targets))
	resultCh := make(chan struct {
		index  int
		result usecase.RouteResult
	}, len(targets))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers && i < len(targets); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range targetCh {
				select {
				case <-ctx.Done():
					return
				default:
					result := s.calculateHaversineDistance(source, targets[idx])
					resultCh <- struct {
						index  int
						result usecase.RouteResult
					}{idx, result}
				}
			}
		}()
	}

	// Send work to workers
	go func() {
		defer close(targetCh)
		for i := range targets {
			select {
			case <-ctx.Done():
				return
			case targetCh <- i:
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	resultsReceived := 0
	for res := range resultCh {
		results[res.index] = res.result
		resultsReceived++
		if resultsReceived == len(targets) {
			break
		}
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "routing calculation cancelled")
	}

	return &usecase.OneToManyResult{
		Source:   source,
		Targets:  targets,
		Results:  results,
		Duration: time.Since(startTime),
	}, nil
}

// FindNearestNode finds the nearest road network node to a given GPS coordinate
func (s *routingService) FindNearestNode(ctx context.Context, coord usecase.Coordinate) (*usecase.NodeInfo, bool, error) {
	// For now, return a mock node within maxSnapDistance
	// TODO: Implement proper spatial index lookup

	// Check if coordinate is within reasonable bounds (Taiwan)
	if !s.isValidCoordinate(coord) {
		return nil, false, errors.New("coordinate is outside valid bounds")
	}

	// Mock implementation: return the input coordinate as a node
	// In real implementation, this would query the spatial index
	nodeInfo := &usecase.NodeInfo{
		ID:       usecase.NodeID(1), // Mock ID
		Location: coord,
	}

	// For Haversine fallback, distance is always 0 (exact match)
	distance := 0.0
	withinRange := distance <= s.maxSnapDistanceKm

	return nodeInfo, withinRange, nil
}

// CalculateDistance calculates the road network distance between two coordinates
func (s *routingService) CalculateDistance(ctx context.Context, source, target usecase.Coordinate) (*usecase.RouteResult, error) {
	result := s.calculateHaversineDistance(source, target)
	return &result, nil
}

// calculateHaversineDistance calculates straight-line distance and estimated duration
func (s *routingService) calculateHaversineDistance(source, target usecase.Coordinate) usecase.RouteResult {
	distanceKm := s.haversineDistance(source.Lat, source.Lng, target.Lat, target.Lng)
	durationMin := (distanceKm / s.defaultSpeedKmh) * 60 // Convert hours to minutes

	return usecase.RouteResult{
		Source:      source,
		Target:      target,
		DistanceKm:  distanceKm,
		DurationMin: durationMin,
		IsReachable: true, // Haversine assumes all points are reachable
	}
}

// haversineDistance calculates the great circle distance between two points in kilometers
func (s *routingService) haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lng1Rad := lng1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lng2Rad := lng2 * math.Pi / 180

	deltaLat := lat2Rad - lat1Rad
	deltaLng := lng2Rad - lng1Rad

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

// isValidCoordinate checks if a coordinate is within reasonable bounds for Taiwan
func (s *routingService) isValidCoordinate(coord usecase.Coordinate) bool {
	// Taiwan approximate bounds
	const (
		minLat = 20.0
		maxLat = 27.0
		minLng = 118.0
		maxLng = 125.0
	)

	return coord.Lat >= minLat && coord.Lat <= maxLat &&
		coord.Lng >= minLng && coord.Lng <= maxLng
}

// IsReady returns whether the routing engine is loaded and ready for queries
func (s *routingService) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isReady
}

// setReady sets the ready state of the routing engine (internal method)
func (s *routingService) setReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isReady = ready
}
