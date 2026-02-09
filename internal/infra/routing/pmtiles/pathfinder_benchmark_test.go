package pmtiles

import (
	"testing"

	"github.com/paulmach/orb"
)

func BenchmarkRoadGraph_AddSegment(b *testing.B) {
	segment := &RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.01},
			{121.52, 25.02},
			{121.53, 25.03},
			{121.54, 25.04},
		},
		Highway:  "primary",
		MaxSpeed: 50.0,
		OneWay:   false,
	}

	for b.Loop() {
		graph := NewRoadGraph()
		graph.AddSegment(segment)
	}
}

func BenchmarkRoadGraph_FindNearestNode(b *testing.B) {
	graph := NewRoadGraph()

	// Create a grid of nodes
	for lat := 25.0; lat <= 25.1; lat += 0.001 {
		for lng := 121.5; lng <= 121.6; lng += 0.001 {
			segment := &RoadSegment{
				Points: []orb.Point{
					{lng, lat},
					{lng + 0.001, lat},
				},
				Highway:  "residential",
				MaxSpeed: 30.0,
			}
			graph.AddSegment(segment)
		}
	}

	queryPoint := orb.Point{121.55, 25.05}

	b.ResetTimer()
	for b.Loop() {
		graph.FindNearestNode(queryPoint)
	}
}

func BenchmarkPathfinder_ShortestPath(b *testing.B) {
	graph := NewRoadGraph()

	// Create a chain of nodes
	var prevPoint orb.Point
	for i := range 100 {
		point := orb.Point{121.5 + float64(i)*0.001, 25.0}
		if i > 0 {
			segment := &RoadSegment{
				Points:   []orb.Point{prevPoint, point},
				Highway:  "primary",
				MaxSpeed: 50.0,
			}
			graph.AddSegment(segment)
		}
		prevPoint = point
	}

	pf := NewPathfinder(graph)

	startKey := pointKey(orb.Point{121.5, 25.0})
	endKey := pointKey(orb.Point{121.5 + 99*0.001, 25.0})
	startNode := graph.pointMap[startKey]
	endNode := graph.pointMap[endKey]

	b.ResetTimer()
	for b.Loop() {
		pf.ShortestPath(startNode, endNode)
	}
}

func BenchmarkPathfinder_ShortestPathToMany(b *testing.B) {
	graph := NewRoadGraph()

	// Create a grid network
	for row := range 10 {
		for col := range 10 {
			baseLng := 121.5 + float64(col)*0.01
			baseLat := 25.0 + float64(row)*0.01

			// Horizontal edge
			if col < 9 {
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
			if row < 9 {
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

	pf := NewPathfinder(graph)

	startNode := graph.pointMap[pointKey(orb.Point{121.5, 25.0})]
	targetNodes := make([]NodeID, 10)
	for i := range targetNodes {
		key := pointKey(orb.Point{121.5 + float64(i)*0.01, 25.0 + float64(i)*0.01})
		targetNodes[i] = graph.pointMap[key]
	}

	b.ResetTimer()
	for b.Loop() {
		pf.ShortestPathToMany(startNode, targetNodes)
	}
}

func BenchmarkHaversineDistance(b *testing.B) {
	p1 := orb.Point{121.5654, 25.0330}
	p2 := orb.Point{121.5170, 25.0478}

	for b.Loop() {
		haversineDistance(p1, p2)
	}
}

func BenchmarkPointKey(b *testing.B) {
	point := orb.Point{121.56543, 25.03301}

	for b.Loop() {
		pointKey(point)
	}
}

func BenchmarkFormatFloat(b *testing.B) {
	value := 121.56543

	for b.Loop() {
		formatFloat(value)
	}
}

func BenchmarkRoadGraph_getOrCreateNode(b *testing.B) {
	graph := NewRoadGraph()
	points := make([]orb.Point, 100)
	for i := range points {
		points[i] = orb.Point{121.5 + float64(i)*0.001, 25.0 + float64(i)*0.001}
	}

	b.ResetTimer()
	for b.Loop() {
		for _, p := range points {
			graph.getOrCreateNode(p)
		}
	}
}
