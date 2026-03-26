package loader

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// Vertex represents a node in the road network graph
type Vertex struct {
	ID         int64   // Internal vertex ID (slice index)
	Lat        float64 // Latitude
	Lng        float64 // Longitude
	OrderPos   int64   // CH order position (contraction order)
	Importance int64   // CH importance value
}

// Edge represents a connection between two vertices
type Edge struct {
	From   int64   // Source vertex ID
	To     int64   // Target vertex ID
	Weight float64 // Edge weight (can be distance in meters or time in seconds)
}

// Shortcut represents a CH shortcut edge
type Shortcut struct {
	From    int64   // Source vertex ID
	To      int64   // Target vertex ID
	Weight  float64 // Shortcut weight
	ViaNode int64   // Intermediate node this shortcut bypasses
}

// GraphData holds all loaded graph data
type GraphData struct {
	Vertices  []Vertex
	Edges     []Edge
	Shortcuts []Shortcut
}

// CSVLoader handles loading of routing data from CSV files
type CSVLoader struct {
	dataDir string
}

// NewCSVLoader creates a new CSV loader for the given data directory
func NewCSVLoader(dataDir string) *CSVLoader {
	return &CSVLoader{dataDir: dataDir}
}

// Load loads all graph data from CSV files
func (l *CSVLoader) Load() (*GraphData, error) {
	vertices, err := l.LoadVertices()
	if err != nil {
		return nil, err
	}

	edges, err := l.LoadEdges()
	if err != nil {
		return nil, err
	}

	shortcuts, err := l.LoadShortcuts()
	if err != nil {
		return nil, err
	}

	return &GraphData{
		Vertices:  vertices,
		Edges:     edges,
		Shortcuts: shortcuts,
	}, nil
}

// LoadVertices loads vertices from vertices.csv
// Expected CSV format: id,lat,lng,order_pos,importance
func (l *CSVLoader) LoadVertices() ([]Vertex, error) {
	path := filepath.Join(l.dataDir, "vertices.csv")
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("read %s header: %w", path, err)
	}

	var vertices []Vertex
	lineNum := 1 // Start at 1 because we skipped header

	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read %s line %d: %w", path, lineNum+1, readErr)
		}
		lineNum++

		if len(record) < 5 {
			return nil, fmt.Errorf("invalid vertices.csv format at line %d: expected 5 columns, got %d", lineNum, len(record))
		}

		vertex, parseErr := parseVertex(record, lineNum)
		if parseErr != nil {
			return nil, parseErr
		}

		vertices = append(vertices, vertex)
	}

	return vertices, nil
}

// LoadEdges loads edges from edges.csv
// Expected CSV format: from,to,weight
func (l *CSVLoader) LoadEdges() ([]Edge, error) {
	path := filepath.Join(l.dataDir, "edges.csv")
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("read %s header: %w", path, err)
	}

	var edges []Edge
	lineNum := 1

	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read %s line %d: %w", path, lineNum+1, readErr)
		}
		lineNum++

		if len(record) < 3 {
			return nil, fmt.Errorf("invalid edges.csv format at line %d: expected 3 columns, got %d", lineNum, len(record))
		}

		edge, parseErr := parseEdge(record, lineNum)
		if parseErr != nil {
			return nil, parseErr
		}

		edges = append(edges, edge)
	}

	return edges, nil
}

// LoadShortcuts loads shortcuts from shortcuts.csv
// Expected CSV format: from,to,weight,via_node
func (l *CSVLoader) LoadShortcuts() ([]Shortcut, error) {
	path := filepath.Join(l.dataDir, "shortcuts.csv")
	file, err := os.Open(path)
	if err != nil {
		// Shortcuts file is optional if no contraction was performed
		if os.IsNotExist(err) {
			return []Shortcut{}, nil
		}

		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("read %s header: %w", path, err)
	}

	var shortcuts []Shortcut
	lineNum := 1

	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read %s line %d: %w", path, lineNum+1, readErr)
		}
		lineNum++

		if len(record) < 4 {
			return nil, fmt.Errorf("invalid shortcuts.csv format at line %d: expected 4 columns, got %d", lineNum, len(record))
		}

		shortcut, parseErr := parseShortcut(record, lineNum)
		if parseErr != nil {
			return nil, parseErr
		}

		shortcuts = append(shortcuts, shortcut)
	}

	return shortcuts, nil
}

func parseVertex(record []string, _ int) (Vertex, error) {
	vertexID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Vertex{}, fmt.Errorf("parse vertex id: %w", err)
	}

	lat, err := strconv.ParseFloat(record[1], 64)
	if err != nil {
		return Vertex{}, fmt.Errorf("parse vertex latitude: %w", err)
	}

	lng, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return Vertex{}, fmt.Errorf("parse vertex longitude: %w", err)
	}

	orderPos, err := strconv.ParseInt(record[3], 10, 64)
	if err != nil {
		return Vertex{}, fmt.Errorf("parse vertex order position: %w", err)
	}

	importance, err := strconv.ParseInt(record[4], 10, 64)
	if err != nil {
		return Vertex{}, fmt.Errorf("parse vertex importance: %w", err)
	}

	return Vertex{
		ID:         vertexID,
		Lat:        lat,
		Lng:        lng,
		OrderPos:   orderPos,
		Importance: importance,
	}, nil
}

func parseEdge(record []string, _ int) (Edge, error) {
	from, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Edge{}, fmt.Errorf("parse edge from vertex: %w", err)
	}

	toVertex, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return Edge{}, fmt.Errorf("parse edge to vertex: %w", err)
	}

	weight, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return Edge{}, fmt.Errorf("parse edge weight: %w", err)
	}

	return Edge{
		From:   from,
		To:     toVertex,
		Weight: weight,
	}, nil
}

func parseShortcut(record []string, _ int) (Shortcut, error) {
	from, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Shortcut{}, fmt.Errorf("parse shortcut from vertex: %w", err)
	}

	toVertex, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return Shortcut{}, fmt.Errorf("parse shortcut to vertex: %w", err)
	}

	weight, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return Shortcut{}, fmt.Errorf("parse shortcut weight: %w", err)
	}

	viaNode, err := strconv.ParseInt(record[3], 10, 64)
	if err != nil {
		return Shortcut{}, fmt.Errorf("parse shortcut via node: %w", err)
	}

	return Shortcut{
		From:    from,
		To:      toVertex,
		Weight:  weight,
		ViaNode: viaNode,
	}, nil
}
