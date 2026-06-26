# NomNom-Radar

NomNom-Radar is a backend for mobile-vendor and market discovery. It helps consumers find nearby mobile vendors and market-style vendor clusters, and it lets vendors notify subscribed users when they are nearby.

## Documentation

Start here:

- `AGENTS.md` - instructions for coding agents working in this repository.
- `docs/product.md` - product definition, v1 scope, non-goals, and future direction.
- `docs/architecture.md` - current runtime architecture and service boundaries.
- `docs/roadmap.md` - completed backend foundation, remaining verification, and next directions.
- `docs/operations.md` - deployment and runtime reference index.

Active reference docs:

- `docs/reference/google-oauth-api.md` - Google OAuth mobile ID-token API contract.
- `docs/reference/device-health-api.md` - device health and rebind API contract.
- `docs/reference/cloud-run-jobs.md` - Cloud Run Job deployment and scheduling.

Historical playbooks:

- `docs/history/tier1-reliability.md`
- `docs/history/tier2-discovery.md`
- `docs/history/routing-engine.md`
- `docs/history/serverless-geo-notification.md`

## Tech Stack

- Go with Echo and Fx.
- PostgreSQL/PostGIS.
- GORM and generated query helpers.
- Firebase Cloud Messaging.
- Google Pub/Sub or local HTTP event publishing.
- PMTiles/MVT routing with Haversine fallback.
- Docker, Docker Compose, Cloud Run, and Cloud Run Jobs.

## Getting Started

### Prerequisites

- Go.
- Docker and Docker Compose.
- PostgreSQL with PostGIS.
- Firebase project credentials for push notifications.

### Install Dependencies

```sh
go mod download
```

### Configure Local Runtime

```sh
cp config/config_demo.yaml config/local.yaml
```

Edit `config/local.yaml` with local database, OAuth, Firebase, Pub/Sub, and PMTiles settings.

### Database Setup

The baseline PostgreSQL migration expects PostGIS and UUID support. Supabase deployments need the Supabase pre/post migration workflow when preparing a new database or changing database functions.

Use a dedicated migration DSN secret named `postgres-migration-dsn` when deploying. For Supabase, this DSN must be a direct connection or Supavisor session-mode connection on port `5432`; do not use the transaction pooler on port `6543` for migrations.

Runtime Cloud Run services may still use `postgres-master-dsn` with `POSTGRES_PRESET=supabase_transaction`.

Do not run `DROP EXTENSION postgis CASCADE` on a migrated database.

### Run the API

```sh
ENV=local go run ./cmd/radar
```

### Run the Local Geo Worker

```sh
docker compose --profile dev up --build geoworker
```

## Routing Data

Runtime route-aware notification filtering reads PMTiles through the `pmtiles` config block. When PMTiles is disabled or unavailable, the routing adapter falls back to Haversine distance.

Example PMTiles config:

```yaml
pmtiles:
  enabled: true
  source: "./data/pmtiles/map.pmtiles"
  roadLayer: "transportation"
  zoomLevel: 14
```

The older CH routing CLI under `cmd/routing` is legacy/offline tooling, not the notification runtime path.

See `docs/operations.md` for the minimal PMTiles data preparation workflow.

## Testing

This project uses mockery for generated mocks:

```sh
mockery
```

Run focused Go tests for the package or behavior you changed. Do not run broad suites for docs-only changes.

## License

NomNom-Radar is licensed under AGPL-3.0. See `LICENSE` for the full legal text.
