package pmtiles

import (
	"container/heap"
	"math"
	"strconv"

	"github.com/paulmach/orb"
)

// NodeID represents a unique node identifier in the road graph
type NodeID int64

// Edge represents a directed edge in the road graph
type Edge struct {
	To       NodeID
	Distance float64 // Distance in meters
	Duration float64 // Duration in seconds
}

// RoadGraph represents the road network graph built from MVT data
type RoadGraph struct {
	Nodes    map[NodeID]orb.Point
	Edges    map[NodeID][]Edge
	nodeIdx  int64
	pointMap map[string]NodeID // Maps "lat,lng" to NodeID for deduplication
}

// NewRoadGraph creates a new empty road graph
func NewRoadGraph() *RoadGraph {
	return &RoadGraph{
		Nodes:    make(map[NodeID]orb.Point),
		Edges:    make(map[NodeID][]Edge),
		nodeIdx:  0,
		pointMap: make(map[string]NodeID),
	}
}

// AddSegment adds a road segment to the graph
func (g *RoadGraph) AddSegment(segment *RoadSegment) {
	if len(segment.Points) < 2 {
		return
	}

	// Add nodes and edges for each consecutive pair of points
	prevNodeID := g.getOrCreateNode(segment.Points[0])

	for i := 1; i < len(segment.Points); i++ {
		currNodeID := g.getOrCreateNode(segment.Points[i])

		// Calculate distance and duration
		dist := haversineDistance(segment.Points[i-1], segment.Points[i])
		speed := segment.MaxSpeed
		if speed <= 0 {
			speed = 30.0 // Default 30 km/h
		}
		duration := (dist / 1000.0 / speed) * 3600.0 // Convert to seconds

		// Add forward edge
		g.Edges[prevNodeID] = append(g.Edges[prevNodeID], Edge{
			To:       currNodeID,
			Distance: dist,
			Duration: duration,
		})

		// Add reverse edge if not one-way
		if !segment.OneWay {
			g.Edges[currNodeID] = append(g.Edges[currNodeID], Edge{
				To:       prevNodeID,
				Distance: dist,
				Duration: duration,
			})
		}

		prevNodeID = currNodeID
	}
}

// getOrCreateNode gets an existing node or creates a new one
func (g *RoadGraph) getOrCreateNode(point orb.Point) NodeID {
	// Create a key for the point (rounded to avoid floating point issues)
	key := pointKey(point)

	if id, exists := g.pointMap[key]; exists {
		return id
	}

	g.nodeIdx++
	id := NodeID(g.nodeIdx)
	g.Nodes[id] = point
	g.pointMap[key] = id

	return id
}

// pointKey creates a string key for a point (rounded to ~1m precision)
func pointKey(p orb.Point) string {
	// Round to 5 decimal places (~1m precision)
	lat := math.Round(p[1]*100000) / 100000
	lng := math.Round(p[0]*100000) / 100000

	return formatFloat(lat) + "," + formatFloat(lng)
}

// formatFloat formats a float for use as a map key
func formatFloat(f float64) string {
	// Use strconv for correct float-to-string conversion
	return strconv.FormatFloat(f, 'f', 5, 64)
}

// FindNearestNode finds the nearest node to a given point
func (g *RoadGraph) FindNearestNode(point orb.Point) (NodeID, float64, bool) {
	if len(g.Nodes) == 0 {
		return 0, 0, false
	}

	var nearestID NodeID
	nearestDist := math.MaxFloat64

	for id, nodePoint := range g.Nodes {
		dist := haversineDistance(point, nodePoint)
		if dist < nearestDist {
			nearestDist = dist
			nearestID = id
		}
	}

	return nearestID, nearestDist, true
}

// PathResult represents the result of a path search
type PathResult struct {
	Distance    float64 // Total distance in meters
	Duration    float64 // Total duration in seconds
	IsReachable bool
}

// Pathfinder implements shortest path algorithms on the road graph
type Pathfinder struct {
	graph *RoadGraph
}

// NewPathfinder creates a new pathfinder for the given graph
func NewPathfinder(graph *RoadGraph) *Pathfinder {
	return &Pathfinder{graph: graph}
}

// dijkstraNode represents a node in the priority queue
type dijkstraNode struct {
	id       NodeID
	distance float64
	duration float64
	index    int // Index in the heap
}

// priorityQueue implements heap.Interface for Dijkstra's algorithm
type priorityQueue []*dijkstraNode

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].distance < pq[j].distance
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	node := x.(*dijkstraNode)
	node.index = n
	*pq = append(*pq, node)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.index = -1
	*pq = old[0 : n-1]

	return node
}

// ShortestPath finds the shortest path from source to target using Dijkstra's algorithm
func (pf *Pathfinder) ShortestPath(sourceID, targetID NodeID) PathResult {
	if _, exists := pf.graph.Nodes[sourceID]; !exists {
		return PathResult{IsReachable: false}
	}
	if _, exists := pf.graph.Nodes[targetID]; !exists {
		return PathResult{IsReachable: false}
	}

	// Initialize distances
	distances, durations, visited := pf.initDijkstraState()
	distances[sourceID] = 0
	durations[sourceID] = 0

	// Initialize priority queue
	priorityQueue := make(priorityQueue, 0)
	heap.Init(&priorityQueue)
	heap.Push(&priorityQueue, &dijkstraNode{
		id:       sourceID,
		distance: 0,
		duration: 0,
	})

	for priorityQueue.Len() > 0 {
		current := heap.Pop(&priorityQueue).(*dijkstraNode)

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		// Found target
		if current.id == targetID {
			return PathResult{
				Distance:    current.distance,
				Duration:    current.duration,
				IsReachable: true,
			}
		}

		// Process neighbors
		pf.relaxEdges(current, distances, durations, visited, &priorityQueue)
	}

	// Target not reachable
	return PathResult{IsReachable: false}
}

// ShortestPathToMany finds shortest paths from source to multiple targets
func (pf *Pathfinder) ShortestPathToMany(sourceID NodeID, targetIDs []NodeID) []PathResult {
	results := make([]PathResult, len(targetIDs))

	// Create a set of targets for quick lookup
	targetSet, remainingTargets := pf.initTargetSet(targetIDs)

	if _, exists := pf.graph.Nodes[sourceID]; !exists || remainingTargets == 0 {
		return results
	}

	// Initialize distances
	distances, durations, visited := pf.initDijkstraState()
	distances[sourceID] = 0
	durations[sourceID] = 0

	// Initialize priority queue
	priorityQueue := make(priorityQueue, 0)
	heap.Init(&priorityQueue)
	heap.Push(&priorityQueue, &dijkstraNode{
		id:       sourceID,
		distance: 0,
		duration: 0,
	})

	for priorityQueue.Len() > 0 && remainingTargets > 0 {
		current := heap.Pop(&priorityQueue).(*dijkstraNode)

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		// Check if current node is a target
		if idx, isTarget := targetSet[current.id]; isTarget {
			results[idx] = PathResult{
				Distance:    current.distance,
				Duration:    current.duration,
				IsReachable: true,
			}
			remainingTargets--
		}

		// Process neighbors
		pf.relaxEdges(current, distances, durations, visited, &priorityQueue)
	}

	return results
}

func (pf *Pathfinder) initTargetSet(targetIDs []NodeID) (map[NodeID]int, int) {
	targetSet := make(map[NodeID]int)
	remainingTargets := 0
	for i, targetID := range targetIDs {
		if _, exists := pf.graph.Nodes[targetID]; exists {
			targetSet[targetID] = i
			remainingTargets++
		}
	}

	return targetSet, remainingTargets
}

func (pf *Pathfinder) initDijkstraState() (map[NodeID]float64, map[NodeID]float64, map[NodeID]bool) {
	distances := make(map[NodeID]float64)
	durations := make(map[NodeID]float64)
	visited := make(map[NodeID]bool)
	for id := range pf.graph.Nodes {
		distances[id] = math.MaxFloat64
		durations[id] = math.MaxFloat64
	}

	return distances, durations, visited
}

func (pf *Pathfinder) relaxEdges(current *dijkstraNode, distances, durations map[NodeID]float64, visited map[NodeID]bool, priorityQueue *priorityQueue) {
	for _, edge := range pf.graph.Edges[current.id] {
		if visited[edge.To] {
			continue
		}

		newDist := current.distance + edge.Distance
		if newDist < distances[edge.To] {
			distances[edge.To] = newDist
			durations[edge.To] = current.duration + edge.Duration
			heap.Push(priorityQueue, &dijkstraNode{
				id:       edge.To,
				distance: newDist,
				duration: current.duration + edge.Duration,
			})
		}
	}
}

// haversineDistance calculates the distance between two points in meters
func haversineDistance(p1, point2 orb.Point) float64 {
	const earthRadiusM = 6371000.0

	lat1Rad := p1[1] * math.Pi / 180
	lng1Rad := p1[0] * math.Pi / 180
	lat2Rad := point2[1] * math.Pi / 180
	lng2Rad := point2[0] * math.Pi / 180

	deltaLat := lat2Rad - lat1Rad
	deltaLng := lng2Rad - lng1Rad

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusM * c
}
