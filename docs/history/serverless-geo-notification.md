# Serverless Geo Notification Playbook

This is compact historical background. It explains the current async notification shape without the original implementation checklist.

## Purpose

The serverless geo notification work split notification publishing from notification delivery:

- API remains responsive after writing and publishing an event.
- Geo worker handles routing and FCM delivery.
- Local development can use HTTP instead of Google Pub/Sub.

## Final Status

The backend async notification foundation is implemented with Pub/Sub/local publishing, `cmd/geoworker`, PMTiles routing, and FCM delivery.

## Current Entry Points

- API service: `cmd/radar`
- Geo worker: `cmd/geoworker`
- Worker handler: `internal/delivery/worker/handler/push_handler.go`
- Pub/Sub providers: `internal/infra/pubsub`
- Notification usecase: `internal/usecase/impl/notification_service.go`
- Operations notes: `docs/operations.md`

## Key Decisions

- API writes the notification record before async delivery.
- API pre-filters subscribers with PostGIS before publishing.
- Production uses Google Pub/Sub; local development can use the local HTTP publisher.
- Geo worker performs route-aware filtering and sends FCM pushes.
- Graceful fallback matters: if route data is unavailable, use Haversine behavior.

## Minimal Flow

1. Merchant calls the notification API.
2. API validates and stores the notification.
3. API pre-filters subscribed users.
4. API publishes a notification event.
5. Geo worker receives the event and filters by route-aware distance.
6. Geo worker sends push notifications to eligible healthy devices.

## Read Next

- `docs/architecture.md`
- `docs/operations.md`
- `docs/reference/cloud-run-jobs.md`
