package pmtiles

import (
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMVTParser(t *testing.T) {
	parser := NewMVTParser("transportation")

	assert.NotNil(t, parser)
	assert.Equal(t, "transportation", parser.roadLayerName)
}

func TestNewMVTParser_CustomLayer(t *testing.T) {
	parser := NewMVTParser("roads")

	assert.Equal(t, "roads", parser.roadLayerName)
}

func TestMVTParser_getStringProperty(t *testing.T) {
	parser := NewMVTParser("transportation")

	tests := []struct {
		name       string
		properties map[string]any
		keys       []string
		expected   string
	}{
		{
			name: "first key exists",
			properties: map[string]any{
				"class": "primary",
				"type":  "road",
			},
			keys:     []string{"class", "type"},
			expected: "primary",
		},
		{
			name: "second key exists",
			properties: map[string]any{
				"type": "secondary",
			},
			keys:     []string{"class", "type"},
			expected: "secondary",
		},
		{
			name: "no key exists",
			properties: map[string]any{
				"other": "value",
			},
			keys:     []string{"class", "type"},
			expected: "",
		},
		{
			name:       "empty properties",
			properties: map[string]any{},
			keys:       []string{"class"},
			expected:   "",
		},
		{
			name: "non-string value",
			properties: map[string]any{
				"class": 123, // int, not string
			},
			keys:     []string{"class"},
			expected: "",
		},
		{
			name: "nil value",
			properties: map[string]any{
				"class": nil,
			},
			keys:     []string{"class"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feature := &geojson.Feature{
				Properties: tt.properties,
			}
			result := parser.getStringProperty(feature, tt.keys...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMVTParser_getBoolProperty(t *testing.T) {
	parser := NewMVTParser("transportation")

	tests := []struct {
		name       string
		properties map[string]any
		key        string
		expected   bool
	}{
		{
			name:       "bool true",
			properties: map[string]any{"oneway": true},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "bool false",
			properties: map[string]any{"oneway": false},
			key:        "oneway",
			expected:   false,
		},
		{
			name:       "int 1",
			properties: map[string]any{"oneway": 1},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "int 0",
			properties: map[string]any{"oneway": 0},
			key:        "oneway",
			expected:   false,
		},
		{
			name:       "int64 1",
			properties: map[string]any{"oneway": int64(1)},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "float64 1",
			properties: map[string]any{"oneway": float64(1.0)},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "float64 0",
			properties: map[string]any{"oneway": float64(0.0)},
			key:        "oneway",
			expected:   false,
		},
		{
			name:       "string yes",
			properties: map[string]any{"oneway": "yes"},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "string true",
			properties: map[string]any{"oneway": "true"},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "string 1",
			properties: map[string]any{"oneway": "1"},
			key:        "oneway",
			expected:   true,
		},
		{
			name:       "string no",
			properties: map[string]any{"oneway": "no"},
			key:        "oneway",
			expected:   false,
		},
		{
			name:       "string false",
			properties: map[string]any{"oneway": "false"},
			key:        "oneway",
			expected:   false,
		},
		{
			name:       "key not exists",
			properties: map[string]any{},
			key:        "oneway",
			expected:   false,
		},
		{
			name:       "unsupported type",
			properties: map[string]any{"oneway": []string{"yes"}},
			key:        "oneway",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feature := &geojson.Feature{
				Properties: tt.properties,
			}
			result := parser.getBoolProperty(feature, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMVTParser_getSpeedForRoadType(t *testing.T) {
	parser := NewMVTParser("transportation")

	tests := []struct {
		highway  string
		expected float64
	}{
		{"motorway", 110.0},
		{"motorway_link", 80.0},
		{"trunk", 80.0},
		{"trunk_link", 60.0},
		{"primary", 60.0},
		{"primary_link", 50.0},
		{"secondary", 50.0},
		{"secondary_link", 40.0},
		{"tertiary", 40.0},
		{"tertiary_link", 30.0},
		{"residential", 30.0},
		{"living_street", 20.0},
		{"service", 20.0},
		{"unclassified", 30.0},
		{"road", 30.0},
		{"unknown", 30.0},    // default
		{"", 30.0},           // empty string
		{"pedestrian", 30.0}, // not in map
		{"cycleway", 30.0},   // not in map
		{"footway", 30.0},    // not in map
	}

	for _, tt := range tests {
		t.Run(tt.highway, func(t *testing.T) {
			result := parser.getSpeedForRoadType(tt.highway)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMVTParser_parseFeatureID(t *testing.T) {
	parser := NewMVTParser("transportation")

	tests := []struct {
		name     string
		id       any
		expected uint64
	}{
		{
			name:     "nil",
			id:       nil,
			expected: 0,
		},
		{
			name:     "float64",
			id:       float64(12345),
			expected: 12345,
		},
		{
			name:     "int",
			id:       int(54321),
			expected: 54321,
		},
		{
			name:     "int64",
			id:       int64(99999),
			expected: 99999,
		},
		{
			name:     "string",
			id:       "abc",
			expected: 0,
		},
		{
			name:     "negative float64",
			id:       float64(-1),
			expected: 0, // Go float64 to uint64 conversion of negative values results in 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseFeatureID(tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMVTParser_extractGeometry_LineString(t *testing.T) {
	parser := NewMVTParser("transportation")

	feature := &geojson.Feature{
		Geometry: orb.LineString{
			{121.50, 25.00},
			{121.51, 25.01},
			{121.52, 25.02},
		},
	}

	points, ok := parser.extractGeometry(feature)

	assert.True(t, ok)
	assert.Len(t, points, 3)
	assert.Equal(t, orb.Point{121.50, 25.00}, points[0])
	assert.Equal(t, orb.Point{121.51, 25.01}, points[1])
	assert.Equal(t, orb.Point{121.52, 25.02}, points[2])
}

func TestMVTParser_extractGeometry_MultiLineString(t *testing.T) {
	parser := NewMVTParser("transportation")

	feature := &geojson.Feature{
		Geometry: orb.MultiLineString{
			{
				{121.50, 25.00},
				{121.51, 25.01},
			},
			{
				{121.52, 25.02},
				{121.53, 25.03},
			},
		},
	}

	points, ok := parser.extractGeometry(feature)

	assert.True(t, ok)
	assert.Len(t, points, 4)
	assert.Equal(t, orb.Point{121.50, 25.00}, points[0])
	assert.Equal(t, orb.Point{121.53, 25.03}, points[3])
}

func TestMVTParser_extractGeometry_Point(t *testing.T) {
	parser := NewMVTParser("transportation")

	feature := &geojson.Feature{
		Geometry: orb.Point{121.50, 25.00},
	}

	points, ok := parser.extractGeometry(feature)

	assert.False(t, ok)
	assert.Nil(t, points)
}

func TestMVTParser_extractGeometry_Polygon(t *testing.T) {
	parser := NewMVTParser("transportation")

	feature := &geojson.Feature{
		Geometry: orb.Polygon{
			{
				{121.50, 25.00},
				{121.51, 25.00},
				{121.51, 25.01},
				{121.50, 25.00},
			},
		},
	}

	points, ok := parser.extractGeometry(feature)

	assert.False(t, ok)
	assert.Nil(t, points)
}

func TestMVTParser_extractGeometry_SinglePoint(t *testing.T) {
	parser := NewMVTParser("transportation")

	// LineString with only 1 point should be rejected
	feature := &geojson.Feature{
		Geometry: orb.LineString{
			{121.50, 25.00},
		},
	}

	points, ok := parser.extractGeometry(feature)

	assert.False(t, ok)
	assert.Nil(t, points)
}

func TestMVTParser_extractRoadSegment(t *testing.T) {
	parser := NewMVTParser("transportation")

	feature := &geojson.Feature{
		ID: float64(12345),
		Geometry: orb.LineString{
			{121.50, 25.00},
			{121.51, 25.01},
		},
		Properties: map[string]any{
			"class":  "primary",
			"name":   "Main Street",
			"oneway": true,
		},
	}

	segment, ok := parser.extractRoadSegment(feature)

	assert.True(t, ok)
	assert.Equal(t, uint64(12345), segment.FeatureID)
	assert.Equal(t, "primary", segment.Highway)
	assert.Equal(t, "Main Street", segment.Name)
	assert.True(t, segment.OneWay)
	assert.Equal(t, 60.0, segment.MaxSpeed) // primary = 60 km/h
	assert.Len(t, segment.Points, 2)
}

func TestMVTParser_extractRoadSegment_InvalidGeometry(t *testing.T) {
	parser := NewMVTParser("transportation")

	// Point geometry should not be extractable
	feature := &geojson.Feature{
		Geometry: orb.Point{121.50, 25.00},
	}

	_, ok := parser.extractRoadSegment(feature)

	assert.False(t, ok)
}

func TestMVTParser_ParseTile_InvalidData(t *testing.T) {
	parser := NewMVTParser("transportation")

	// Invalid data (not MVT or gzipped MVT)
	invalidData := []byte("not valid mvt data")

	segments, err := parser.ParseTile(invalidData, TileForTest())

	assert.Error(t, err)
	assert.Nil(t, segments)
}

func TestMVTParser_ParseTile_EmptyData(t *testing.T) {
	parser := NewMVTParser("transportation")

	segments, err := parser.ParseTile([]byte{}, TileForTest())

	// Empty data is handled gracefully by the MVT library
	assert.NoError(t, err)
	assert.Empty(t, segments)
}

func TestMVTParser_ParseTile_LayerNotFound(t *testing.T) {
	parser := NewMVTParser("nonexistent_layer")

	// Create valid MVT data with a different layer name
	layers := mvt.Layers{
		&mvt.Layer{
			Name: "transportation",
			Features: []*geojson.Feature{
				{
					Geometry: orb.LineString{
						{121.50, 25.00},
						{121.51, 25.01},
					},
					Properties: map[string]any{
						"class": "primary",
					},
				},
			},
		},
	}

	data, err := mvt.Marshal(layers)
	require.NoError(t, err)

	// Parse with parser looking for nonexistent layer
	segments, err := parser.ParseTile(data, TileForTest())

	// Should return empty segments, not error
	assert.NoError(t, err)
	assert.Empty(t, segments)
}

func TestMVTParser_ParseTile_ValidData(t *testing.T) {
	parser := NewMVTParser("transportation")

	// Create valid MVT data
	layers := mvt.Layers{
		&mvt.Layer{
			Name: "transportation",
			Features: []*geojson.Feature{
				{
					ID: float64(123),
					Geometry: orb.LineString{
						{121.50, 25.00},
						{121.51, 25.01},
					},
					Properties: map[string]any{
						"class":  "primary",
						"name":   "Test Road",
						"oneway": true,
					},
				},
				{
					ID: float64(456),
					Geometry: orb.LineString{
						{121.52, 25.02},
						{121.53, 25.03},
						{121.54, 25.04},
					},
					Properties: map[string]any{
						"class": "secondary",
					},
				},
			},
		},
	}

	data, err := mvt.Marshal(layers)
	require.NoError(t, err)

	segments, err := parser.ParseTile(data, TileForTest())

	require.NoError(t, err)
	require.Len(t, segments, 2)

	// Verify first segment
	assert.Equal(t, uint64(123), segments[0].FeatureID)
	assert.Equal(t, "primary", segments[0].Highway)
	assert.Equal(t, "Test Road", segments[0].Name)
	assert.True(t, segments[0].OneWay)
	assert.Len(t, segments[0].Points, 2)

	// Verify second segment
	assert.Equal(t, uint64(456), segments[1].FeatureID)
	assert.Equal(t, "secondary", segments[1].Highway)
	assert.Len(t, segments[1].Points, 3)
}

func TestMVTParser_ParseTile_MultipleGeometryTypes(t *testing.T) {
	parser := NewMVTParser("transportation")

	// Create MVT data with different geometry types
	layers := mvt.Layers{
		&mvt.Layer{
			Name: "transportation",
			Features: []*geojson.Feature{
				{
					// LineString - should be included
					Geometry: orb.LineString{
						{121.50, 25.00},
						{121.51, 25.01},
					},
					Properties: map[string]any{"class": "primary"},
				},
				{
					// Point - should be skipped
					Geometry:   orb.Point{121.52, 25.02},
					Properties: map[string]any{"class": "bus_stop"},
				},
				{
					// MultiLineString - should be included
					Geometry: orb.MultiLineString{
						{{121.53, 25.03}, {121.54, 25.04}},
						{{121.55, 25.05}, {121.56, 25.06}},
					},
					Properties: map[string]any{"class": "secondary"},
				},
				{
					// Polygon - should be skipped
					Geometry: orb.Polygon{
						{{121.57, 25.07}, {121.58, 25.07}, {121.58, 25.08}, {121.57, 25.07}},
					},
					Properties: map[string]any{"class": "parking"},
				},
			},
		},
	}

	data, err := mvt.Marshal(layers)
	require.NoError(t, err)

	segments, err := parser.ParseTile(data, TileForTest())

	require.NoError(t, err)
	// Only LineString and MultiLineString should be extracted
	assert.Len(t, segments, 2)
	assert.Equal(t, "primary", segments[0].Highway)
	assert.Equal(t, "secondary", segments[1].Highway)
}

// TileForTest creates a simple maptile.Tile for testing
func TileForTest() maptile.Tile {
	return maptile.Tile{X: 0, Y: 0, Z: 14}
}

func TestRoadSegment_Fields(t *testing.T) {
	segment := RoadSegment{
		Points: []orb.Point{
			{121.50, 25.00},
			{121.51, 25.01},
		},
		Highway:   "residential",
		MaxSpeed:  30.0,
		OneWay:    false,
		Name:      "Test Road",
		FeatureID: 12345,
	}

	assert.Len(t, segment.Points, 2)
	assert.Equal(t, "residential", segment.Highway)
	assert.Equal(t, 30.0, segment.MaxSpeed)
	assert.False(t, segment.OneWay)
	assert.Equal(t, "Test Road", segment.Name)
	assert.Equal(t, uint64(12345), segment.FeatureID)
}
