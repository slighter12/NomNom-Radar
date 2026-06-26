# Device Health API

This is the active client contract for device health and rebind behavior.

## Endpoint

```text
GET /api/v1/devices/health
```

Auth is required. The endpoint returns computed health information for the authenticated user's device records.

## Response Shape

Each device health item includes:

```json
{
  "id": "uuid-of-device-record",
  "client_device_id": "client-side-device-identifier",
  "health_status": "healthy",
  "token_refreshed_at": "2026-04-01T12:00:00Z",
  "requires_rebind": false
}
```

`health_status` values:

- `healthy`: the token is fresh enough for normal push delivery.
- `stale`: the token is too old for normal push delivery and should be rebound.
- `invalid`: the token was invalidated or the device record was soft-deleted by system cleanup.

`requires_rebind` is `true` for `stale` and `invalid`.

## Rebind Contract

When `requires_rebind=true`, the client should:

1. Ask the Firebase SDK for a fresh FCM token.
2. Call `POST /api/v1/devices` with the same client device identifier and the fresh token.

The register path is the recovery path because it can update stale records and restore soft-deleted invalid records. `PUT /api/v1/devices/{deviceId}/token` is for routine token updates on healthy device records.

## Related Runtime Behavior

- Push fanout uses active, non-deleted, healthy device records.
- System cleanup soft-deletes devices whose token freshness exceeds the long-term stale threshold.
- User deactivation and system invalidation are separate states: a user can pause push without the system treating the token as invalid.
