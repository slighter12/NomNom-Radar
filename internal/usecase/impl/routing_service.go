package impl

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"radar/config"
	"radar/internal/infra/routing/ch"
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

	// CH Engine (infrastructure layer)
	engine *ch.Engine
	logger *slog.Logger

	// Engine state
	mu sync.RWMutex
}

// NewRoutingService creates a new routing service instance
func NewRoutingService(cfg *config.RoutingConfig, logger *slog.Logger) usecase.RoutingUsecase {
	if logger == nil {
		logger = slog.Default()
	}

	snapDistance := cfg.MaxSnapDistanceKm
	if snapDistance <= 0 {
		snapDistance = defaultSnapDistanceKm
	}

	speedKmh := cfg.DefaultSpeedKmh
	if speedKmh <= 0 {
		speedKmh = defaultSpeedKmh
	}

	svc := &routingService{
		maxSnapDistanceKm: snapDistance,
		defaultSpeedKmh:   speedKmh,
		logger:            logger,
	}

	// Initialize CH engine if enabled and data path is configured
	if cfg.Enabled && cfg.DataPath != "" {
		engineConfig := buildEngineConfig(cfg, snapDistance, speedKmh)
		svc.engine = ch.NewEngine(engineConfig, logger)

		// Try to load routing data
		if err := svc.engine.LoadData(cfg.DataPath); err != nil {
			logger.Warn("Failed to load CH routing data, using Haversine fallback",
				"dataPath", cfg.DataPath,
				"error", err,
			)
			svc.engine = nil
		} else {
			logger.Info("CH routing engine loaded successfully",
				"dataPath", cfg.DataPath,
			)
		}
	} else {
		logger.Info("Routing engine disabled or no data path configured, using Haversine fallback")
	}

	return svc
}

// buildEngineConfig creates a CH engine config with sensible defaults
func buildEngineConfig(cfg *config.RoutingConfig, snapDistance, speedKmh float64) ch.EngineConfig {
	return ch.EngineConfig{
		MaxSnapDistanceMeters:     snapDistance * 1000, // Convert km to meters
		DefaultSpeedKmH:           speedKmh,
		MaxQueryRadiusMeters:      getFloatWithDefault(cfg.MaxQueryRadiusKm*1000, 10000),
		OneToManyWorkers:          getIntWithDefault(cfg.OneToManyWorkers, 20),
		PreFilterRadiusMultiplier: getFloatWithDefault(cfg.PreFilterRadiusMultiplier, 1.3),
		GridCellSizeKm:            getFloatWithDefault(cfg.GridCellSizeKm, 1.0),
	}
}

func getFloatWithDefault(value, defaultValue float64) float64 {
	if value <= 0 {
		return defaultValue
	}

	return value
}

func getIntWithDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}

	return value
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

	var (
		result *usecase.OneToManyResult
		err    error
	)

	// Use CH engine if ready, otherwise fallback to Haversine
	if s.IsReady() {
		result, err = s.oneToManyCH(ctx, source, targets)
	} else {
		result, err = s.oneToManyHaversine(ctx, source, targets)
	}

	if err != nil {
		return nil, err
	}

	// Always stamp duration here to avoid double-timing in helpers
	if duration := time.Since(startTime); duration > 0 {
		result.Duration = duration
	} else {
		result.Duration = time.Nanosecond
	}

	return result, nil
}

// oneToManyCH implements OneToMany using CH routing engine
func (s *routingService) oneToManyCH(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) (*usecase.OneToManyResult, error) {
	// Convert usecase coordinates to CH coordinates
	chSource := ch.Coordinate{Lat: source.Lat, Lng: source.Lng}
	chTargets := make([]ch.Coordinate, len(targets))
	for idx, target := range targets {
		chTargets[idx] = ch.Coordinate{Lat: target.Lat, Lng: target.Lng}
	}

	// Call CH engine
	chResults, err := s.engine.OneToMany(ctx, chSource, chTargets)
	if err != nil {
		// Fallback to Haversine on CH engine error
		s.logger.Warn("CH engine OneToMany failed, falling back to Haversine", "error", err)

		return s.oneToManyHaversine(ctx, source, targets)
	}

	// Convert CH results to usecase results
	results := make([]usecase.RouteResult, len(chResults))
	for idx, chResult := range chResults {
		results[idx] = usecase.RouteResult{
			Source:      source,
			Target:      targets[chResult.TargetIdx],
			DistanceKm:  chResult.Distance / 1000, // Convert meters to km
			DurationMin: chResult.Duration.Minutes(),
			IsReachable: chResult.IsReachable,
		}
	}

	return &usecase.OneToManyResult{
		Source:  source,
		Targets: targets,
		Results: results,
	}, nil
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
	if s.maxSnapDistanceKm <= 0 {
		return nil, false, errors.New("invalid max snap distance configuration")
	}

	// Check if coordinate is within reasonable bounds
	if !s.isValidCoordinate(coord) {
		return nil, false, errors.New("coordinate is outside valid bounds")
	}

	// Use CH engine if ready
	if s.IsReady() {
		chCoord := ch.Coordinate{Lat: coord.Lat, Lng: coord.Lng}
		result, findErr := s.engine.FindNearestNode(ctx, chCoord)
		if findErr != nil {
			// Return mock on error (GPS too far from road)
			return nil, false, errors.Wrap(findErr, "failed to find nearest node")
		}

		nodeInfo := &usecase.NodeInfo{
			ID:       usecase.NodeID(result.NodeID),
			Location: usecase.Coordinate{Lat: result.NodeLat, Lng: result.NodeLng},
		}

		return nodeInfo, result.IsValid, nil
	}

	// Fallback: return the input coordinate as a mock node
	nodeInfo := &usecase.NodeInfo{
		ID:       usecase.NodeID(1),
		Location: coord,
	}

	return nodeInfo, true, nil
}

// CalculateDistance calculates the road network distance between two coordinates
func (s *routingService) CalculateDistance(ctx context.Context, source, target usecase.Coordinate) (*usecase.RouteResult, error) {
	// Use CH engine if ready
	if s.IsReady() {
		chSource := ch.Coordinate{Lat: source.Lat, Lng: source.Lng}
		chTarget := ch.Coordinate{Lat: target.Lat, Lng: target.Lng}

		chResult, chErr := s.engine.ShortestPath(ctx, chSource, chTarget)
		if chErr != nil {
			// Fallback to Haversine on error (log but don't fail)
			s.logger.Warn("CH ShortestPath failed, using Haversine fallback", "error", chErr)
			result := s.calculateHaversineDistance(source, target)

			return &result, nil
		}

		return &usecase.RouteResult{
			Source:      source,
			Target:      target,
			DistanceKm:  chResult.Distance / 1000,
			DurationMin: chResult.Duration.Minutes(),
			IsReachable: chResult.IsReachable,
		}, nil
	}

	// Fallback to Haversine
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

	return s.engine != nil && s.engine.IsReady()
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

	for range workerCount {
		workerGroup.Go(func() {
			for idx := range targetCh {
				if ctx.Err() != nil {
					return
				}

				result := s.calculateHaversineDistance(source, targets[idx])
				resultCh <- routeResultWithIndex{index: idx, result: result}
			}
		})
	}

	return &workerGroup
}

func (s *routingService) dispatchRouteWork(ctx context.Context, targetCh chan<- int, targetCount int) {
	defer close(targetCh)

	for i := range targetCount {
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
