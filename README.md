# NomNom-Radar

NomNom-Radar is a backend for mobile-vendor and market discovery. It helps consumers find nearby mobile vendors and market-style vendor clusters, and it lets vendors notify subscribed users when they are nearby.

## Documentation

Read the active documentation in this order:

| Document | Authoritative for |
| --- | --- |
| `docs/product.md` | Product positioning, actors, v1 scope, non-goals, and long-term product rules. |
| `docs/roadmap.md` | Current implementation status, remaining verification risks, and decided next directions. |
| `docs/architecture.md` | Current runtime services, data flow, integrations, and package boundaries. |
| `docs/operations.md` | Local operation, deployment entry points, database migration policy, and release checks. |
| `docs/reference/api-conventions.md` | Shared HTTP success and error envelopes. |
| `docs/reference/google-oauth-api.md` | Google OAuth, provider linking, and merchant onboarding client contract. |
| `docs/reference/device-health-api.md` | Device health and rebind client contract. |
| `docs/reference/cloud-run-jobs.md` | Cloud Run Job deployment, scheduling, and execution details. |
| `AGENTS.md` | Instructions for coding agents working in this repository. |

Files under `docs/history/` are background material for investigation only. They are not current sources of truth.

Agent tooling contracts live under `docs/agents/` and are indexed from `AGENTS.md`.

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

The baseline schema requires PostgreSQL with PostGIS and UUID support. Start the local PostgreSQL master if needed, then install goose once and apply the shared migrations:

```sh
docker compose up -d postgres-master
make db-postgres-install-goose
make db-postgres-up POSTGRES_PORT=5432
```

`make db-postgres-up` builds the DSN from the Makefile's `POSTGRES_*` variables or accepts an explicit `PG_URI=...`. Port `5432` matches the checked-in Docker Compose master; pass a different port or `PG_URI` for another database. Deployment migration policy, including Supabase pre/post ordering and migration DSN rules, is documented in `docs/operations.md` and does not apply to normal local setup.

### Run the API

```sh
ENV=local go run ./cmd/radar
```

### Run the Local Geo Worker

```sh
docker compose --profile dev up --build geoworker
```

## Routing Data

Runtime routing uses PMTiles/MVT with Haversine fallback. See `docs/architecture.md` for runtime behavior and `docs/operations.md` for PMTiles preparation and configuration.

## Testing

This project uses mockery for generated mocks:

```sh
mockery
```

Run focused Go tests for the package or behavior you changed. Do not run broad suites for docs-only changes.

## License

NomNom-Radar is licensed under AGPL-3.0. See `LICENSE` for the full legal text.
