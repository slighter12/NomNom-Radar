# Operations

This is the entry point for deployment and runtime operations. Keep detailed runbooks in focused reference files.

## Runtime Units

- `cmd/radar`: main API service.
- `cmd/geoworker`: Pub/Sub/local HTTP push worker for async notification delivery.
- `cmd/device-cleanup`: scheduled Cloud Run Job for stale device cleanup.

## Local Development

Use `config/config_demo.yaml` as the local configuration template:

```sh
cp config/config_demo.yaml config/local.yaml
ENV=local go run ./cmd/radar
```

Run the local geo worker through Docker Compose when testing async notification delivery:

```sh
docker compose --profile dev up --build geoworker
```

Runtime PMTiles routing uses the `pmtiles` config block. When PMTiles is disabled or unavailable, routing falls back to straight-line Haversine behavior.

## PMTiles Data Preparation

Prepare road PMTiles outside git and provide them through deployment storage or local bind mounts.

Install the common local tools:

```sh
brew install osmium-tool tippecanoe
```

Build a road-only Taiwan PMTiles file from an OSM PBF:

```sh
osmium tags-filter taiwan-latest.osm.pbf w/highway -o filtered-roads.osm.pbf --overwrite
osmium export filtered-roads.osm.pbf -o roads.geojson --overwrite
tippecanoe -o map.pmtiles -z15 -Z15 --buffer=100 --no-clipping --layer=transportation roads.geojson
```

Set `pmtiles.source`, `pmtiles.roadLayer`, and `pmtiles.zoomLevel` to match the generated file. Do not commit generated PMTiles or intermediate OSM/GeoJSON files.

## Deployment References

- Cloud Run services and reusable GitHub Actions workflows are under `.github/workflows/` and `deploy/cloud-run/`.
- Cloud Run Job deployment and scheduling are documented in `docs/reference/cloud-run-jobs.md`.
- Supabase/PostgreSQL migration guidance is documented in `README.md`.
- Google OAuth mobile ID-token contract is documented in `docs/reference/google-oauth-api.md`.
- Device health and rebind contract is documented in `docs/reference/device-health-api.md`.

## Configuration Notes

Important runtime config areas:

- `postgres`: primary database connection and pool settings.
- `secretKey`: access, refresh, onboarding, and linking token keys.
- `googleOAuth.clientId`: mobile ID-token audience.
- `auth`: token TTLs, session limits, and Argon2id settings.
- `loginThrottle`: credential-login lockout settings.
- `firebase`: FCM project and credentials.
- `pubsub`: local or Google Pub/Sub notification event publishing.
- `pmtiles`: route-aware distance source.
- `deviceCleanup`: stale-device cleanup timeout.

Prefer environment overrides and Secret Manager for deployed secrets. Do not commit local credentials.

## Operational Checks

Before a release that touches notifications or device health:

- Confirm Firebase credentials are present in the target environment.
- Confirm Pub/Sub topic/subscription or local publisher endpoint is configured.
- Confirm PMTiles source, layer name, and zoom level are valid.
- Confirm device-cleanup job image is deployed.
- Confirm scheduler configuration only changes when intentionally requested.

Before a release that touches database schema:

- Use the shared migration workflow.
- For Supabase, use the Supabase pre/post migration workflow only when needed.
- Use a direct or session-mode migration DSN on port `5432`, not the transaction pooler on port `6543`.
