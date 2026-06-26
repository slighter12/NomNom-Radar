# AGENTS.md - NomNom-Radar Agent Guide

These instructions apply to the whole repository.

## Working Rules

- Chat with the user in Traditional Chinese.
- Keep repository artifacts in English.
- Keep changes minimal. Do not refactor, rename public APIs, add dependencies, or introduce speculative abstractions unless the task explicitly requires it.
- Do not treat historical implementation plans as current work. Check the current docs first.
- Do not implement future pass/redemption, organizer-account, or advanced merchant-ops features unless explicitly requested.
- For docs-only work, prefer manual verification over running programs.

## Reading Order

1. `docs/product.md` - product definition, v1 scope, non-goals, and future direction.
2. `docs/architecture.md` - current runtime architecture and service boundaries.
3. `docs/roadmap.md` - completed work, remaining verification, and next product directions.
4. `docs/operations.md` - deployment and runtime reference index.
5. Active reference docs only when relevant: `docs/reference/google-oauth-api.md`, `docs/reference/device-health-api.md`, and `docs/reference/cloud-run-jobs.md`.

Historical playbooks are background only:

- `docs/history/tier1-reliability.md`
- `docs/history/tier2-discovery.md`
- `docs/history/routing-engine.md`
- `docs/history/serverless-geo-notification.md`

## Product Boundary

NomNom-Radar is a mobile-vendor and market-discovery backend. The v1 backend direction is reliable notifications plus discovery.

- Mobile vendors use location publishing, menus, QR subscription, discovery profile, and notification history features.
- Consumers use subscription, authenticated discovery of publicly visible merchants, category/hub filters, and route-aware notification delivery.
- Markets and hubs are platform-defined discovery concepts today.
- Electronic passes, redemption, organizer accounts, and advanced merchant operations are future directions, not v1 requirements.

## Architecture Boundary

Current runtime services:

- `cmd/radar`: main API service.
- `cmd/geoworker`: async Pub/Sub push worker for route-aware notification delivery.
- `cmd/device-cleanup`: Cloud Run Job for stale device cleanup.

Current storage and integrations:

- PostgreSQL/PostGIS for application data and geospatial queries.
- Firebase Cloud Messaging for push delivery.
- Google Pub/Sub or local HTTP publisher for async notification events.
- PMTiles/MVT routing with Haversine fallback for route-aware distance checks.

Legacy/offline routing tooling:

- `cmd/routing`
- `internal/infra/routing/ch`
- `internal/infra/routing/loader`

## Implementation Notes

- Follow the existing Go layering: delivery handlers -> usecases -> domain interfaces/entities -> infra implementations.
- Keep HTTP parsing and response mapping in handlers.
- Keep product validation and authorization-sensitive behavior in usecases.
- Keep persistence and external integration details in infra packages.
- Preserve existing response envelopes, auth boundaries, error semantics, and config names.

## Verification

- Do not run broad test suites unless requested or needed for non-trivial code changes.
- For docs-only changes, check links and verify the docs do not describe completed work as pending.
- For code changes, run the narrowest relevant check and report any skipped verification.
