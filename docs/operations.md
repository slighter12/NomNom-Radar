# Operations

This document is the source of truth for local operation, deployment entry points, shared database migration policy, and release checks. Keep service-specific commands and contracts in focused reference files.

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

## Focused References

- Cloud Run services and reusable GitHub Actions workflows are under `.github/workflows/` and `deploy/cloud-run/`.
- Cloud Run Job deployment and scheduling are documented in `docs/reference/cloud-run-jobs.md`.
- Shared HTTP response envelopes are documented in `docs/reference/api-conventions.md`.
- Google OAuth mobile ID-token contract is documented in `docs/reference/google-oauth-api.md`.
- Device health and rebind contract is documented in `docs/reference/device-health-api.md`.

## Database Migrations

### Local PostgreSQL

Start the repository's PostgreSQL 18/PostGIS master when a compatible database is not already running:

```sh
docker compose up -d postgres-master
```

Install goose once, apply all shared migrations, and inspect their status:

```sh
make db-postgres-install-goose
make db-postgres-up POSTGRES_PORT=5432
make db-postgres-status POSTGRES_PORT=5432
```

The Makefile builds the migration DSN from `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB_NAME`, `POSTGRES_DB_USER`, `POSTGRES_DB_PASSWORD`, and `POSTGRES_SSLMODE`. Port `5432` matches the checked-in Docker Compose master; pass a different `POSTGRES_PORT` or use `PG_URI=...` for another database. The Compose defaults use database `auth_db` and user `user`.

Only shared PostgreSQL migrations have a local apply target. The repository provides `db-supabase-create` for authoring Supabase pre/post migrations, but it does not provide `db-supabase-up`; apply those phases through the deployment workflow described below.

### Deployment

Shared application schema changes live in `database/migration/postgres/` and run through the shared goose migration path. Supabase-specific preparation and function hardening use separate version tables and run around the shared migrations:

1. `database/migration/supabase/pre/`
2. `database/migration/postgres/`
3. `database/migration/supabase/post/`

Use the deployment workflow inputs according to the change:

- New Supabase database: enable both `run_supabase_migration` and `run_migration`.
- Normal shared schema change: enable only `run_migration`.
- Shared function change that requires Supabase hardening: enable both inputs.
- Supabase-only compatibility or hardening change: enable only `run_supabase_migration`.

Deployment migrations prefer the GCP Secret Manager secret `postgres-migration-dsn` and fall back to `postgres-master-dsn` only when the dedicated secret does not exist. For Supabase, the migration DSN must use a direct connection or Supavisor session-mode connection on port `5432`; the workflow rejects transaction-pooler connections on port `6543` because session-level settings are not reliable there.

Runtime services may continue using `postgres-master-dsn` with `POSTGRES_PRESET=supabase_transaction`. Runtime connectivity does not make a transaction-pooler DSN suitable for migrations.

Add a new migration rather than rewriting a migration already applied to a shared environment. Do not run `DROP EXTENSION postgis CASCADE` on a migrated database because dependent geometry columns and spatial indexes can be removed with it.

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

- Select the shared and Supabase workflow inputs according to the deployment policy above.
- Confirm the migration secret resolves to the intended database before deployment.
- For Supabase, confirm the migration DSN uses a direct or session-mode connection on port `5432`.
