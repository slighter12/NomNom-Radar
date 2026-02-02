package pmtiles

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"radar/config"
	"radar/internal/usecase"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"github.com/pkg/errors"
	"github.com/protomaps/go-pmtiles/pmtiles"
	"go.uber.org/fx"
)

// pmtilesRoutingService implements RoutingUsecase using PMTiles for tile data
type pmtilesRoutingService struct {
	source      string
	tilesetName string // Name of the tileset (extracted from filename, e.g., "walking" from "walking.pmtiles")
	roadLayer   string
	zoomLevel   int
	logger      *slog.Logger
	server      *pmtiles.Server
	parser      *MVTParser

	// Cache for loaded tiles
	tileCache   map[string]*RoadGraph
	tileCacheMu sync.RWMutex
}

// PMTilesServiceParams holds dependencies for PMTiles routing service
type PMTilesServiceParams struct {
	fx.In

	Config *config.PMTilesConfig `optional:"true"`
	Logger *slog.Logger
}

// NewPMTilesRoutingService creates a new PMTiles-based routing service
func NewPMTilesRoutingService(params PMTilesServiceParams) (usecase.RoutingUsecase, error) {
	cfg := params.Config
	logger := params.Logger

	if cfg == nil || !cfg.Enabled {
		logger.Info("PMTiles routing disabled, using Haversine fallback")

		return newHaversineFallbackService(logger), nil
	}

	if cfg.Source == "" {
		return nil, errors.New("PMTiles source is required when enabled")
	}

	roadLayer := cfg.RoadLayer
	if roadLayer == "" {
		roadLayer = "transportation" // Default OpenMapTiles layer name
	}

	zoomLevel := cfg.ZoomLevel
	if zoomLevel == 0 {
		zoomLevel = 14 // Default zoom level for routing
	}

	// Parse source to extract bucket directory and tileset name
	// The PMTiles server expects a bucket (directory) and looks for {name}.pmtiles files
	bucketPath, tilesetName := parseSourcePath(cfg.Source)

	// Create a silent logger for pmtiles (it requires *log.Logger)
	silentLogger := log.New(io.Discard, "", 0)

	// Create PMTiles server - handles local files, HTTP, and cloud storage
	// bucketPath is the directory, tilesetName is derived from filename
	cacheSize := 64 // Cache up to 64 tiles in memory
	server, err := pmtiles.NewServer(bucketPath, "", silentLogger, cacheSize, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create PMTiles server")
	}

	// Start the server (required for serving tiles)
	server.Start()

	svc := &pmtilesRoutingService{
		source:      cfg.Source,
		tilesetName: tilesetName,
		roadLayer:   roadLayer,
		zoomLevel:   zoomLevel,
		logger:      logger,
		server:      server,
		parser:      NewMVTParser(roadLayer),
		tileCache:   make(map[string]*RoadGraph),
	}

	logger.Info("PMTiles routing service initialized",
		slog.String("source", cfg.Source),
		slog.String("tileset", tilesetName),
		slog.String("road_layer", roadLayer),
		slog.Int("zoom_level", zoomLevel),
	)

	return svc, nil
}

// tileKey creates a string key for a tile
func tileKey(tile maptile.Tile) string {
	return fmt.Sprintf("%d/%d/%d", tile.Z, tile.X, tile.Y)
}

// parseSourcePath extracts the bucket directory and tileset name from a source path.
// Examples:
//   - "file:///path/to/walking.pmtiles" -> ("file:///path/to", "walking")
//   - "/path/to/walking.pmtiles" -> ("file:///path/to", "walking")
//   - "https://example.com/tiles/walking.pmtiles" -> ("https://example.com/tiles", "walking")
func parseSourcePath(source string) (bucketPath, tilesetName string) {
	// Handle file:// prefix
	if strings.HasPrefix(source, "file://") {
		path := strings.TrimPrefix(source, "file://")
		dir := filepath.Dir(path)
		filename := filepath.Base(path)
		tilesetName = strings.TrimSuffix(filename, ".pmtiles")

		return "file://" + dir, tilesetName
	}

	// Handle HTTP/HTTPS URLs
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		lastSlash := strings.LastIndex(source, "/")
		if lastSlash > 0 {
			bucketPath = source[:lastSlash]
			filename := source[lastSlash+1:]
			tilesetName = strings.TrimSuffix(filename, ".pmtiles")

			return bucketPath, tilesetName
		}
	}

	// Handle local file path without prefix
	dir := filepath.Dir(source)
	filename := filepath.Base(source)
	tilesetName = strings.TrimSuffix(filename, ".pmtiles")

	return "file://" + dir, tilesetName
}

// OneToMany calculates routes from one source to multiple targets
func (s *pmtilesRoutingService) OneToMany(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) (*usecase.OneToManyResult, error) {
	startTime := time.Now()

	if len(targets) == 0 {
		return &usecase.OneToManyResult{
			Source:   source,
			Targets:  targets,
			Results:  []usecase.RouteResult{},
			Duration: time.Since(startTime),
		}, nil
	}

	// Build road graph for the area covering source and all targets
	graph := s.buildGraphForArea(ctx, source, targets)

	// Find nearest nodes
	sourcePoint := orb.Point{source.Lng, source.Lat}
	sourceNodeID, sourceSnapDist, found := graph.FindNearestNode(sourcePoint)
	if !found || sourceSnapDist > 500 { // Max 500m snap distance
		s.logger.Debug("Source too far from road network, using Haversine fallback",
			slog.Float64("snap_distance", sourceSnapDist),
		)

		return s.haversineFallback(source, targets, startTime)
	}

	// Find nearest nodes for all targets
	targetNodeIDs := make([]NodeID, len(targets))
	targetSnapDistances := make([]float64, len(targets))
	for i, target := range targets {
		targetPoint := orb.Point{target.Lng, target.Lat}
		nodeID, snapDist, ok := graph.FindNearestNode(targetPoint)
		if ok && snapDist <= 500 {
			targetNodeIDs[i] = nodeID
			targetSnapDistances[i] = snapDist
		}
	}

	// Run pathfinding
	pathfinder := NewPathfinder(graph)
	pathResults := pathfinder.ShortestPathToMany(sourceNodeID, targetNodeIDs)

	// Convert results
	results := make([]usecase.RouteResult, len(targets))
	for idx, pathResult := range pathResults {
		if pathResult.IsReachable {
			// Add snap distances to the total
			totalDistance := pathResult.Distance + sourceSnapDist + targetSnapDistances[idx]
			results[idx] = usecase.RouteResult{
				Source:      source,
				Target:      targets[idx],
				DistanceKm:  totalDistance / 1000,
				DurationMin: pathResult.Duration / 60,
				IsReachable: true,
			}
		} else {
			// Fallback to Haversine for unreachable targets
			results[idx] = s.haversineResult(source, targets[idx])
		}
	}

	return &usecase.OneToManyResult{
		Source:   source,
		Targets:  targets,
		Results:  results,
		Duration: time.Since(startTime),
	}, nil
}

// FindNearestNode finds the nearest road network node to a coordinate
func (s *pmtilesRoutingService) FindNearestNode(ctx context.Context, coord usecase.Coordinate) (*usecase.NodeInfo, bool, error) {
	// Build a small graph around the coordinate
	graph := s.buildGraphForPoint(ctx, coord)

	point := orb.Point{coord.Lng, coord.Lat}
	nodeID, snapDist, found := graph.FindNearestNode(point)
	if !found || snapDist > 500 {
		return nil, false, nil
	}

	nodePoint := graph.Nodes[nodeID]

	return &usecase.NodeInfo{
		ID:       usecase.NodeID(nodeID),
		Location: usecase.Coordinate{Lat: nodePoint[1], Lng: nodePoint[0]},
	}, true, nil
}

// CalculateDistance calculates road distance between two coordinates
func (s *pmtilesRoutingService) CalculateDistance(ctx context.Context, source, target usecase.Coordinate) (*usecase.RouteResult, error) {
	result, err := s.OneToMany(ctx, source, []usecase.Coordinate{target})
	if err != nil {
		return nil, err
	}

	if len(result.Results) > 0 {
		return &result.Results[0], nil
	}

	// Return Haversine fallback
	hr := s.haversineResult(source, target)

	return &hr, nil
}

// IsReady returns whether the service is ready
func (s *pmtilesRoutingService) IsReady() bool {
	return s.server != nil
}

// buildGraphForArea builds a road graph covering the area between source and targets
func (s *pmtilesRoutingService) buildGraphForArea(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) *RoadGraph {
	// Calculate bounding box
	minLat, maxLat := source.Lat, source.Lat
	minLng, maxLng := source.Lng, source.Lng

	for _, target := range targets {
		if target.Lat < minLat {
			minLat = target.Lat
		}
		if target.Lat > maxLat {
			maxLat = target.Lat
		}
		if target.Lng < minLng {
			minLng = target.Lng
		}
		if target.Lng > maxLng {
			maxLng = target.Lng
		}
	}

	// Add padding (approximately 500m)
	padding := 0.005 // ~500m at equator
	minLat -= padding
	maxLat += padding
	minLng -= padding
	maxLng += padding

	// Get required tiles
	tiles := getTilesForBounds(minLat, maxLat, minLng, maxLng, maptile.Zoom(s.zoomLevel))

	// Build combined graph
	graph := NewRoadGraph()

	for _, tile := range tiles {
		tileGraph, err := s.loadTileGraph(ctx, tile)
		if err != nil {
			s.logger.Debug("Failed to load tile",
				slog.String("tile", tileKey(tile)),
				slog.Any("error", err),
			)

			continue
		}
		mergeGraphs(graph, tileGraph)
	}

	return graph
}

// buildGraphForPoint builds a road graph around a single point
func (s *pmtilesRoutingService) buildGraphForPoint(ctx context.Context, coord usecase.Coordinate) *RoadGraph {
	tile := maptile.At(orb.Point{coord.Lng, coord.Lat}, maptile.Zoom(s.zoomLevel))

	// Load the center tile and all 8 surrounding tiles (3x3 grid)
	// This ensures coverage when the point is near a tile corner
	tiles := []maptile.Tile{
		// Center
		tile,
		// Cardinal directions (up, down, left, right)
		{X: tile.X - 1, Y: tile.Y, Z: tile.Z},
		{X: tile.X + 1, Y: tile.Y, Z: tile.Z},
		{X: tile.X, Y: tile.Y - 1, Z: tile.Z},
		{X: tile.X, Y: tile.Y + 1, Z: tile.Z},
		// Diagonal directions (corners)
		{X: tile.X - 1, Y: tile.Y - 1, Z: tile.Z},
		{X: tile.X + 1, Y: tile.Y - 1, Z: tile.Z},
		{X: tile.X - 1, Y: tile.Y + 1, Z: tile.Z},
		{X: tile.X + 1, Y: tile.Y + 1, Z: tile.Z},
	}

	graph := NewRoadGraph()

	for _, t := range tiles {
		tileGraph, err := s.loadTileGraph(ctx, t)
		if err != nil {
			continue
		}
		mergeGraphs(graph, tileGraph)
	}

	return graph
}

// loadTileGraph loads and parses a single tile into a road graph
func (s *pmtilesRoutingService) loadTileGraph(ctx context.Context, tile maptile.Tile) (*RoadGraph, error) {
	cacheKey := tileKey(tile)

	// Check cache
	s.tileCacheMu.RLock()
	if graph, ok := s.tileCache[cacheKey]; ok {
		s.tileCacheMu.RUnlock()

		return graph, nil
	}
	s.tileCacheMu.RUnlock()

	// Load tile data
	data, err := s.fetchTile(ctx, tile)
	if err != nil {
		return nil, err
	}

	// Parse MVT
	segments, err := s.parser.ParseTile(data, tile)
	if err != nil {
		return nil, err
	}

	// Build graph
	graph := NewRoadGraph()
	for idx := range segments {
		graph.AddSegment(&segments[idx])
	}

	// Cache the graph
	s.tileCacheMu.Lock()
	s.tileCache[cacheKey] = graph
	s.tileCacheMu.Unlock()

	return graph, nil
}

// fetchTile fetches tile data from PMTiles using HTTP Range requests
func (s *pmtilesRoutingService) fetchTile(ctx context.Context, tile maptile.Tile) ([]byte, error) {
	// Build the tile path in the format expected by PMTiles server
	// Format: /{tileset}/{z}/{x}/{y}.mvt
	tilePath := fmt.Sprintf("/%s/%d/%d/%d.mvt", s.tilesetName, tile.Z, tile.X, tile.Y)

	// Get tile data using the PMTiles server
	// The server handles HTTP Range requests internally for remote files
	statusCode, _, data := s.server.Get(ctx, tilePath)

	if statusCode == http.StatusNotFound {
		return nil, errors.New("tile not found")
	}

	if statusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected status code: %d", statusCode)
	}

	return data, nil
}

// getTilesForBounds returns all tiles that cover the given bounds
func getTilesForBounds(minLat, maxLat, minLng, maxLng float64, zoom maptile.Zoom) []maptile.Tile {
	minTile := maptile.At(orb.Point{minLng, maxLat}, zoom)
	maxTile := maptile.At(orb.Point{maxLng, minLat}, zoom)

	tiles := make([]maptile.Tile, 0)
	for x := minTile.X; x <= maxTile.X; x++ {
		for y := minTile.Y; y <= maxTile.Y; y++ {
			tiles = append(tiles, maptile.Tile{X: x, Y: y, Z: zoom})
		}
	}

	return tiles
}

// mergeGraphs merges source graph into target graph by remapping node IDs
// to avoid collisions between tiles with independent ID spaces
func mergeGraphs(target, source *RoadGraph) {
	// Build mapping from source node IDs to target node IDs
	idMapping := make(map[NodeID]NodeID)
	for sourceID, point := range source.Nodes {
		targetID := target.getOrCreateNode(point)
		idMapping[sourceID] = targetID
	}

	// Add edges with remapped node IDs
	for sourceFromID, edges := range source.Edges {
		targetFromID := idMapping[sourceFromID]
		for _, edge := range edges {
			remappedEdge := Edge{
				To:       idMapping[edge.To],
				Distance: edge.Distance,
				Duration: edge.Duration,
			}
			target.Edges[targetFromID] = append(target.Edges[targetFromID], remappedEdge)
		}
	}
}

// haversineFallback returns Haversine-based results for all targets
func (s *pmtilesRoutingService) haversineFallback(source usecase.Coordinate, targets []usecase.Coordinate, startTime time.Time) (*usecase.OneToManyResult, error) {
	results := make([]usecase.RouteResult, len(targets))
	for i, target := range targets {
		results[i] = s.haversineResult(source, target)
	}

	return &usecase.OneToManyResult{
		Source:   source,
		Targets:  targets,
		Results:  results,
		Duration: time.Since(startTime),
	}, nil
}

// haversineResult calculates a Haversine-based result
func (s *pmtilesRoutingService) haversineResult(source, target usecase.Coordinate) usecase.RouteResult {
	p1 := orb.Point{source.Lng, source.Lat}
	p2 := orb.Point{target.Lng, target.Lat}
	dist := haversineDistance(p1, p2)

	return usecase.RouteResult{
		Source:      source,
		Target:      target,
		DistanceKm:  dist / 1000,
		DurationMin: (dist / 1000 / 30) * 60, // Assume 30 km/h
		IsReachable: true,
	}
}

// haversineFallbackService is a simple Haversine-only implementation
type haversineFallbackService struct {
	logger *slog.Logger
}

func newHaversineFallbackService(logger *slog.Logger) *haversineFallbackService {
	return &haversineFallbackService{logger: logger}
}

func (s *haversineFallbackService) OneToMany(ctx context.Context, source usecase.Coordinate, targets []usecase.Coordinate) (*usecase.OneToManyResult, error) {
	startTime := time.Now()
	results := make([]usecase.RouteResult, len(targets))

	for i, target := range targets {
		p1 := orb.Point{source.Lng, source.Lat}
		p2 := orb.Point{target.Lng, target.Lat}
		dist := haversineDistance(p1, p2)

		results[i] = usecase.RouteResult{
			Source:      source,
			Target:      target,
			DistanceKm:  dist / 1000,
			DurationMin: (dist / 1000 / 30) * 60,
			IsReachable: true,
		}
	}

	return &usecase.OneToManyResult{
		Source:   source,
		Targets:  targets,
		Results:  results,
		Duration: time.Since(startTime),
	}, nil
}

func (s *haversineFallbackService) FindNearestNode(ctx context.Context, coord usecase.Coordinate) (*usecase.NodeInfo, bool, error) {
	return &usecase.NodeInfo{
		ID:       usecase.NodeID(1),
		Location: coord,
	}, true, nil
}

func (s *haversineFallbackService) CalculateDistance(ctx context.Context, source, target usecase.Coordinate) (*usecase.RouteResult, error) {
	p1 := orb.Point{source.Lng, source.Lat}
	p2 := orb.Point{target.Lng, target.Lat}
	dist := haversineDistance(p1, p2)

	return &usecase.RouteResult{
		Source:      source,
		Target:      target,
		DistanceKm:  dist / 1000,
		DurationMin: (dist / 1000 / 30) * 60,
		IsReachable: true,
	}, nil
}

func (s *haversineFallbackService) IsReady() bool {
	return true
}
