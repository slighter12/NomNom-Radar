package main

// RegionConfig defines supported OSM regions
type RegionConfig struct {
	Name        string
	URL         string
	Filename    string
	Description string
}

// SupportedRegions maps region names to their configurations
var SupportedRegions = map[string]RegionConfig{
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

// GetRegionConfig returns the configuration for a given region
func GetRegionConfig(region string) (RegionConfig, bool) {
	config, exists := SupportedRegions[region]
	return config, exists
}

// ListRegions returns a list of all supported region names
func ListRegions() []string {
	regions := make([]string, 0, len(SupportedRegions))
	for region := range SupportedRegions {
		regions = append(regions, region)
	}
	return regions
}
