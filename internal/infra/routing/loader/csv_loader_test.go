package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVLoader_LoadVertices(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test vertices.csv
	verticesCSV := `id,lat,lng,order_pos,importance
0,25.0330,121.5654,100,1
1,25.0478,121.5170,101,2
2,23.5711,119.5793,102,3
`
	err := os.WriteFile(filepath.Join(tmpDir, "vertices.csv"), []byte(verticesCSV), 0644)
	require.NoError(t, err)

	loader := NewCSVLoader(tmpDir)
	vertices, err := loader.LoadVertices()
	require.NoError(t, err)

	assert.Len(t, vertices, 3)

	// Verify first vertex (Taipei)
	assert.Equal(t, int64(0), vertices[0].ID)
	assert.InDelta(t, 25.0330, vertices[0].Lat, 0.0001)
	assert.InDelta(t, 121.5654, vertices[0].Lng, 0.0001)
	assert.Equal(t, int64(100), vertices[0].OrderPos)
	assert.Equal(t, int64(1), vertices[0].Importance)

	// Verify last vertex (Penghu)
	assert.Equal(t, int64(2), vertices[2].ID)
	assert.InDelta(t, 23.5711, vertices[2].Lat, 0.0001)
	assert.InDelta(t, 119.5793, vertices[2].Lng, 0.0001)
}

func TestCSVLoader_LoadEdges(t *testing.T) {
	tmpDir := t.TempDir()

	edgesCSV := `from,to,weight
0,1,1500.5
1,2,2500.75
0,2,5000.0
`
	err := os.WriteFile(filepath.Join(tmpDir, "edges.csv"), []byte(edgesCSV), 0644)
	require.NoError(t, err)

	loader := NewCSVLoader(tmpDir)
	edges, err := loader.LoadEdges()
	require.NoError(t, err)

	assert.Len(t, edges, 3)
	assert.Equal(t, int64(0), edges[0].From)
	assert.Equal(t, int64(1), edges[0].To)
	assert.InDelta(t, 1500.5, edges[0].Weight, 0.01)
}

func TestCSVLoader_LoadShortcuts(t *testing.T) {
	tmpDir := t.TempDir()

	shortcutsCSV := `from,to,weight,via_node
0,2,4000.0,1
1,3,3500.25,2
`
	err := os.WriteFile(filepath.Join(tmpDir, "shortcuts.csv"), []byte(shortcutsCSV), 0644)
	require.NoError(t, err)

	loader := NewCSVLoader(tmpDir)
	shortcuts, err := loader.LoadShortcuts()
	require.NoError(t, err)

	assert.Len(t, shortcuts, 2)
	assert.Equal(t, int64(0), shortcuts[0].From)
	assert.Equal(t, int64(2), shortcuts[0].To)
	assert.InDelta(t, 4000.0, shortcuts[0].Weight, 0.01)
	assert.Equal(t, int64(1), shortcuts[0].ViaNode)
}

func TestCSVLoader_LoadShortcuts_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewCSVLoader(tmpDir)
	shortcuts, err := loader.LoadShortcuts()
	require.NoError(t, err) // Should not error, just return empty
	assert.Empty(t, shortcuts)
}

func TestCSVLoader_Load_Full(t *testing.T) {
	tmpDir := t.TempDir()

	// Create all required files
	verticesCSV := `id,lat,lng,order_pos,importance
0,25.0,121.5,0,1
1,25.1,121.6,1,2
`
	edgesCSV := `from,to,weight
0,1,1000
`
	shortcutsCSV := `from,to,weight,via_node
0,1,900,2
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vertices.csv"), []byte(verticesCSV), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "edges.csv"), []byte(edgesCSV), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "shortcuts.csv"), []byte(shortcutsCSV), 0644))

	loader := NewCSVLoader(tmpDir)
	data, err := loader.Load()
	require.NoError(t, err)

	assert.Len(t, data.Vertices, 2)
	assert.Len(t, data.Edges, 1)
	assert.Len(t, data.Shortcuts, 1)
}

func TestCSVLoader_LoadVertices_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// CSV with wrong number of columns
	verticesCSV := `id,lat,lng
0,25.0,121.5
`
	err := os.WriteFile(filepath.Join(tmpDir, "vertices.csv"), []byte(verticesCSV), 0644)
	require.NoError(t, err)

	loader := NewCSVLoader(tmpDir)
	_, err = loader.LoadVertices()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 5 columns")
}

func TestCSVLoader_LoadVertices_InvalidNumber(t *testing.T) {
	tmpDir := t.TempDir()

	verticesCSV := `id,lat,lng,order_pos,importance
0,invalid,121.5,0,1
`
	err := os.WriteFile(filepath.Join(tmpDir, "vertices.csv"), []byte(verticesCSV), 0644)
	require.NoError(t, err)

	loader := NewCSVLoader(tmpDir)
	_, err = loader.LoadVertices()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid vertex lat")
}

func TestCSVLoader_LoadVertices_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewCSVLoader(tmpDir)
	_, err := loader.LoadVertices()
	assert.Error(t, err)
}
