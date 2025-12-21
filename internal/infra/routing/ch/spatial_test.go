package ch

import (
	"testing"

	"radar/internal/infra/routing/loader"

	"github.com/stretchr/testify/assert"
)

func TestGridIndex_Build(t *testing.T) {
	vertices := []loader.Vertex{
		{ID: 0, Lat: 25.0330, Lng: 121.5654}, // Taipei Station
		{ID: 1, Lat: 25.0478, Lng: 121.5170}, // Taipei Main Station
		{ID: 2, Lat: 23.5711, Lng: 119.5793}, // Penghu
	}

	index := NewGridIndex(1.0)
	index.Build(vertices)

	assert.Equal(t, 3, index.Size())
}

func TestGridIndex_Nearest(t *testing.T) {
	vertices := []loader.Vertex{
		{ID: 0, Lat: 25.0330, Lng: 121.5654}, // Taipei Station
		{ID: 1, Lat: 25.0478, Lng: 121.5170}, // Taipei Main Station
		{ID: 2, Lat: 24.1500, Lng: 120.6800}, // Taichung
	}

	index := NewGridIndex(1.0)
	index.Build(vertices)

	// Query near Taipei Station - should return vertex 0
	idx, ok := index.Nearest(25.0340, 121.5660)
	assert.True(t, ok)
	assert.Equal(t, 0, idx)

	// Query near Taipei Main Station - should return vertex 1
	idx, ok = index.Nearest(25.0480, 121.5175)
	assert.True(t, ok)
	assert.Equal(t, 1, idx)

	// Query near Taichung - should return vertex 2
	idx, ok = index.Nearest(24.1490, 120.6810)
	assert.True(t, ok)
	assert.Equal(t, 2, idx)
}

func TestGridIndex_Nearest_Empty(t *testing.T) {
	index := NewGridIndex(1.0)
	index.Build([]loader.Vertex{})

	idx, ok := index.Nearest(25.0, 121.0)
	assert.False(t, ok)
	assert.Equal(t, -1, idx)
}

func TestGridIndex_GetVertex(t *testing.T) {
	vertices := []loader.Vertex{
		{ID: 0, Lat: 25.0330, Lng: 121.5654},
		{ID: 1, Lat: 25.0478, Lng: 121.5170},
	}

	index := NewGridIndex(1.0)
	index.Build(vertices)

	v := index.GetVertex(0)
	assert.NotNil(t, v)
	assert.Equal(t, int64(0), v.ID)
	assert.InDelta(t, 25.0330, v.Lat, 0.0001)

	// Out of bounds
	v = index.GetVertex(-1)
	assert.Nil(t, v)

	v = index.GetVertex(10)
	assert.Nil(t, v)
}

func TestGridIndex_LargeDataset(t *testing.T) {
	// Create a grid of vertices
	var vertices []loader.Vertex
	id := int64(0)
	for lat := 22.0; lat <= 25.5; lat += 0.1 {
		for lng := 120.0; lng <= 122.0; lng += 0.1 {
			vertices = append(vertices, loader.Vertex{
				ID:  id,
				Lat: lat,
				Lng: lng,
			})
			id++
		}
	}

	index := NewGridIndex(0.5) // Smaller grid cells for precision
	index.Build(vertices)

	assert.Equal(t, len(vertices), index.Size())

	// Query should find the closest vertex
	idx, ok := index.Nearest(23.5, 121.0)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, idx, 0)

	// Verify the found vertex is actually close
	v := index.GetVertex(idx)
	assert.InDelta(t, 23.5, v.Lat, 0.1)
	assert.InDelta(t, 121.0, v.Lng, 0.1)
}
