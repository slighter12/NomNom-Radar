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
const (
	// fallback defaults to keep routing functional when config is missing/invalid
	defaultSnapDistanceKm = 1.0
	defaultSpeedKmh       = 30.0
)

type routingService struct {
	// Configuration
	maxSnapDistanceKm float64 // Maximum distance for GPS coordinate snapping
	defaultSpeedKmh   float64 // Default speed for duration estimation

	// Engine state
	isReady bool
	mu      sync.RWMutex

	// CH graph and query pool integrations will be added once dependencies are available
	// chGraph    *routing.CHGraph
	// queryPool  *routing.QueryPool
	// spatialIdx *routing.SpatialIndex
}

// NewRoutingService creates a new routing service instance
func NewRoutingService(config *config.RoutingConfig) usecase.RoutingUsecase {
	snapDistance := config.MaxSnapDistanceKm
	if snapDistance <= 0 {
		snapDistance = defaultSnapDistanceKm
	}

	speedKmh := config.DefaultSpeedKmh
	if speedKmh <= 0 {
		speedKmh = defaultSpeedKmh
	}

	return &routingService{
		maxSnapDistanceKm: snapDistance,
		defaultSpeedKmh:   speedKmh,
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
		// CH-based OneToMany routing will be wired when the engine is available; fallback for now
		return s.oneToManyHaversine(ctx, source, targets)
	}

	// Use Haversine fallback
	return s.oneToManyHaversine(ctx, source, targets)
}

// oneToManyHaversine implements OneToMany using Haversine distance (straight-line)
func (s *routingService) oneToManyHaversine(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) (*usecase.OneToManyResult, error) {
	results := make([]usecase.RouteResult, len(targets))

	targetCh := make(chan int, len(targets))
	resultCh := make(chan routeResultWithIndex, len(targets))

	workerCount := s.workerCount(len(targets))
	workerGroup := s.spawnRouteWorkers(ctx, workerCount, targetCh, resultCh, source, targets)

	go s.dispatchRouteWork(ctx, targetCh, len(targets))
	collectRouteResults(resultCh, results, workerGroup)

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "routing calculation canceled")
	}

	return &usecase.OneToManyResult{
		Source:  source,
		Targets: targets,
		Results: results,
	}, nil
}

// FindNearestNode finds the nearest road network node to a given GPS coordinate
func (s *routingService) FindNearestNode(ctx context.Context, coord usecase.Coordinate) (*usecase.NodeInfo, bool, error) {
	// For now, return a mock node within maxSnapDistance
	// Spatial index lookup will be added when the routing engine is integrated

	if s.maxSnapDistanceKm <= 0 {
		return nil, false, errors.New("invalid max snap distance configuration")
	}

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
	// Handle invalid coordinates
	if !s.isValidCoordinate(source) || !s.isValidCoordinate(target) {
		return usecase.RouteResult{
			Source:      source,
			Target:      target,
			IsReachable: false,
		}
	}

	distanceKm := s.haversineDistance(source.Lat, source.Lng, target.Lat, target.Lng)
	durationMin := (distanceKm / s.defaultSpeedKmh) * 60 // Convert hours to minutes

	return usecase.RouteResult{
		Source:      source,
		Target:      target,
		DistanceKm:  distanceKm,
		DurationMin: durationMin,
		IsReachable: true, // Haversine assumes all valid points are reachable
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

// isValidCoordinate checks if a coordinate is within valid geographic bounds (Earth)
func (s *routingService) isValidCoordinate(coord usecase.Coordinate) bool {
	// Reject NaN or infinities early
	if math.IsNaN(coord.Lat) || math.IsNaN(coord.Lng) ||
		math.IsInf(coord.Lat, 0) || math.IsInf(coord.Lng, 0) {
		return false
	}

	// Basic Earth bounds; allows offshore islands and nearby areas
	return coord.Lat >= -90 && coord.Lat <= 90 &&
		coord.Lng >= -180 && coord.Lng <= 180
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

func (s *routingService) workerCount(targetCount int) int {
	const numWorkers = 10
	if targetCount < numWorkers {
		return targetCount
	}

	return numWorkers
}

type routeResultWithIndex struct {
	index  int
	result usecase.RouteResult
}

func (s *routingService) spawnRouteWorkers(
	ctx context.Context,
	workerCount int,
	targetCh <-chan int,
	resultCh chan<- routeResultWithIndex,
	source usecase.Coordinate,
	targets []usecase.Coordinate,
) *sync.WaitGroup {
	var workerGroup sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			for idx := range targetCh {
				if ctx.Err() != nil {
					return
				}

				result := s.calculateHaversineDistance(source, targets[idx])
				resultCh <- routeResultWithIndex{index: idx, result: result}
			}
		}()
	}

	return &workerGroup
}

func (s *routingService) dispatchRouteWork(ctx context.Context, targetCh chan<- int, targetCount int) {
	defer close(targetCh)

	for i := 0; i < targetCount; i++ {
		if ctx.Err() != nil {
			return
		}

		targetCh <- i
	}
}

func collectRouteResults(resultCh chan routeResultWithIndex, results []usecase.RouteResult, workerGroup *sync.WaitGroup) {
	go func() {
		workerGroup.Wait()
		close(resultCh)
	}()

	for res := range resultCh {
		results[res.index] = res.result
	}
}
