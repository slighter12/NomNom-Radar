# Architecture

This document describes the current runtime architecture. Historical implementation details are kept in the old plan/spec files only as background.

## Runtime Services

```text
Consumer / Merchant app
        |
        v
cmd/radar API service
        |
        +--> PostgreSQL/PostGIS
        +--> Firebase Cloud Messaging
        +--> Pub/Sub or local HTTP publisher
                    |
                    v
             cmd/geoworker
                    |
                    +--> PMTiles/MVT routing
                    +--> Firebase Cloud Messaging

Cloud Scheduler / manual run
        |
        v
cmd/device-cleanup
        |
        v
PostgreSQL
```

## Main API Service

`cmd/radar` is the primary HTTP API service. It wires configuration, logging, PostgreSQL repositories, auth services, OAuth, Firebase, QR generation, Pub/Sub publishing, PMTiles routing, usecases, middleware, and handlers through Fx.

Current API areas:

- Public auth: email registration/login, refresh/logout, Google OAuth callback, merchant onboarding, provider linking.
- Authenticated user: profile, user locations, devices, device health, subscriptions, QR subscription.
- Discovery: active categories, subcategories, hubs, and authenticated consumer search over publicly visible merchants.
- Merchant: locations, menu, QR, verification, discovery profile, location notifications, notification history.

## Geo Notification Flow

The notification flow is split into a fast API write path and an async delivery path:

1. Merchant publishes a location notification through `cmd/radar`.
2. API validates input and writes the notification record.
3. API pre-filters subscribers with PostGIS so the routing workload stays bounded.
4. API publishes a notification event through Google Pub/Sub in production or local HTTP in development.
5. `cmd/geoworker` receives the event, calculates route-aware distance, filters eligible subscribers, and sends FCM pushes.
6. If async publishing or routing is unavailable, the system uses existing fallback behavior instead of making notification publishing fail by default.

## Routing

The current runtime routing path is PMTiles/MVT based:

- `internal/infra/routing/pmtiles` implements the runtime routing adapter.
- PMTiles routing supports local/remote tile sources, road-layer parsing, local pathfinding, and Haversine fallback.
- The fallback keeps notifications functional when route data is missing, incomplete, or outside tile boundaries.

Legacy routing components remain for offline or historical context:

- `cmd/routing`
- `internal/infra/routing/ch`
- `internal/infra/routing/loader`

Do not treat the old CH routing specification as the current runtime design.

## Device Reliability

Device reliability is part of the v1 trust boundary:

- Devices track token freshness.
- Push delivery uses active, non-deleted, healthy device records.
- Invalid tokens can be soft-deleted after FCM confirms token invalidity.
- `cmd/device-cleanup` soft-deletes permanently stale device records.
- Clients can query device health and rebind stale or invalid devices through the register path. See `docs/reference/device-health-api.md` for the active client contract.

## Data and Integrations

Core dependencies:

- PostgreSQL/PostGIS: application state, auth/session data, geospatial filtering, discovery search.
- Firebase Cloud Messaging: push delivery.
- Google Pub/Sub: production notification-event queue.
- Local HTTP publisher: development substitute for Pub/Sub push delivery.
- PMTiles/MVT: route-aware distance checks.
- Google ID token verification: mobile OAuth sign-in.

## Package Boundaries

Follow the existing layering:

- `internal/delivery`: HTTP/worker delivery, middleware, handlers, request parsing, response mapping.
- `internal/usecase`: application behavior, product validation, authorization-sensitive orchestration.
- `internal/domain`: entities, constants, repository/service interfaces, domain errors, policies.
- `internal/infra`: database, auth, OAuth, Firebase, Pub/Sub, QR, routing implementations.

Keep new behavior in the same boundary. Do not bypass usecases from handlers for business logic.
