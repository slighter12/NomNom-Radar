package ch

import (
	"math"
	"sort"

	"radar/internal/infra/routing/loader"
)

// SpatialIndex provides efficient nearest-neighbor lookup for coordinates
type SpatialIndex interface {
	// Nearest finds the nearest vertex to the given coordinate
	// Returns the vertex index and true if found, or -1 and false if the index is empty
	Nearest(lat, lng float64) (vertexIdx int, ok bool)

	// Build constructs the spatial index from vertices
	Build(vertices []loader.Vertex)

	// Size returns the number of vertices in the index
	Size() int
}

// GridIndex implements a simple grid-based spatial index
// This is a lightweight alternative to KD-tree with no external dependencies
type GridIndex struct {
	vertices    []loader.Vertex
	grid        map[gridKey][]int // maps grid cell to vertex indices
	cellSizeLat float64           // grid cell size in latitude degrees
	cellSizeLng float64           // grid cell size in longitude degrees
	minLat      float64
	maxLat      float64
	minLng      float64
	maxLng      float64
}

type gridKey struct {
	latCell int
	lngCell int
}

// NewGridIndex creates a new grid-based spatial index
// cellSizeKm determines the grid cell size (smaller = more cells, faster lookup but more memory)
func NewGridIndex(cellSizeKm float64) *GridIndex {
	// Convert km to approximate degrees
	// 1 degree latitude ≈ 111 km
	// 1 degree longitude ≈ 111 km * cos(lat) at the equator
	// For Taiwan (lat ~23-25), cos(24°) ≈ 0.91, so 1° lng ≈ 101 km
	cellSizeLat := cellSizeKm / 111.0
	cellSizeLng := cellSizeKm / 101.0 // Approximate for Taiwan's latitude

	return &GridIndex{
		grid:        make(map[gridKey][]int),
		cellSizeLat: cellSizeLat,
		cellSizeLng: cellSizeLng,
	}
}

// Build constructs the grid index from vertices
func (g *GridIndex) Build(vertices []loader.Vertex) {
	g.vertices = vertices
	g.grid = make(map[gridKey][]int)

	if len(vertices) == 0 {
		return
	}

	// Find bounding box
	g.minLat, g.maxLat = vertices[0].Lat, vertices[0].Lat
	g.minLng, g.maxLng = vertices[0].Lng, vertices[0].Lng

	for _, vertex := range vertices {
		if vertex.Lat < g.minLat {
			g.minLat = vertex.Lat
		}
		if vertex.Lat > g.maxLat {
			g.maxLat = vertex.Lat
		}
		if vertex.Lng < g.minLng {
			g.minLng = vertex.Lng
		}
		if vertex.Lng > g.maxLng {
			g.maxLng = vertex.Lng
		}
	}

	// Insert vertices into grid cells
	for idx, vertex := range vertices {
		key := g.getGridKey(vertex.Lat, vertex.Lng)
		g.grid[key] = append(g.grid[key], idx)
	}
}

// Nearest finds the nearest vertex to the given coordinate
func (g *GridIndex) Nearest(lat, lng float64) (vertexIdx int, ok bool) {
	if len(g.vertices) == 0 {
		return -1, false
	}

	key := g.getGridKey(lat, lng)

	// Search in expanding rings of grid cells
	bestIdx := -1
	bestDistSq := math.MaxFloat64

	// Start with the center cell and expand outward
	for ring := 0; ring <= g.maxSearchRing(); ring++ {
		found := g.searchRing(lat, lng, key, ring, &bestIdx, &bestDistSq)

		// If we found a result and the next ring can't possibly contain a closer point,
		// we can stop early
		if found && ring > 0 {
			minPossibleDistSq := g.minDistanceToRingSq(center(key), ring+1)
			if minPossibleDistSq >= bestDistSq {
				break
			}
		}
	}

	if bestIdx < 0 {
		return -1, false
	}

	return bestIdx, true
}

// Size returns the number of vertices in the index
func (g *GridIndex) Size() int {
	return len(g.vertices)
}

func (g *GridIndex) getGridKey(lat, lng float64) gridKey {
	latCell := int(math.Floor((lat - g.minLat) / g.cellSizeLat))
	lngCell := int(math.Floor((lng - g.minLng) / g.cellSizeLng))

	return gridKey{latCell: latCell, lngCell: lngCell}
}

func (g *GridIndex) searchRing(lat, lng float64, centerKey gridKey, ring int, bestIdx *int, bestDistSq *float64) bool {
	found := false

	// For ring 0, just search the center cell
	if ring == 0 {
		return g.searchCell(lat, lng, centerKey, bestIdx, bestDistSq)
	}

	// For ring > 0, search the perimeter of the ring
	for dLat := -ring; dLat <= ring; dLat++ {
		for dLng := -ring; dLng <= ring; dLng++ {
			// Only process cells on the perimeter of this ring
			if abs(dLat) != ring && abs(dLng) != ring {
				continue
			}

			cellKey := gridKey{
				latCell: centerKey.latCell + dLat,
				lngCell: centerKey.lngCell + dLng,
			}

			if g.searchCell(lat, lng, cellKey, bestIdx, bestDistSq) {
				found = true
			}
		}
	}

	return found
}

func (g *GridIndex) searchCell(lat, lng float64, key gridKey, bestIdx *int, bestDistSq *float64) bool {
	indices, exists := g.grid[key]
	if !exists {
		return false
	}

	found := false
	for _, idx := range indices {
		vertex := g.vertices[idx]
		distSq := squaredDistance(lat, lng, vertex.Lat, vertex.Lng)
		if distSq < *bestDistSq {
			*bestDistSq = distSq
			*bestIdx = idx
			found = true
		}
	}

	return found
}

func (g *GridIndex) maxSearchRing() int {
	// Calculate maximum ring needed to cover the entire bounding box
	latCells := int(math.Ceil((g.maxLat - g.minLat) / g.cellSizeLat))
	lngCells := int(math.Ceil((g.maxLng - g.minLng) / g.cellSizeLng))

	return max(latCells, lngCells) + 1
}

// center returns a dummy representation for the grid key (used for distance calculations)
func center(_ gridKey) gridKey {
	return gridKey{}
}

func (g *GridIndex) minDistanceToRingSq(_ gridKey, ring int) float64 {
	// Calculate the minimum possible squared distance to any point in the ring
	// Distance to the inner edge of the ring
	latDist := float64(ring-1) * g.cellSizeLat
	lngDist := float64(ring-1) * g.cellSizeLng

	return latDist*latDist + lngDist*lngDist
}

// squaredDistance calculates the squared Euclidean distance in degrees
// For nearest-neighbor comparison, we don't need the actual distance
func squaredDistance(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := lat2 - lat1
	dLng := lng2 - lng1

	return dLat*dLat + dLng*dLng
}

func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}

// GetVertex returns the vertex at the given index
func (g *GridIndex) GetVertex(idx int) *loader.Vertex {
	if idx < 0 || idx >= len(g.vertices) {
		return nil
	}

	return &g.vertices[idx]
}

// GetVertices returns all vertices
func (g *GridIndex) GetVertices() []loader.Vertex {
	return g.vertices
}

// distIdx holds a vertex index and its squared distance for sorting.
type distIdx struct {
	idx    int
	distSq float64
}

// NearestK finds the k nearest vertices to the given coordinate using ring expansion.
// Returns vertex indices sorted by distance (closest first).
// Time complexity: O(k log k) in typical cases, O(N) worst case for very sparse data.
func (g *GridIndex) NearestK(lat, lng float64, count int) []int {
	if len(g.vertices) == 0 || count <= 0 {
		return nil
	}

	key := g.getGridKey(lat, lng)
	var candidates []distIdx

	// Expand rings until we have enough candidates and next ring can't have closer points
	for ring := 0; ring <= g.maxSearchRing(); ring++ {
		// Collect vertices from this ring
		ringCandidates := g.collectRingVertices(lat, lng, key, ring)
		candidates = append(candidates, ringCandidates...)

		// Early termination: if we have enough candidates and next ring is too far
		if len(candidates) >= count && ring > 0 {
			// Sort current candidates to find k-th distance
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].distSq < candidates[j].distSq
			})

			// Check if next ring's minimum distance is greater than k-th candidate
			kthDistSq := candidates[min(count-1, len(candidates)-1)].distSq
			minNextRingDistSq := g.minDistanceToRingSq(key, ring+1)

			if minNextRingDistSq >= kthDistSq {
				break
			}
		}
	}

	// Final sort and return top count
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distSq < candidates[j].distSq
	})

	result := make([]int, 0, min(count, len(candidates)))
	for idx := 0; idx < len(candidates) && idx < count; idx++ {
		result = append(result, candidates[idx].idx)
	}

	return result
}

// collectRingVertices collects all vertices from a specific ring around the center cell.
func (g *GridIndex) collectRingVertices(lat, lng float64, centerKey gridKey, ring int) []distIdx {
	var results []distIdx

	if ring == 0 {
		// Just the center cell
		if indices, exists := g.grid[centerKey]; exists {
			for _, idx := range indices {
				vertex := g.vertices[idx]
				distSq := squaredDistance(lat, lng, vertex.Lat, vertex.Lng)
				results = append(results, distIdx{idx: idx, distSq: distSq})
			}
		}

		return results
	}

	// Perimeter of the ring
	for dLat := -ring; dLat <= ring; dLat++ {
		for dLng := -ring; dLng <= ring; dLng++ {
			// Only process cells on the perimeter
			if abs(dLat) != ring && abs(dLng) != ring {
				continue
			}

			cellKey := gridKey{
				latCell: centerKey.latCell + dLat,
				lngCell: centerKey.lngCell + dLng,
			}

			if indices, exists := g.grid[cellKey]; exists {
				for _, idx := range indices {
					vertex := g.vertices[idx]
					distSq := squaredDistance(lat, lng, vertex.Lat, vertex.Lng)
					results = append(results, distIdx{idx: idx, distSq: distSq})
				}
			}
		}
	}

	return results
}
