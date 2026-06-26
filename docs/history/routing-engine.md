# Routing Engine Playbook

This is compact historical background. It explains how routing evolved and what the current runtime expects.

## Purpose

Routing exists to improve notification quality by filtering subscribed users with route-aware distance instead of only straight-line distance.

## Final Status

The current runtime uses PMTiles/MVT routing with Haversine fallback. The older CH preprocessing path remains legacy/offline tooling only.

## Current Entry Points

- Runtime routing adapter: `internal/infra/routing/pmtiles`
- Geo worker: `cmd/geoworker`
- Notification API service: `cmd/radar`
- Operations notes: `docs/operations.md`

Legacy/offline tooling:

- `cmd/routing`
- `internal/infra/routing/ch`
- `internal/infra/routing/loader`

## Key Decisions

- Notification reliability is more important than exact routing precision.
- PostGIS pre-filtering keeps the routing workload bounded.
- PMTiles/MVT routing is the runtime path.
- Haversine fallback is required when route data is unavailable, incomplete, or outside tile boundaries.
- Generated routing data should not be committed to git.

## Minimal Flow

1. Merchant publishes a location notification.
2. API pre-filters candidate subscribers with PostGIS.
3. Geo worker loads relevant PMTiles/MVT road data.
4. Routing checks route-aware distance.
5. If routing cannot produce a safe answer, fallback distance is used.

## Data Preparation

For local Taiwan data, generate a road-only PMTiles file outside git:

```sh
brew install osmium-tool tippecanoe
osmium tags-filter taiwan-latest.osm.pbf w/highway -o filtered-roads.osm.pbf --overwrite
osmium export filtered-roads.osm.pbf -o roads.geojson --overwrite
tippecanoe -o map.pmtiles -z15 -Z15 --buffer=100 --no-clipping --layer=transportation roads.geojson
```

Configure `pmtiles.source`, `pmtiles.roadLayer`, and `pmtiles.zoomLevel` for the generated file.

## Read Next

- `docs/architecture.md`
- `docs/operations.md`
