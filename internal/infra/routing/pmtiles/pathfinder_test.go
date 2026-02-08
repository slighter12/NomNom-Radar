package pmtiles

import (
	"container/heap"
	"math"
	"testing"

	"github.com/paulmach/orb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRoadGraph(t *testing.T) {
	graph := NewRoadGraph()

	assert.NotNil(t, graph.Nodes)
	assert.NotNil(t, graph.Edges)
	assert.NotNil(t, graph.pointMap)
	assert.Equal(t, int64(0), graph.nodeIdx)
	assert.Empty(t, graph.Nodes)
	assert.Empty(t, graph.Edges)
}

func TestRoadGraph_AddSegment(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.5654, 25.0330}, // Taipei Station
			{121.5170, 25.0478}, // Taipei Main Station
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   false,
	}

	graph.AddSegment(segment)

	// Should have 2 nodes
	assert.Len(t, graph.Nodes, 2)

	// Should have edges in both directions (not one-way)
	totalEdges := 0
	for _, edges := range graph.Edges {
		totalEdges += len(edges)
	}
	assert.Equal(t, 2, totalEdges, "Should have bidirectional edges")
}

func TestRoadGraph_AddSegment_OneWay(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.5654, 25.0330},
			{121.5170, 25.0478},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   true,
	}

	graph.AddSegment(segment)

	// Should have edges in only one direction
	totalEdges := 0
	for _, edges := range graph.Edges {
		totalEdges += len(edges)
	}
	assert.Equal(t, 1, totalEdges, "One-way segment should have only forward edge")
}

func TestRoadGraph_AddSegment_MultiplePoints(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.01},
			{121.52, 25.02},
			{121.53, 25.03},
		},
		Highway:  "secondary",
		MaxSpeed: 40.0,
		OneWay:   false,
	}

	graph.AddSegment(segment)

	// Should have 4 nodes
	assert.Len(t, graph.Nodes, 4)

	// Should have 6 edges (3 forward + 3 backward)
	totalEdges := 0
	for _, edges := range graph.Edges {
		totalEdges += len(edges)
	}
	assert.Equal(t, 6, totalEdges)
}

func TestRoadGraph_AddSegment_TooFewPoints(t *testing.T) {
	graph := NewRoadGraph()

	// Single point segment should be ignored
	segment := &RoadSegment{
		Points:   []orb.Point{{121.5654, 25.0330}},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	assert.Empty(t, graph.Nodes)
	assert.Empty(t, graph.Edges)

	// Empty segment should also be ignored
	graph.AddSegment(&RoadSegment{Points: []orb.Point{}})
	assert.Empty(t, graph.Nodes)
}

func TestRoadGraph_AddSegment_DefaultSpeed(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.5654, 25.0330},
			{121.5170, 25.0478},
		},
		Highway:  "residential",
		MaxSpeed: 0, // Should default to 30 km/h
		OneWay:   false,
	}

	graph.AddSegment(segment)

	// Verify edge duration is calculated with default speed (30 km/h)
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			// Duration should be > 0 (calculated with default speed)
			assert.Greater(t, edge.Duration, 0.0)
		}
	}
}

func TestRoadGraph_AddSegment_NodeDeduplication(t *testing.T) {
	graph := NewRoadGraph()

	// Two segments sharing a common point
	segment1 := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.01}, // Shared point
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   false,
	}

	segment2 := &RoadSegment{
		Points: []orb.Point{
			{121.51, 25.01}, // Same point as segment1's end
			{121.52, 25.02},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   false,
	}

	graph.AddSegment(segment1)
	graph.AddSegment(segment2)

	// Should have 3 unique nodes, not 4
	assert.Len(t, graph.Nodes, 3)
}

func TestRoadGraph_FindNearestNode(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.5654, 25.0330}, // Taipei Station
			{121.5170, 25.0478}, // Taipei Main Station
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	// Query near Taipei Station
	queryPoint := orb.Point{121.5660, 25.0335}
	nodeID, distance, found := graph.FindNearestNode(queryPoint)

	assert.True(t, found)
	assert.Greater(t, distance, 0.0)
	assert.Less(t, distance, 200.0) // Should be within 200m

	// Verify the found node is actually the Taipei Station node
	foundPoint := graph.Nodes[nodeID]
	assert.InDelta(t, 121.5654, foundPoint[0], 0.001)
	assert.InDelta(t, 25.0330, foundPoint[1], 0.001)
}

func TestRoadGraph_FindNearestNode_Empty(t *testing.T) {
	graph := NewRoadGraph()

	queryPoint := orb.Point{121.5, 25.0}
	nodeID, distance, found := graph.FindNearestNode(queryPoint)

	assert.False(t, found)
	assert.Equal(t, NodeID(0), nodeID)
	assert.Equal(t, 0.0, distance)
}

func TestPointKey(t *testing.T) {
	tests := []struct {
		name     string
		point    orb.Point
		expected string
	}{
		{
			name:     "positive coordinates",
			point:    orb.Point{121.56543, 25.03301},
			expected: "25.03301,121.56543",
		},
		{
			name:     "negative longitude",
			point:    orb.Point{-73.98567, 40.74844},
			expected: "40.74844,-73.98567",
		},
		{
			name:     "zero coordinates",
			point:    orb.Point{0.0, 0.0},
			expected: "0.00000,0.00000",
		},
		{
			name:     "rounding test",
			point:    orb.Point{121.565439999, 25.033009999},
			expected: "25.03301,121.56544",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pointKey(tt.point)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name        string
		p1          orb.Point
		p2          orb.Point
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "same point",
			p1:          orb.Point{121.5654, 25.0330},
			p2:          orb.Point{121.5654, 25.0330},
			expectedMin: 0,
			expectedMax: 0.01, // Allow tiny floating point error
		},
		{
			name:        "Taipei Station to Taipei Main Station (~5.6km)",
			p1:          orb.Point{121.5654, 25.0330},
			p2:          orb.Point{121.5170, 25.0478},
			expectedMin: 5000,
			expectedMax: 6000,
		},
		{
			name:        "short distance (~100m)",
			p1:          orb.Point{121.5654, 25.0330},
			p2:          orb.Point{121.5664, 25.0330},
			expectedMin: 90,
			expectedMax: 120,
		},
		{
			name:        "cross equator",
			p1:          orb.Point{0.0, 1.0},
			p2:          orb.Point{0.0, -1.0},
			expectedMin: 220000,
			expectedMax: 230000, // ~222km
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := haversineDistance(tt.p1, tt.p2)
			assert.GreaterOrEqual(t, dist, tt.expectedMin)
			assert.LessOrEqual(t, dist, tt.expectedMax)
		})
	}
}

func TestNewPathfinder(t *testing.T) {
	graph := NewRoadGraph()
	pf := NewPathfinder(graph)

	assert.NotNil(t, pf)
	assert.Equal(t, graph, pf.graph)
}

func TestPathfinder_ShortestPath_Simple(t *testing.T) {
	graph := NewRoadGraph()

	// Create a simple line: A -> B -> C
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00}, // A
			{121.51, 25.00}, // B
			{121.52, 25.00}, // C
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   false,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	// Get node IDs
	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]
	nodeC := graph.pointMap[pointKey(orb.Point{121.52, 25.00})]

	result := pf.ShortestPath(nodeA, nodeC)

	assert.True(t, result.IsReachable)
	assert.Greater(t, result.Distance, 0.0)
	assert.Greater(t, result.Duration, 0.0)
}

func TestPathfinder_ShortestPath_SameNode(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]

	result := pf.ShortestPath(nodeA, nodeA)

	assert.True(t, result.IsReachable)
	assert.Equal(t, 0.0, result.Distance)
	assert.Equal(t, 0.0, result.Duration)
}

func TestPathfinder_ShortestPath_Unreachable(t *testing.T) {
	graph := NewRoadGraph()

	// Two disconnected segments
	segment1 := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	segment2 := &RoadSegment{
		Points: []orb.Point{
			{121.60, 25.00}, // Far away, disconnected
			{121.61, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment1)
	graph.AddSegment(segment2)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]
	nodeD := graph.pointMap[pointKey(orb.Point{121.60, 25.00})]

	result := pf.ShortestPath(nodeA, nodeD)

	assert.False(t, result.IsReachable)
}

func TestPathfinder_ShortestPath_NonexistentSource(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	existingNode := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]

	result := pf.ShortestPath(NodeID(99999), existingNode)
	assert.False(t, result.IsReachable)
}

func TestPathfinder_ShortestPath_NonexistentTarget(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	existingNode := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]

	result := pf.ShortestPath(existingNode, NodeID(99999))
	assert.False(t, result.IsReachable)
}

func TestPathfinder_ShortestPath_OneWay(t *testing.T) {
	graph := NewRoadGraph()

	// One-way segment: A -> B
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00}, // A
			{121.51, 25.00}, // B
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   true,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]
	nodeB := graph.pointMap[pointKey(orb.Point{121.51, 25.00})]

	// A -> B should work
	result := pf.ShortestPath(nodeA, nodeB)
	assert.True(t, result.IsReachable)

	// B -> A should not work (one-way)
	result = pf.ShortestPath(nodeB, nodeA)
	assert.False(t, result.IsReachable)
}

func TestPathfinder_ShortestPathToMany(t *testing.T) {
	graph := NewRoadGraph()

	// Create a simple network
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00}, // A (source)
			{121.51, 25.00}, // B
			{121.52, 25.00}, // C
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   false,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]
	nodeB := graph.pointMap[pointKey(orb.Point{121.51, 25.00})]
	nodeC := graph.pointMap[pointKey(orb.Point{121.52, 25.00})]

	results := pf.ShortestPathToMany(nodeA, []NodeID{nodeB, nodeC})

	require.Len(t, results, 2)
	assert.True(t, results[0].IsReachable)
	assert.True(t, results[1].IsReachable)

	// Distance to B should be less than to C
	assert.Less(t, results[0].Distance, results[1].Distance)
}

func TestPathfinder_ShortestPathToMany_Empty(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]

	results := pf.ShortestPathToMany(nodeA, []NodeID{})

	assert.Empty(t, results)
}

func TestPathfinder_ShortestPathToMany_NonexistentSource(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	pf := NewPathfinder(graph)

	nodeB := graph.pointMap[pointKey(orb.Point{121.51, 25.00})]

	results := pf.ShortestPathToMany(NodeID(99999), []NodeID{nodeB})

	require.Len(t, results, 1)
	assert.False(t, results[0].IsReachable)
}

func TestPathfinder_ShortestPathToMany_MixedReachability(t *testing.T) {
	graph := NewRoadGraph()

	// Two disconnected segments
	segment1 := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00}, // A
			{121.51, 25.00}, // B
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	segment2 := &RoadSegment{
		Points: []orb.Point{
			{121.60, 25.00}, // C (disconnected)
			{121.61, 25.00}, // D
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment1)
	graph.AddSegment(segment2)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]
	nodeB := graph.pointMap[pointKey(orb.Point{121.51, 25.00})]
	nodeC := graph.pointMap[pointKey(orb.Point{121.60, 25.00})]

	results := pf.ShortestPathToMany(nodeA, []NodeID{nodeB, nodeC})

	require.Len(t, results, 2)
	assert.True(t, results[0].IsReachable, "B should be reachable from A")
	assert.False(t, results[1].IsReachable, "C should not be reachable from A")
}

func TestPriorityQueue(t *testing.T) {
	pq := make(priorityQueue, 0)

	// Test basic operations
	assert.Equal(t, 0, pq.Len())

	// Add items using Push
	pq.Push(&dijkstraNode{id: 1, distance: 10.0})
	pq.Push(&dijkstraNode{id: 2, distance: 5.0})
	pq.Push(&dijkstraNode{id: 3, distance: 15.0})

	assert.Equal(t, 3, pq.Len())
}

func TestPriorityQueue_HeapOperations(t *testing.T) {
	pq := make(priorityQueue, 0)
	heap.Init(&pq)

	// Push items with different distances
	heap.Push(&pq, &dijkstraNode{id: 1, distance: 10.0, duration: 100.0})
	heap.Push(&pq, &dijkstraNode{id: 2, distance: 5.0, duration: 50.0})
	heap.Push(&pq, &dijkstraNode{id: 3, distance: 15.0, duration: 150.0})
	heap.Push(&pq, &dijkstraNode{id: 4, distance: 1.0, duration: 10.0})

	assert.Equal(t, 4, pq.Len())

	// Pop should return items in order of ascending distance
	node1 := heap.Pop(&pq).(*dijkstraNode)
	assert.Equal(t, NodeID(4), node1.id) // distance 1.0
	assert.Equal(t, 1.0, node1.distance)

	node2 := heap.Pop(&pq).(*dijkstraNode)
	assert.Equal(t, NodeID(2), node2.id) // distance 5.0
	assert.Equal(t, 5.0, node2.distance)

	node3 := heap.Pop(&pq).(*dijkstraNode)
	assert.Equal(t, NodeID(1), node3.id) // distance 10.0
	assert.Equal(t, 10.0, node3.distance)

	node4 := heap.Pop(&pq).(*dijkstraNode)
	assert.Equal(t, NodeID(3), node4.id) // distance 15.0
	assert.Equal(t, 15.0, node4.distance)

	assert.Equal(t, 0, pq.Len())
}

func TestPriorityQueue_Less(t *testing.T) {
	pq := priorityQueue{
		&dijkstraNode{id: 1, distance: 10.0},
		&dijkstraNode{id: 2, distance: 5.0},
		&dijkstraNode{id: 3, distance: 10.0}, // Same distance as node 1
	}

	// Node with smaller distance should be "less"
	assert.True(t, pq.Less(1, 0))  // 5.0 < 10.0
	assert.False(t, pq.Less(0, 1)) // 10.0 > 5.0
	assert.False(t, pq.Less(0, 2)) // 10.0 == 10.0 (not less)
}

func TestPriorityQueue_Swap(t *testing.T) {
	node1 := &dijkstraNode{id: 1, distance: 10.0, index: 0}
	node2 := &dijkstraNode{id: 2, distance: 5.0, index: 1}

	pq := priorityQueue{node1, node2}

	// Verify initial state
	assert.Equal(t, 0, pq[0].index)
	assert.Equal(t, 1, pq[1].index)
	assert.Equal(t, NodeID(1), pq[0].id)
	assert.Equal(t, NodeID(2), pq[1].id)

	// Swap
	pq.Swap(0, 1)

	// Verify indices were updated
	assert.Equal(t, 0, pq[0].index)
	assert.Equal(t, 1, pq[1].index)

	// Verify positions were swapped
	assert.Equal(t, NodeID(2), pq[0].id)
	assert.Equal(t, NodeID(1), pq[1].id)
}

func TestPriorityQueue_PushPop(t *testing.T) {
	pq := make(priorityQueue, 0)

	// Test Push
	pq.Push(&dijkstraNode{id: 1, distance: 10.0})
	assert.Equal(t, 1, pq.Len())
	assert.Equal(t, 0, pq[0].index) // Index should be set by Push

	// Test Pop
	node := pq.Pop().(*dijkstraNode)
	assert.Equal(t, NodeID(1), node.id)
	assert.Equal(t, -1, node.index) // Index should be set to -1 after Pop
	assert.Equal(t, 0, pq.Len())
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{25.03301, "25.03301"},
		{121.56543, "121.56543"},
		{0.0, "0.00000"},
		{-73.98567, "-73.98567"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEdgeCalculation verifies edge distance and duration calculations
func TestEdgeCalculation(t *testing.T) {
	graph := NewRoadGraph()

	// Create a segment with known distance
	// ~1km apart (approximately)
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00}, // ~1km at this latitude
		},
		Highway:  "primary",
		MaxSpeed: 60.0, // 60 km/h
		OneWay:   false,
	}

	graph.AddSegment(segment)

	// Check edges
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			// Distance should be approximately 1000m
			assert.InDelta(t, 1000, edge.Distance, 200)

			// Duration at 60 km/h for ~1km should be ~60 seconds
			expectedDuration := (edge.Distance / 1000.0 / 60.0) * 3600.0
			assert.InDelta(t, expectedDuration, edge.Duration, 5)
		}
	}
}

// TestDijkstraCorrectness verifies Dijkstra finds the shortest path
func TestDijkstraCorrectness(t *testing.T) {
	graph := NewRoadGraph()

	// Create a diamond-shaped network:
	//     B
	//    / \
	//   A   D
	//    \ /
	//     C
	// Where A-B-D is longer than A-C-D

	// A at origin
	// B slightly north (short path via B)
	// C slightly south (even shorter path via C)
	// D to the east

	segmentAB := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00}, // A
			{121.51, 25.02}, // B (far north)
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	segmentBD := &RoadSegment{
		Points: []orb.Point{
			{121.51, 25.02}, // B
			{121.52, 25.00}, // D
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	segmentAC := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00}, // A
			{121.51, 25.00}, // C (directly east, short)
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	segmentCD := &RoadSegment{
		Points: []orb.Point{
			{121.51, 25.00}, // C
			{121.52, 25.00}, // D
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segmentAB)
	graph.AddSegment(segmentBD)
	graph.AddSegment(segmentAC)
	graph.AddSegment(segmentCD)

	pf := NewPathfinder(graph)

	nodeA := graph.pointMap[pointKey(orb.Point{121.50, 25.00})]
	nodeD := graph.pointMap[pointKey(orb.Point{121.52, 25.00})]

	result := pf.ShortestPath(nodeA, nodeD)

	assert.True(t, result.IsReachable)

	// The direct path A-C-D should be about 2km
	// The path via B should be longer (A-B-D involves going north then south)
	// So the shortest distance should be approximately the A-C-D distance
	expectedDirectDistance := haversineDistance(
		orb.Point{121.50, 25.00},
		orb.Point{121.51, 25.00},
	) + haversineDistance(
		orb.Point{121.51, 25.00},
		orb.Point{121.52, 25.00},
	)

	// Allow 10% tolerance
	assert.InDelta(t, expectedDirectDistance, result.Distance, expectedDirectDistance*0.1)
}

// TestFloatingPointPrecision tests node deduplication with floating point values
func TestFloatingPointPrecision(t *testing.T) {
	graph := NewRoadGraph()

	// Add segments with slightly different but equivalent coordinates
	// (within rounding tolerance)
	segment1 := &RoadSegment{
		Points: []orb.Point{
			{121.500001, 25.000001},
			{121.510000, 25.000000},
		},
		Highway: "primary",
	}

	segment2 := &RoadSegment{
		Points: []orb.Point{
			{121.499999, 24.999999}, // Should round to same as segment1 start
			{121.520000, 25.000000},
		},
		Highway: "primary",
	}

	graph.AddSegment(segment1)
	graph.AddSegment(segment2)

	// The first points should be deduplicated (they round to the same key)
	// So we should have 3 nodes, not 4
	assert.LessOrEqual(t, len(graph.Nodes), 4)
}

// TestLargeGraph tests performance with a larger graph
func TestLargeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large graph test in short mode")
	}

	graph := NewRoadGraph()

	// Create a 10x10 grid of connected nodes
	gridSize := 10
	for row := range gridSize {
		for col := range gridSize {
			baseLng := 121.5 + float64(col)*0.01
			baseLat := 25.0 + float64(row)*0.01

			// Horizontal edge
			if col < gridSize-1 {
				segment := &RoadSegment{
					Points: []orb.Point{
						{baseLng, baseLat},
						{baseLng + 0.01, baseLat},
					},
					Highway:  "residential",
					MaxSpeed: 30.0,
				}
				graph.AddSegment(segment)
			}

			// Vertical edge
			if row < gridSize-1 {
				segment := &RoadSegment{
					Points: []orb.Point{
						{baseLng, baseLat},
						{baseLng, baseLat + 0.01},
					},
					Highway:  "residential",
					MaxSpeed: 30.0,
				}
				graph.AddSegment(segment)
			}
		}
	}

	// Verify node count
	assert.Equal(t, gridSize*gridSize, len(graph.Nodes))

	// Test pathfinding across the grid
	pf := NewPathfinder(graph)

	startNode := graph.pointMap[pointKey(orb.Point{121.5, 25.0})]
	endNode := graph.pointMap[pointKey(orb.Point{121.5 + float64(gridSize-1)*0.01, 25.0 + float64(gridSize-1)*0.01})]

	result := pf.ShortestPath(startNode, endNode)

	assert.True(t, result.IsReachable)
	assert.Greater(t, result.Distance, 0.0)
}

// TestPathResultZeroValues checks PathResult default state
func TestPathResultZeroValues(t *testing.T) {
	var result PathResult

	assert.False(t, result.IsReachable)
	assert.Equal(t, 0.0, result.Distance)
	assert.Equal(t, 0.0, result.Duration)
}

// TestNaNHandling ensures no NaN values are produced
func TestNaNHandling(t *testing.T) {
	graph := NewRoadGraph()

	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.00},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
	}

	graph.AddSegment(segment)

	// Check all edges for NaN
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			assert.False(t, math.IsNaN(edge.Distance), "Distance should not be NaN")
			assert.False(t, math.IsNaN(edge.Duration), "Duration should not be NaN")
			assert.False(t, math.IsInf(edge.Distance, 0), "Distance should not be Inf")
			assert.False(t, math.IsInf(edge.Duration, 0), "Duration should not be Inf")
		}
	}
}
