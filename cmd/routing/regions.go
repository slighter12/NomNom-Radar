package main

import "sort"

// RegionConfig defines supported OSM regions
type RegionConfig struct {
	Name        string
	URL         string
	Filename    string
	Description string
}

// getSupportedRegions returns the map of supported OSM regions
func getSupportedRegions() map[string]RegionConfig {
	return map[string]RegionConfig{
		"taiwan": {
			Name:        "Taiwan",
			URL:         "https://download.geofabrik.de/asia/taiwan-latest.osm.pbf",
			Filename:    "taiwan-latest.osm.pbf",
			Description: "Taiwan island and surrounding islands (~310 MB)",
		},
		"japan": {
			Name:        "Japan",
			URL:         "https://download.geofabrik.de/asia/japan-latest.osm.pbf",
			Filename:    "japan-latest.osm.pbf",
			Description: "Japan and surrounding islands (~2.5 GB)",
		},
	}
}

// GetRegionConfig returns the configuration for a given region
func GetRegionConfig(region string) (RegionConfig, bool) {
	config, exists := getSupportedRegions()[region]

	return config, exists
}

// ListRegions returns a list of all supported region names
func ListRegions() []string {
	supported := getSupportedRegions()
	regions := make([]string, 0, len(supported))

	for region := range supported {
		regions = append(regions, region)
	}

	sort.Strings(regions)

	return regions
}
