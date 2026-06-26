# Tier 1 Reliability Playbook

This is compact historical background. It preserves the important implementation context without the original long checklist.

## Purpose

Tier 1 established the reliability and security baseline for public release:

- Device health and rebind.
- Login throttling.
- Refresh-token rotation with reuse detection.
- Argon2id password hashing.
- OAuth provider linking with re-authentication.

## Final Status

The backend foundation is implemented. Remaining work is runtime or client verification, not new backend scope:

- Cloud Run device-cleanup scheduling.
- Firebase invalid-token behavior with real credentials.
- Client device rebind flow.
- Client refresh-token single-flight behavior.
- Argon2id benchmark on the target Cloud Run shape.

## Current Entry Points

- Device API routes: `internal/delivery/api/router/router.go`
- Device contract: `docs/reference/device-health-api.md`
- Cleanup job: `cmd/device-cleanup`
- Auth DTOs: `internal/usecase/user_usecase.go`
- OAuth contract: `docs/reference/google-oauth-api.md`

## Key Decisions

- Device health is computed from token freshness and deletion state, not stored as a cached status column.
- User deactivation and system invalidation are separate states.
- Rebind uses `POST /api/v1/devices`, not a separate reactivation endpoint.
- Login throttling applies to normalized email attempts, including non-existing accounts.
- Refresh tokens rotate and reuse detection revokes the token family.
- OAuth email matching requires re-authentication before provider linking.

## Minimal Flow

Device reliability:

1. Client registers or updates a device token.
2. Push fanout selects active, non-deleted, healthy devices.
3. FCM invalid-token responses or long-term staleness can soft-delete devices.
4. Client checks `/api/v1/devices/health`.
5. Stale or invalid devices recover through the register path.

OAuth linking:

1. Google ID token is verified by the backend.
2. If provider identity already exists, login completes.
3. If email matches an existing local account without that provider, backend returns `linking_required`.
4. Client re-authenticates with password and calls `/auth/link-provider`.

## Read Next

- `docs/roadmap.md`
- `docs/reference/device-health-api.md`
- `docs/reference/google-oauth-api.md`
- `docs/reference/cloud-run-jobs.md`
