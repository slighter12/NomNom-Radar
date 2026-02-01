package pmtiles

import (
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/pkg/errors"
)

// RoadSegment represents a road segment extracted from MVT data
type RoadSegment struct {
	Points    []orb.Point
	Highway   string  // road type (e.g., "primary", "secondary", "residential")
	MaxSpeed  float64 // max speed in km/h (0 if unknown)
	OneWay    bool
	Name      string
	FeatureID uint64
}

// MVTParser handles parsing of MVT tiles to extract road network data
type MVTParser struct {
	roadLayerName string
}

// NewMVTParser creates a new MVT parser
func NewMVTParser(roadLayerName string) *MVTParser {
	return &MVTParser{
		roadLayerName: roadLayerName,
	}
}

// ParseTile parses MVT tile data and extracts road segments
func (p *MVTParser) ParseTile(data []byte, tile maptile.Tile) ([]RoadSegment, error) {
	// Try to decode as gzipped first, then as regular MVT
	layers, err := mvt.UnmarshalGzipped(data)
	if err != nil {
		// Try regular unmarshal
		layers, err = mvt.Unmarshal(data)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	// Find the road layer
	var roadLayer *mvt.Layer
	for _, layer := range layers {
		if layer.Name == p.roadLayerName {
			roadLayer = layer

			break
		}
	}

	if roadLayer == nil {
		// Layer not found - return empty segments
		return []RoadSegment{}, nil
	}

	// Project layer to WGS84 coordinates
	roadLayer.ProjectToWGS84(tile)

	// Extract road segments from features
	segments := make([]RoadSegment, 0)

	for _, feature := range roadLayer.Features {
		segment, ok := p.extractRoadSegment(feature)
		if ok {
			segments = append(segments, segment)
		}
	}

	return segments, nil
}

// extractRoadSegment extracts a road segment from a GeoJSON feature
func (p *MVTParser) extractRoadSegment(feature *geojson.Feature) (RoadSegment, bool) {
	var segment RoadSegment

	points, ok := p.extractGeometry(feature)
	if !ok {
		return segment, false
	}
	segment.Points = points

	segment.FeatureID = p.parseFeatureID(feature.ID)
	segment.Highway = p.getStringProperty(feature, "class", "highway", "type")
	segment.Name = p.getStringProperty(feature, "name", "")
	segment.OneWay = p.getBoolProperty(feature, "oneway")
	segment.MaxSpeed = p.getSpeedForRoadType(segment.Highway)

	return segment, true
}

func (p *MVTParser) extractGeometry(feature *geojson.Feature) ([]orb.Point, bool) {
	var points []orb.Point

	// Get geometry - we only care about LineStrings
	switch geom := feature.Geometry.(type) {
	case orb.LineString:
		points = make([]orb.Point, 0, len(geom))
		points = append(points, geom...)
	case orb.MultiLineString:
		// For MultiLineString, concatenate all parts
		for _, ls := range geom {
			points = append(points, ls...)
		}
	default:
		// Not a line feature, skip
		return nil, false
	}

	if len(points) < 2 {
		return nil, false
	}

	return points, true
}

func (p *MVTParser) parseFeatureID(id any) uint64 {
	if id == nil {
		return 0
	}

	switch fid := id.(type) {
	case float64:
		return uint64(fid)
	case int:
		return uint64(fid)
	case int64:
		return uint64(fid)
	default:
		return 0
	}
}

// getStringProperty gets a string property from feature properties
func (p *MVTParser) getStringProperty(feature *geojson.Feature, keys ...string) string {
	for _, key := range keys {
		if val, ok := feature.Properties[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}

	return ""
}

// getBoolProperty gets a boolean property from feature properties
func (p *MVTParser) getBoolProperty(feature *geojson.Feature, key string) bool {
	if val, ok := feature.Properties[key]; ok {
		switch value := val.(type) {
		case bool:
			return value
		case int:
			return value != 0
		case int64:
			return value != 0
		case float64:
			return value != 0
		case string:
			return value == "yes" || value == "true" || value == "1"
		}
	}

	return false
}

// getSpeedForRoadType returns default speed in km/h based on road type
func (p *MVTParser) getSpeedForRoadType(highway string) float64 {
	speeds := map[string]float64{
		"motorway":       110.0,
		"motorway_link":  80.0,
		"trunk":          80.0,
		"trunk_link":     60.0,
		"primary":        60.0,
		"primary_link":   50.0,
		"secondary":      50.0,
		"secondary_link": 40.0,
		"tertiary":       40.0,
		"tertiary_link":  30.0,
		"residential":    30.0,
		"living_street":  20.0,
		"service":        20.0,
		"unclassified":   30.0,
		"road":           30.0,
	}

	if speed, ok := speeds[highway]; ok {
		return speed
	}

	return 30.0 // Default speed
}
