package ch

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"radar/internal/infra/routing/loader"

	"github.com/pkg/errors"
)

// ErrSnapDistanceExceeded is returned when a coordinate is too far from the road network
var ErrSnapDistanceExceeded = errors.New("coordinate too far from road network")

// ErrEngineNotReady is returned when the engine hasn't been initialized
var ErrEngineNotReady = errors.New("routing engine not ready")

// ErrUnreachable is returned when no route exists between two points
var ErrUnreachable = errors.New("destination is unreachable")

// Coordinate represents a geographic coordinate
type Coordinate struct {
	Lat float64
	Lng float64
}

// RouteResult represents the result of a routing calculation
type RouteResult struct {
	SourceIdx   int           // Source index in the input array
	TargetIdx   int           // Target index in the input array
	Distance    float64       // Road network distance in meters
	Duration    time.Duration // Estimated travel time
	IsReachable bool          // Whether the destination is reachable via road network
}

// NearestNodeResult represents the result of finding the nearest road network node
type NearestNodeResult struct {
	NodeID   int     // Internal graph node ID (slice index)
	Distance float64 // Distance from input coordinate to node in meters
	NodeLat  float64 // Node latitude
	NodeLng  float64 // Node longitude
	IsValid  bool    // Whether snap was successful (within MaxSnapDistance)
}

// EngineConfig holds configuration for the routing engine
type EngineConfig struct {
	MaxSnapDistanceMeters     float64 // Maximum distance to snap GPS to road network
	DefaultSpeedKmH           float64 // Default speed for ETA calculation
	MaxQueryRadiusMeters      float64 // Maximum query radius
	OneToManyWorkers          int     // Concurrent workers for One-to-Many
	PreFilterRadiusMultiplier float64 // Haversine pre-filter multiplier
	GridCellSizeKm            float64 // Grid cell size for spatial index
}

// DefaultEngineConfig returns sensible defaults for Taiwan
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxSnapDistanceMeters:     500,   // 500m - Taiwan urban default
		DefaultSpeedKmH:           30,    // 30 km/h urban scooter speed
		MaxQueryRadiusMeters:      10000, // 10 km
		OneToManyWorkers:          20,
		PreFilterRadiusMultiplier: 1.3,
		GridCellSizeKm:            1.0, // 1km grid cells
	}
}

// Engine wraps the CH routing functionality
type Engine struct {
	config    EngineConfig
	spatial   *GridIndex
	vertices  []loader.Vertex
	edges     []loader.Edge
	shortcuts []loader.Shortcut
	metadata  *loader.RoutingMetadata
	logger    *slog.Logger
	ready     bool
	mu        sync.RWMutex

	// Adjacency list for graph traversal
	// adjList[v] contains pairs of (neighbor_vertex, edge_weight)
	adjList [][]edgeEntry

	// CH graph integration will be added when LdDl/ch is imported
	// chGraph   *ch.Graph
	// queryPool *ch.QueryPool
}

type edgeEntry struct {
	to     int
	weight float64
}

// NewEngine creates a new routing engine instance
func NewEngine(config EngineConfig, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}

	return &Engine{
		config: config,
		logger: logger,
		ready:  false,
	}
}

// LoadData loads routing data from the specified directory
func (e *Engine) LoadData(dataDir string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.ready = false

	// Load metadata
	metadata, err := loader.LoadMetadata(dataDir)
	if err != nil {
		e.logger.Warn("Failed to load routing metadata", "error", err)
		// Continue without metadata - it's not strictly required
	} else {
		if err := metadata.Validate(); err != nil {
			e.logger.Warn("Routing metadata validation failed", "error", err)
		}
		e.metadata = metadata
	}

	// Load graph data
	csvLoader := loader.NewCSVLoader(dataDir)
	graphData, err := csvLoader.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load graph data")
	}

	e.vertices = graphData.Vertices
	e.edges = graphData.Edges
	e.shortcuts = graphData.Shortcuts

	// Build spatial index
	e.spatial = NewGridIndex(e.config.GridCellSizeKm)
	e.spatial.Build(e.vertices)

	// Build adjacency list
	e.buildAdjacencyList()

	// Log startup info
	e.logMetadata()

	e.ready = true
	e.logger.Info("Routing engine loaded successfully",
		"vertices", len(e.vertices),
		"edges", len(e.edges),
		"shortcuts", len(e.shortcuts),
	)

	return nil
}

func (e *Engine) buildAdjacencyList() {
	if len(e.vertices) == 0 {
		return
	}

	e.adjList = make([][]edgeEntry, len(e.vertices))
	e.addEdgesToAdjList()
	e.addShortcutsToAdjList()
}

func (e *Engine) addEdgesToAdjList() {
	for _, edge := range e.edges {
		from := int(edge.From)
		toNode := int(edge.To)
		if e.isValidVertexRange(from, toNode) {
			e.adjList[from] = append(e.adjList[from], edgeEntry{to: toNode, weight: edge.Weight})
		}
	}
}

func (e *Engine) addShortcutsToAdjList() {
	for _, shortcut := range e.shortcuts {
		from := int(shortcut.From)
		toNode := int(shortcut.To)
		if e.isValidVertexRange(from, toNode) {
			e.adjList[from] = append(e.adjList[from], edgeEntry{to: toNode, weight: shortcut.Weight})
		}
	}
}

func (e *Engine) isValidVertexRange(from, toNode int) bool {
	return from >= 0 && from < len(e.vertices) && toNode >= 0 && toNode < len(e.vertices)
}

func (e *Engine) logMetadata() {
	if e.metadata == nil {
		e.logger.Info("Routing engine initialized without metadata")

		return
	}

	e.logger.Info("Routing engine initialized",
		"region", e.metadata.Source.Region,
		"osm_timestamp", e.metadata.Source.OSMTimestamp,
		"generated_at", e.metadata.Processing.GeneratedAt,
		"profile", e.metadata.Processing.Profile,
		"vertices", e.metadata.Output.VerticesCount,
		"edges", e.metadata.Output.EdgesCount,
		"shortcuts", e.metadata.Output.ShortcutsCount,
	)
}

// IsReady returns whether the engine is ready for queries
func (e *Engine) IsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.ready
}

// FindNearestNode finds the nearest road network node to a coordinate
func (e *Engine) FindNearestNode(ctx context.Context, coord Coordinate) (*NearestNodeResult, error) {
	if !e.IsReady() {
		return nil, ErrEngineNotReady
	}

	// Find nearest using spatial index
	vertexIdx, ok := e.spatial.Nearest(coord.Lat, coord.Lng)
	if !ok {
		return nil, errors.New("no nodes in spatial index")
	}

	vertex := e.spatial.GetVertex(vertexIdx)
	if vertex == nil {
		return nil, errors.New("vertex not found")
	}

	// Calculate actual distance (Haversine) in meters
	distance := haversineMeters(coord.Lat, coord.Lng, vertex.Lat, vertex.Lng)

	result := &NearestNodeResult{
		NodeID:   vertexIdx,
		Distance: distance,
		NodeLat:  vertex.Lat,
		NodeLng:  vertex.Lng,
		IsValid:  distance <= e.config.MaxSnapDistanceMeters,
	}

	if !result.IsValid {
		return result, ErrSnapDistanceExceeded
	}

	return result, nil
}

// ShortestPath calculates the shortest path between two coordinates
func (e *Engine) ShortestPath(ctx context.Context, from, target Coordinate) (*RouteResult, error) {
	if !e.IsReady() {
		return nil, ErrEngineNotReady
	}

	// Snap source
	srcNode, err := e.FindNearestNode(ctx, from)
	if err != nil {
		return &RouteResult{IsReachable: false}, err
	}

	// Snap target
	dstNode, err := e.FindNearestNode(ctx, target)
	if err != nil {
		return &RouteResult{
			TargetIdx:   0,
			IsReachable: false,
		}, err
	}

	// Calculate shortest path using Dijkstra (placeholder until CH integration)
	distance, reachable := e.dijkstra(srcNode.NodeID, dstNode.NodeID)

	if !reachable {
		return &RouteResult{
			IsReachable: false,
		}, nil
	}

	// Calculate ETA
	duration := e.calculateDuration(distance)

	return &RouteResult{
		Distance:    distance,
		Duration:    duration,
		IsReachable: true,
	}, nil
}

// snapResult holds snap information for routing
type snapResult struct {
	originalIdx int
	targetNode  int
}

// OneToMany calculates routes from one source to multiple targets
func (e *Engine) OneToMany(ctx context.Context, from Coordinate, targets []Coordinate) ([]RouteResult, error) {
	if !e.IsReady() {
		return nil, ErrEngineNotReady
	}

	results := make([]RouteResult, len(targets))

	// Snap source (fail fast if source is invalid)
	srcNode, err := e.FindNearestNode(ctx, from)
	if err != nil {
		return e.markAllUnreachable(results), err
	}

	// Pre-filter targets by Haversine distance
	candidateRadius := e.config.MaxQueryRadiusMeters * e.config.PreFilterRadiusMultiplier
	candidateIdxs := e.preFilterByHaversine(from, targets, candidateRadius)

	// Mark non-candidates as unreachable
	for idx := range results {
		results[idx] = RouteResult{TargetIdx: idx, IsReachable: false}
	}

	// Snap targets and prepare for routing
	snapped := e.snapTargets(ctx, candidateIdxs, targets)
	if len(snapped) == 0 {
		return results, nil
	}

	// Route to all snapped targets using worker pool
	return e.routeWithWorkerPool(ctx, srcNode.NodeID, snapped, results)
}

func (e *Engine) markAllUnreachable(results []RouteResult) []RouteResult {
	for idx := range results {
		results[idx] = RouteResult{TargetIdx: idx, IsReachable: false}
	}

	return results
}

func (e *Engine) snapTargets(ctx context.Context, candidateIdxs []int, targets []Coordinate) []snapResult {
	snapped := make([]snapResult, 0, len(candidateIdxs))

	for _, idx := range candidateIdxs {
		nearestNode, snapErr := e.FindNearestNode(ctx, targets[idx])
		if snapErr != nil {
			// Target too far from road network
			continue
		}
		snapped = append(snapped, snapResult{originalIdx: idx, targetNode: nearestNode.NodeID})
	}

	return snapped
}

func (e *Engine) routeWithWorkerPool(ctx context.Context, srcNodeID int, snapped []snapResult, results []RouteResult) ([]RouteResult, error) {
	workerCount := min(e.config.OneToManyWorkers, len(snapped))
	if workerCount <= 0 {
		return results, nil
	}

	jobs := make(chan snapResult, len(snapped))
	resultsCh := make(chan routingResult, len(snapped))

	// Start workers
	var waitGroup sync.WaitGroup
	for workerIdx := 0; workerIdx < workerCount; workerIdx++ {
		waitGroup.Add(1)
		go e.routingWorker(ctx, &waitGroup, srcNodeID, jobs, resultsCh)
	}

	// Send jobs
	go func() {
		for _, job := range snapped {
			jobs <- job
		}
		close(jobs)
	}()

	// Collect results
	go func() {
		waitGroup.Wait()
		close(resultsCh)
	}()

	for res := range resultsCh {
		results[res.idx] = res.result
	}

	return results, nil
}

type routingResult struct {
	idx    int
	result RouteResult
}

func (e *Engine) routingWorker(ctx context.Context, waitGroup *sync.WaitGroup, srcNodeID int, jobs <-chan snapResult, resultsCh chan<- routingResult) {
	defer waitGroup.Done()

	for job := range jobs {
		if ctx.Err() != nil {
			return
		}

		distance, reachable := e.dijkstra(srcNodeID, job.targetNode)
		result := RouteResult{
			TargetIdx:   job.originalIdx,
			Distance:    distance,
			Duration:    e.calculateDuration(distance),
			IsReachable: reachable,
		}

		resultsCh <- routingResult{idx: job.originalIdx, result: result}
	}
}

func (e *Engine) preFilterByHaversine(source Coordinate, targets []Coordinate, radiusMeters float64) []int {
	var candidates []int
	for idx, target := range targets {
		dist := haversineMeters(source.Lat, source.Lng, target.Lat, target.Lng)
		if dist <= radiusMeters {
			candidates = append(candidates, idx)
		}
	}

	return candidates
}

func (e *Engine) calculateDuration(distanceMeters float64) time.Duration {
	if e.config.DefaultSpeedKmH <= 0 {
		return 0
	}
	// distance (m) / speed (km/h) = time in hours
	// time (hours) = distance (km) / speed (km/h)
	// time (seconds) = (distance_m / 1000) / speed_kmh * 3600
	speedMps := e.config.DefaultSpeedKmH * 1000 / 3600 // meters per second
	seconds := distanceMeters / speedMps

	return time.Duration(seconds * float64(time.Second))
}

// dijkstra performs Dijkstra's shortest path algorithm
// Returns (distance in meters, reachable)
func (e *Engine) dijkstra(source, target int) (float64, bool) {
	if !e.isValidVertexRange(source, target) {
		return 0, false
	}

	// Same node
	if source == target {
		return 0, true
	}

	distances := e.initializeDistances(source)
	priorityQueue := []pqItem{{node: source, dist: 0}}

	return e.runDijkstraSearch(target, distances, priorityQueue)
}

func (e *Engine) initializeDistances(source int) []float64 {
	const inf = math.MaxFloat64
	distances := make([]float64, len(e.vertices))
	for idx := range distances {
		distances[idx] = inf
	}
	distances[source] = 0

	return distances
}

type pqItem struct {
	node int
	dist float64
}

func (e *Engine) runDijkstraSearch(target int, distances []float64, priorityQueue []pqItem) (float64, bool) {
	for len(priorityQueue) > 0 {
		// Find minimum
		minIdx := e.findMinInQueue(priorityQueue)
		current := priorityQueue[minIdx]
		priorityQueue[minIdx] = priorityQueue[len(priorityQueue)-1]
		priorityQueue = priorityQueue[:len(priorityQueue)-1]

		if current.node == target {
			return distances[target], true
		}

		if current.dist > distances[current.node] {
			continue
		}

		priorityQueue = e.relaxEdges(current.node, distances, priorityQueue)
	}

	return 0, false
}

func (e *Engine) findMinInQueue(priorityQueue []pqItem) int {
	minIdx := 0
	for idx := 1; idx < len(priorityQueue); idx++ {
		if priorityQueue[idx].dist < priorityQueue[minIdx].dist {
			minIdx = idx
		}
	}

	return minIdx
}

func (e *Engine) relaxEdges(currentNode int, distances []float64, priorityQueue []pqItem) []pqItem {
	for _, edge := range e.adjList[currentNode] {
		newDist := distances[currentNode] + edge.weight
		if newDist < distances[edge.to] {
			distances[edge.to] = newDist
			priorityQueue = append(priorityQueue, pqItem{node: edge.to, dist: newDist})
		}
	}

	return priorityQueue
}

// haversineMeters calculates the great circle distance between two points in meters
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusMeters = 6371000.0

	lat1Rad := lat1 * math.Pi / 180
	lng1Rad := lng1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lng2Rad := lng2 * math.Pi / 180

	dLat := lat2Rad - lat1Rad
	dLng := lng2Rad - lng1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMeters * c
}

// GetMetadata returns the loaded metadata
func (e *Engine) GetMetadata() *loader.RoutingMetadata {
	return e.metadata
}
