package usecase

import (
	"context"
	"time"
)

// Coordinate represents a geographic coordinate
type Coordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// RouteResult represents the result of a routing calculation
type RouteResult struct {
	Source      Coordinate `json:"source"`
	Target      Coordinate `json:"target"`
	DistanceKm  float64    `json:"distance_km"`  // Road network distance in kilometers
	DurationMin float64    `json:"duration_min"` // Estimated travel time in minutes
	IsReachable bool       `json:"is_reachable"` // Whether target is reachable via road network
}

// OneToManyResult represents the result of a one-to-many routing query
type OneToManyResult struct {
	Source   Coordinate    `json:"source"`
	Targets  []Coordinate  `json:"targets"`
	Results  []RouteResult `json:"results"`
	Duration time.Duration `json:"duration"` // Total query execution time
}

// NodeID represents a road network node identifier
type NodeID int

// NodeInfo represents information about a road network node
type NodeInfo struct {
	ID       NodeID     `json:"id"`
	Location Coordinate `json:"location"`
}

// RoutingUsecase defines the interface for routing engine use cases
type RoutingUsecase interface {
	// OneToMany calculates routes from one source coordinate to multiple target coordinates
	// Returns results for all targets, with unreachable targets marked accordingly
	OneToMany(ctx context.Context, source Coordinate, targets []Coordinate) (*OneToManyResult, error)

	// FindNearestNode finds the nearest road network node to a given GPS coordinate
	// Returns the node information and whether it was within the maximum snap distance
	FindNearestNode(ctx context.Context, coord Coordinate) (*NodeInfo, bool, error)

	// CalculateDistance calculates the road network distance between two coordinates
	// Returns RouteResult with distance, duration, and reachability information
	CalculateDistance(ctx context.Context, source, target Coordinate) (*RouteResult, error)

	// IsReady returns whether the routing engine is loaded and ready for queries
	IsReady() bool
}
