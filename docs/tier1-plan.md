# Tier 1 — Core Reliability Implementation Plan

This document details the implementation plan for Tier 1 items from [product-priority-notes.md](./product-priority-notes.md). All five items must be completed before public release.

---

## 1. Device Health / Rebind Flow

### Goal

Implement FCM token lifecycle management following [Google's official best practices](https://firebase.google.com/docs/cloud-messaging/manage-tokens), enabling proactive stale token detection, push delivery prioritization by device activity, and a client-facing health query API with rebind contract.

### Current State

- Device CRUD exists: register, update FCM token, deactivate (`device_handler.go`).
- Tokens confirmed as unregistered by FCM are reactively cleaned during push send via `cleanupInvalidTokens()` (`push_handler.go:443`).
- `UserDevice` entity has `UpdatedAt` but no dedicated token-refresh timestamp.
- No proactive staleness detection, no device health status, no client recovery flow.
- Device uses GORM soft-delete (`DeletedAt`) + `IsActive` flag — two state axes that must be kept consistent.

### Design

#### 1.1 Schema Changes — `user_devices` table

| Column | Type | Description |
|--------|------|-------------|
| `token_refreshed_at` | `timestamptz NOT NULL` | Last time the client reported a fresh FCM token. Initialized on register, updated on `UpdateFCMToken`. |

**Note on `health_status`**: Staleness is a derived state computed from `token_refreshed_at` at query time, not stored as a column. This avoids the consistency issues of maintaining a cached status that can drift between scheduled job runs.

Staleness thresholds (applied in queries and API responses):

- `healthy`: `token_refreshed_at` within the last 30 days.
- `stale`: `token_refreshed_at` older than 30 days.
- `invalid`: Device has been soft-deleted via `DeletedAt` after FCM returned `UNREGISTERED`, or `INVALID_ARGUMENT` when the outgoing payload is known to be valid.

**Device state model — two independent axes**:

The existing `IsActive` flag and GORM `DeletedAt` soft-delete serve different purposes and must remain independent:

| State | Meaning | Triggered by |
|-------|---------|-------------|
| `IsActive = false` | User voluntarily disabled push for this device. Record remains visible and can be reactivated. | User calls `DeactivateDevice` API. |
| `DeletedAt IS NOT NULL` | System determined the device token is permanently invalid. This is the terminal state. | FCM returns `UNREGISTERED`, FCM returns token-related `INVALID_ARGUMENT` with a known-valid payload, or token exceeds 270-day expiration. |

**Independence rule**: System invalidation (soft-delete) does not check or depend on the current `IsActive` value. A device that is already `IsActive = false` (user-paused) can still be soft-deleted by the system if its token becomes invalid. Possible state combinations:

| `IsActive` | `DeletedAt` | Meaning |
|------------|-------------|---------|
| `true` | `NULL` | Normal active device. |
| `false` | `NULL` | User paused push. Can be reactivated. |
| `true` | `NOT NULL` | System invalidated an active device. |
| `false` | `NOT NULL` | System invalidated a user-paused device. |

Push delivery filter combines all three conditions: `IsActive = true AND deleted_at IS NULL AND token_refreshed_at > NOW() - 30 days`.

**Behavior change required**: The existing `DeactivateDevice()` implementation currently calls `DeleteDevice()` (GORM soft-delete via `deleted_at`). This must be changed to only set `IsActive = false` without touching `deleted_at`, so that user-voluntary deactivation and system-level invalidation remain distinct. `DeleteDevice()` continues to be used only for system-level invalidation (`UNREGISTERED`, token-related `INVALID_ARGUMENT` with a known-valid payload, 270-day expiry).

**FCM error handling note**: Firebase documents `INVALID_ARGUMENT` as ambiguous: it can represent an invalid registration token, but it can also represent an invalid message payload. The implementation may treat `INVALID_ARGUMENT` as token invalid only when the payload shape is controlled and known to be valid; otherwise, only `UNREGISTERED` should trigger token deletion.

#### 1.2 Push Delivery — Active/Inactive Segmentation

Modify the push send path to filter devices at query time:

1. Query devices with `is_active = true AND deleted_at IS NULL` (existing behavior for active, non-deleted devices).
2. Add filter: `token_refreshed_at > NOW() - INTERVAL '30 days'` to only return healthy devices.
3. Stale devices (>30 days, still active and not deleted) are excluded from push send.
4. User-deactivated devices (`IsActive = false`) are excluded by the existing filter.
5. System-invalidated devices (`DeletedAt IS NOT NULL`) are excluded by GORM soft-delete.

This query-time filtering replaces the need for a cached `health_status` column and eliminates the time window where a stale device could still receive pushes between scheduled job runs.

This segmentation applies to both the sync path (`notification_service.go`) and the async Pub/Sub worker path (`push_handler.go`).

> **Tier 2 linkage**: The `token_refreshed_at` field provides the data foundation for Merchant Notification Analytics (Tier 2 #6). Active subscriber count (token refreshed within 30 days) vs. total subscriber count can be derived directly from this field.

#### 1.3 Stale Token Cleanup — Scheduled Task

Implement a periodic cleanup job (Cloud Scheduler + Pub/Sub or Cloud Run Jobs):

- Soft-delete devices (set `DeletedAt`) when `token_refreshed_at` exceeds 270 days (Android FCM expiration threshold).
- Unsubscribe expired devices from any topic-based subscriptions if applicable.
- Devices between 30–270 days are excluded from push at query time but kept in the database for potential rebind.
- Implementation target: `cmd/device-cleanup`, built with Docker target `device-cleanup` and executed as a Cloud Run Job. See [cloud-run-jobs.md](./cloud-run-jobs.md).

#### 1.4 Device Health Query API

New endpoint: `GET /api/v1/devices/health`

Response per device:

```json
{
  "id": "uuid-of-device-record",
  "client_device_id": "client-side-device-identifier",
  "health_status": "healthy | stale | invalid",
  "token_refreshed_at": "2026-04-01T12:00:00Z",
  "requires_rebind": true
}
```

- Both `id` (DB UUID) and `client_device_id` are returned to avoid ambiguity.
- `health_status` is computed at response time from `token_refreshed_at` and `deleted_at`.
- The health query must include soft-deleted records (for example, via an explicit deleted-device query or an unscoped lookup), otherwise `invalid` devices cannot be returned to the client for recovery.
- `requires_rebind = true` when `health_status` is `stale` or `invalid`.
- Client contract: when `requires_rebind` is true, the client should call `getToken()` (FCM SDK) and then `POST /api/v1/devices` (register) with the same `device_id` and the new `fcm_token`. The register endpoint's upsert logic handles all cases: updating stale devices, restoring soft-deleted (invalid) devices, and creating new ones.
- **Note**: `PUT /api/v1/devices/{id}/token` is only for routine token updates on healthy devices. Rebind always goes through register, since `UpdateFCMToken` cannot restore a soft-deleted device.

#### 1.5 Rebind Recovery for Invalid Devices

Currently, `DeleteDevice()` uses GORM soft-delete (`deleted_at`), and `UpdateFCMToken()` only updates the token column without clearing `deleted_at`. To support rebind of invalid devices:

- The existing `RegisterDevice` flow (which uses upsert by `user_id` + `device_id`) will be extended to restore soft-deleted devices: if a matching device exists with `deleted_at IS NOT NULL`, clear `deleted_at`, update `fcm_token`, and set `token_refreshed_at` to now.
- This reuses the existing register path rather than adding a separate "reactivate" endpoint.

#### 1.6 Files to Modify

| File | Changes |
|------|---------|
| `internal/domain/entity/device.go` | Add `TokenRefreshedAt` field. |
| `internal/domain/repository/device_repository.go` | Add `DeviceListFilter`, `SoftDeleteStaleDevices`. Consolidate list queries into `FindDevicesByUser(ctx, userID, filter)` to support healthy filtering and health queries that include soft-deleted devices. |
| `internal/infra/persistence/model/device_model.go` | Add `TokenRefreshedAt` column. |
| `internal/infra/persistence/postgres/device_repository.go` | Implement new repository methods. Extend upsert to restore soft-deleted devices on rebind. |
| `internal/usecase/impl/device_service.go` | Health query logic (compute status from `token_refreshed_at`); set `TokenRefreshedAt` on register and token update. |
| `internal/domain/repository/subscription_repository.go` | Update `FindDevicesForUsers` interface to include `token_refreshed_at` filter. |
| `internal/infra/persistence/postgres/subscription_repository.go` | Add `token_refreshed_at > 30 days` filter to `FindDevicesForUsers()` query (line 300–306). This is the primary push fanout query path. |
| `internal/usecase/impl/notification_service.go` | Verify push path uses updated `FindDevicesForUsers` with healthy filter. |
| `internal/delivery/worker/handler/push_handler.go` | Verify async push path uses updated `FindDevicesForUsers`; mark devices as soft-deleted only when FCM returns `UNREGISTERED`. Keep `INVALID_ARGUMENT` out of cleanup unless the implementation can prove the payload is known-valid and the error is token-related. |
| `internal/usecase/impl/device_service.go` | Change `DeactivateDevice` to set `IsActive = false` instead of calling `DeleteDevice`. Add health query method. |
| `internal/usecase/device_usecase.go` | Add health query method to usecase interface. |
| `internal/delivery/api/router/handler/device_handler.go` | New health query endpoint; set `TokenRefreshedAt` in register/update flows. |
| `internal/delivery/api/router/router.go` | Register new health route. |
| DB migration | Add `token_refreshed_at` column (NOT NULL, default NOW()). Add index on `(user_id, is_active, token_refreshed_at) WHERE deleted_at IS NULL` for push fanout queries. |

---

## 2. Login Throttling / Brute-force Protection

### Goal

Prevent credential brute-forcing with progressive lockout using the existing PostgreSQL database. Throttle by normalized email (`attempt_key`) to cover both existing and non-existing accounts, preventing email spray / account enumeration attacks. IP-level edge protection is deferred to Cloud Armor (see Scale-up Notes).

**Scope**: Throttling applies only to the email/password login path (`authMethodEmailPassword`). OAuth login paths (`authMethodOAuth`) are not subject to login throttling, since OAuth does not involve password guessing — failures are token verification issues, not brute-force scenarios.

### Current State

- No rate limiting or login attempt tracking exists.
- Login flow: `user_handler.go:85` → `userService.Login()` → bcrypt verification.
- Invalid credentials return a generic error (does not leak account existence).

### Design

#### 2.1 Schema — New `login_attempts` table

| Column | Type | Description |
|--------|------|-------------|
| `id` | `uuid` | Primary key. |
| `attempt_key` | `text` | `UNIQUE`. Normalized email address. Covers both existing and non-existing accounts. |
| `user_id` | `uuid` | Nullable FK to users. Set when the account exists; null for non-existing email attempts. |
| `failed_count` | `int` | Consecutive failed attempts since last successful login or lockout reset. |
| `lockout_count` | `int` | Number of times this key has been locked out (for progressive backoff). |
| `locked_until` | `timestamptz` | Null if not locked; otherwise the time when lockout expires. |
| `last_failed_at` | `timestamptz` | Timestamp of the most recent failed attempt. |
| `last_lockout_at` | `timestamptz` | Timestamp of the most recent lockout trigger (for decay calculation). |

**Key design choice**: Using `attempt_key` (normalized email) instead of only `user_id` ensures that brute-force attempts against non-existent accounts are also throttled, preventing email spray / account enumeration attacks.

**Unique constraint on `attempt_key`** ensures exactly one record per email, enabling row-level locked updates without race conditions.

**Account creation boundary**: If an anonymous attempt row exists for an email before the account is created, account creation resets its counters and attaches the new `user_id`. This prevents a newly registered account from inheriting pre-registration lockout state while keeping future audit records associated with the account from the creation boundary onward.

**Attribution boundary**: Runtime login checks may resolve a `user_id` for the current request and use it for in-process behavior such as security notifications. They must not backfill `user_id` onto an existing anonymous row during login. Persisting that association outside the account creation boundary would incorrectly attribute pre-account activity to the newly created account.

#### 2.2 Progressive Lockout (Exponential Backoff)

| Lockout round | Trigger | Lock duration |
|---------------|---------|---------------|
| 1st | 5 consecutive failures | 15 minutes |
| 2nd | 5 more failures after unlock | 1 hour |
| 3rd | 5 more failures after unlock | 4 hours |
| 4th+ | 5 more failures after unlock | 24 hours (cap) |

**Reset conditions:**

- Successful login → reset `failed_count` and `lockout_count` to 0.
- 7 days after `last_lockout_at` with no further failures → decay `lockout_count` to 0.

**Lockout duration formula:** `min(15 * 4^(lockout_count - 1), 1440)` minutes.

**Concurrency**: `failed_count` and `lockout_count` updates run inside a row-level locked transaction (`SELECT ... FOR UPDATE`). After the row lock is acquired, the implementation rechecks `locked_until`; requests that arrive after another request has triggered lockout return the same generic lockout response without incrementing the next lockout round.

**Future optimization**: `DecayLockoutCounts()` currently runs inline during login throttle checks. If the `login_attempts` table becomes large, move decay to a scheduled cleanup job or make it lazy per attempt key.

#### 2.3 Push Notification on Lockout

When a lockout is triggered, send a push notification to the user's active devices:

- **Trigger**: On each new lockout event (not on every failed attempt during an active lockout).
- **Content** (example): "Your account has been temporarily locked due to multiple failed login attempts. It will unlock at {time}. If this wasn't you, please change your password."
- **Precondition**: User exists and has at least one healthy device (not applicable for non-existing account attempts). Since #1 Device Health is part of Tier 1, security alerts use `FindDevicesByUser` with `OnlyHealthy` filter so user-paused, system-invalidated, and stale-token devices are excluded.
- **No device available**: Skip notification silently. Email notification can be added after Tier 3 email infrastructure is in place.

#### 2.4 Login Flow Changes

```txt
Login request received (with email + password)
  → Normalize email
  → UPSERT into login_attempts by attempt_key (normalized email)
  → If locked_until > now: reject with generic INVALID_CREDENTIALS error
      (include Retry-After header with seconds until unlock, but do NOT
       return a distinct ACCOUNT_LOCKED error code to avoid leaking account existence)
  → If not locked: proceed with account lookup and password verification
    → On failure (wrong password OR account not found):
        In a row-level locked transaction:
          - Re-check locked_until
          - Increment failed_count or trigger the next lockout
        If failed_count >= 5: trigger lockout, increment lockout_count
        If user exists and has active devices: send push notification
    → On success:
        Reset failed_count and lockout_count to 0
```

**Security note**: The locked state returns the same generic error as invalid credentials. The `Retry-After` HTTP header provides timing information without confirming account existence (the header is set on all rejections during lockout, regardless of whether the account exists).

#### 2.5 Scale-up Note: Cloud Armor

When request volume grows beyond what application-level throttling can handle, enable [Google Cloud Armor](https://cloud.google.com/armor/pricing) with a rate-limiting security policy on the Global External Application Load Balancer in front of Cloud Run. This provides IP-based edge-level protection against volumetric brute-force and DDoS attacks without application code changes.

This is intentionally deferred because Cloud Armor requires a GLB ($18+/month forwarding rule) plus per-policy/rule/request fees, which is not justified at pre-launch scale.

#### 2.6 Files to Modify

| File | Changes |
|------|---------|
| `internal/domain/entity/` | New `login_attempt.go` entity. |
| `internal/domain/repository/` | New `login_attempt_repository.go` interface (with row-level locked update methods). |
| `internal/infra/persistence/model/` | New `login_attempt_model.go`. |
| `internal/infra/persistence/postgres/` | New `login_attempt_repository.go` implementation (row-level locked update transaction). |
| `config/config.go` | Add `LoginThrottle` config (max attempts, lockout durations, decay period). |
| `internal/usecase/impl/user_service_auth.go` | Integrate lockout check before password verification; atomic update on success/failure. |
| `internal/usecase/impl/notification_service.go` | Add method to send security alert push notification. |
| `internal/delivery/api/router/handler/user_handler.go` | In the login handler, detect lockout errors and set `Retry-After` HTTP header directly. This is a delivery-layer concern — no changes needed to the domain `AppError` interface. |
| DB migration | New `login_attempts` table with unique constraint on `attempt_key`. |

---

## 3. Refresh-token Rotation with Reuse Detection

### Goal

Implement refresh token rotation with token family tracking and automatic reuse detection, following [RFC 9700](https://datatracker.ietf.org/doc/rfc9700/) (OAuth 2.0 Security Best Current Practice, January 2025) and the [Auth0 token family model](https://auth0.com/docs/secure/tokens/refresh-tokens/refresh-token-rotation).

### Current State

- Refresh token stored as SHA-256 hash in `refresh_tokens` table.
- `RefreshToken` entity: `ID`, `UserID`, `TokenHash`, `ExpiresAt`, `CreatedAt`.
- On refresh: only a new access token is issued; refresh token is reused indefinitely.
- `RotateTokens()` method exists in `jwt_service.go:157` but is never called.
- Session management exists: `maxActiveSessions`, `LogoutAllDevices`, `DeleteExpiredRefreshTokens`.
- `AcquireSessionMutex()` uses `SELECT ... FOR UPDATE` (blocking lock, not try-lock).

### Design

#### 3.1 Schema Changes — `refresh_tokens` table

| Column | Type | Description |
|--------|------|-------------|
| `family_id` | `uuid NOT NULL` | Groups all tokens descended from the same login event. Indexed. |
| `is_revoked` | `bool NOT NULL DEFAULT false` | Set true when token is rotated out or family is invalidated. |
| `replaced_by` | `uuid` | Nullable FK to `refresh_tokens.id`. Points to the new token that replaced this one. |

**Index requirements:**

- `idx_refresh_tokens_family_id` on `family_id` — for bulk family revocation.
- Modify existing session queries to use composite condition: `user_id + is_revoked = false + expires_at > now()`.

#### 3.2 Token Rotation Flow

All steps within a single database transaction, serialized by `AcquireSessionMutex()`:

```txt
POST /auth/refresh  (with refresh_token_v2)
  → Validate JWT signature
  → Hash token
  → BEGIN TRANSACTION
    → AcquireSessionMutex(user_id)  [blocking FOR UPDATE on users row]
    → Lookup token by hash (including revoked tokens)
    → If token not found: ROLLBACK, reject (invalid token)
    → If token.is_revoked == true:
        ⚠️ REUSE DETECTED — this token was already rotated
        → UPDATE refresh_tokens SET is_revoked = true WHERE family_id = token.family_id
        → COMMIT
        → Send push notification to user's active devices (security alert, async/outside tx)
        → Return error, force re-authentication
    → If token is valid and not revoked:
        → Mark current token: is_revoked = true
        → Generate new token pair via RotateTokens()
        → INSERT new refresh token with same family_id
        → UPDATE old token: replaced_by = new token ID
        → COMMIT
        → Return new access_token + new refresh_token
```

**Transaction boundary**: The lookup, revocation check, old token update, and new token insert are all within the same transaction to prevent TOCTOU races.

#### 3.3 Token Family Lifecycle

- **Created**: On login or registration, a new `family_id` (UUID) is generated.
- **Extended**: Each refresh creates a new token in the same family.
- **Destroyed**: On reuse detection, logout, or `LogoutAllDevices`, all tokens in the family are revoked.

#### 3.4 Race Condition Handling

Concurrent refresh requests with the same token can cause false reuse detection.

- `AcquireSessionMutex()` uses `SELECT ... FOR UPDATE` on the user row, which is a **blocking lock** — concurrent requests from the same user will serialize, not fail.
- This means the second concurrent request will wait for the first to complete, then find the token already revoked and trigger reuse detection.
- This is an intentional safety-first tradeoff: if two processes hold the same refresh token and try to use it simultaneously, one of them may be an attacker.
- The server does not currently implement idempotent refresh replay because it stores only refresh token hashes. Replaying the same successful rotation response would require a short-lived response store that can safely retain the raw newly issued refresh token.

#### 3.5 Client Retry Contract

The refresh endpoint is intentionally non-idempotent in Tier 1. Clients must implement single-flight refresh behavior:

- Only one refresh request may be in flight per client session.
- Concurrent API calls that encounter an expired access token must wait for the same refresh operation instead of starting separate refresh requests.
- The client must not blindly retry a timed-out refresh request with the same refresh token.
- Reusing an already rotated refresh token is treated as reuse detection and revokes the token family.

Server-side idempotent refresh retry can be added later with Redis or a dedicated short-lived PostgreSQL idempotency table if mobile retry behavior proves problematic. That future design must avoid long-lived storage of raw refresh tokens.

#### 3.6 Session Query Updates

The following existing queries must be updated to exclude revoked tokens:

| Method | Current behavior | Required change |
|--------|-----------------|-----------------|
| `FindRefreshTokensByUserID()` | Returns all non-expired tokens | Add `AND is_revoked = false` |
| `CountActiveSessionsByUserID()` | Counts all non-expired tokens | Add `AND is_revoked = false` |

This ensures revoked tokens do not count toward `maxActiveSessions` and do not appear in session listings.

#### 3.7 Cleanup

- `DeleteExpiredRefreshTokens()` already exists. Extend it to also hard-delete revoked tokens older than the refresh TTL (7 days), since they are only needed for reuse detection within their validity window.

#### 3.8 Client Contract Changes

The refresh response will now include both tokens:

```json
{
  "access_token": "...",
  "refresh_token": "..."
}
```

Clients must store and use the new refresh token for subsequent refreshes. Using an old refresh token after rotation will trigger reuse detection and full session revocation.

#### 3.9 Files to Modify

| File | Changes |
|------|---------|
| `internal/domain/entity/auth.go` | Add `FamilyID`, `IsRevoked`, `ReplacedBy` to `RefreshToken`. |
| `internal/domain/repository/refresh_token_repository.go` | Add `RevokeTokenFamily`, `RevokeTokenFamiliesByUserID`, `FindRefreshTokenByHashIncludingRevoked`. |
| `internal/infra/persistence/model/auth_model.go` | Add new columns to refresh token model. |
| `internal/infra/persistence/postgres/refresh_token_repository.go` | Implement new methods. Update `FindRefreshTokensByUserID` and `CountActiveSessionsByUserID` to filter `is_revoked = false`. |
| `internal/usecase/impl/user_service.go` | Rewrite `RefreshToken()` to implement rotation + reuse detection within a transaction. |
| `internal/usecase/impl/user_service_auth.go` | Set `family_id` on login/registration token creation. |
| `internal/usecase/user_usecase.go` | Update `RefreshTokenOutput` DTO to include both access and refresh tokens. |
| `internal/infra/auth/jwt_service.go` | No changes needed — `RotateTokens()` already exists. |
| `internal/delivery/api/router/handler/user_handler.go` | Return both tokens in refresh response. |
| DB migration | Add `family_id` (NOT NULL, no default), `is_revoked` (NOT NULL DEFAULT false), `replaced_by` (nullable FK) columns. Add index on `family_id`. **Migration note**: Since there are no existing users (pre-launch), the `refresh_tokens` table is expected to be empty. If stale test data exists, backfill `family_id` before setting `NOT NULL`; do not keep a default because refresh token creation must explicitly assign a token family. |

---

## 4. Password Hashing Upgrade (Argon2id)

### Goal

Replace bcrypt with Argon2id for password hashing. Since there are no existing users, this is a direct replacement with no migration path required.

### Current State

- `bcrypt_hasher.go` implements `PasswordHasher` interface with bcrypt (cost=12).
- Interface: `Hash(password) (string, error)`, `Check(password, hash) bool`, `ValidatePasswordStrength(password) error`.
- Password strength validation (length, uppercase, lowercase, numbers, special chars) is in the hasher.
- Config: `AuthConfig.BcryptCost`.

### Design

#### 4.1 Argon2id Parameters

Based on OWASP 2025 recommendations, tuned for Cloud Run (512Mi memory limit):

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Memory | 64 MB (65536 KB) | OWASP recommended. ~12.5% of 512Mi per hash operation. |
| Iterations | 3 | OWASP recommended for 64MB memory. |
| Parallelism | 1 | Cloud Run typically runs 1-2 vCPU; more lanes add scheduling overhead without real speedup. |
| Salt length | 16 bytes | Standard recommendation. |
| Key length | 32 bytes | Standard recommendation. |

#### 4.2 Benchmark (prerequisite)

Before finalizing parameters, run a benchmark test in a Cloud Run-equivalent environment (512Mi memory) to measure:

- Single hash latency (target: <500ms for acceptable login UX).
- Memory consumption per hash operation.
- Behavior under concurrent hash load (4–6 simultaneous operations).

If single-hash latency exceeds 500ms, reduce memory to 46MB (OWASP minimum) and re-test. Parameters are configurable — adjustments require only a config change, no code change.

**Future consideration**: If peak login traffic causes latency issues, a dedicated Cloud Run service for auth operations (with higher memory allocation) can isolate hash workload from the main API. This is not needed at pre-launch scale.

#### 4.3 Concurrency Control

At 64MB per hash operation, unbounded concurrent hashing could exhaust the 512Mi memory limit (e.g., 7+ simultaneous hash operations ≈ 448MB + runtime overhead → OOM).

**Mitigation**: Add a counting semaphore (buffered channel) in the hasher to limit concurrent hash/verify operations. Cap at **4 concurrent operations** (4 × 64MB = 256MB, leaving ~256MB for runtime + other request processing).

The semaphore is internal to the `Argon2idHasher` — callers (login, registration) are unaware of it. If the semaphore is full, the caller blocks briefly until a slot opens. This is acceptable because password operations are already CPU/memory-bound and infrequent relative to total request volume.

#### 4.4 Implementation

- Create `argon2id_hasher.go` implementing the existing `PasswordHasher` interface.
- Include the concurrency semaphore within the hasher struct.
- Replace `NewBcryptHasher` with `NewArgon2idHasher` in dependency injection.
- Remove `bcrypt_hasher.go` entirely (no dual-hash migration needed).
- Update `AuthConfig`: replace `BcryptCost` with Argon2id-specific fields (`Argon2Memory`, `Argon2Iterations`, `Argon2Parallelism`, `Argon2MaxConcurrent`).

#### 4.5 Hash Format

Use the standard PHC string format for storage:

```txt
$argon2id$v=19$m=65536,t=3,p=1$<base64-salt>$<base64-hash>
```

This is self-describing: parameters are embedded in the hash string. If parameters change in the future, old hashes can still be verified, and re-hashing can happen on next login.

#### 4.6 Dependency

- `golang.org/x/crypto/argon2` — already available since `golang.org/x/crypto` is a current dependency for bcrypt.

#### 4.7 Files to Modify

| File | Changes |
|------|---------|
| `internal/infra/auth/bcrypt_hasher.go` | Delete. |
| `internal/infra/auth/argon2id_hasher.go` | New file implementing `PasswordHasher` with Argon2id + concurrency semaphore. |
| `internal/domain/service/password_hasher.go` | No changes — interface is algorithm-agnostic. |
| `config/config.go` | Replace `BcryptCost` in `AuthConfig` with `Argon2Memory`, `Argon2Iterations`, `Argon2Parallelism`, `Argon2MaxConcurrent`. |
| DI / wire setup (e.g., `cmd/radar/main.go`) | Replace `NewBcryptHasher` → `NewArgon2idHasher`. |

---

## 5. OAuth Account Linking — Require Re-authentication

### Goal

Prevent unauthorized account takeover by requiring existing users to re-authenticate before an OAuth provider is linked to their account. Currently, if an OAuth login (e.g., Google) uses an email that already exists in the system, the provider is automatically linked without verification — this is a security vulnerability.

**Official reference**: [Google Identity — Verify Google ID token](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token) recommends that if a site already has an account with that email, the user should be prompted to verify their existing credentials before the OAuth provider link is created.

**Future Apple provider note**: Sign in with Apple can return a Hide My Email relay address (`@privaterelay.appleid.com`) instead of the user's real email. Email matching is therefore only a convenience signal, not a universal account-linking guarantee. Future relay-email providers must keep `(provider, provider_user_id)` as the identity key and provide an explicit "I already have an account" linking path when the provider email does not match the existing account email.

### Current State

- When OAuth login finds a matching email in the database but no existing provider link, it automatically creates the link (`user_service_auth.go` L228, L500).
- This means an attacker who controls a Google account with the victim's email could gain access to the victim's account without any additional verification.

### Design

#### 5.1 Linking Flow Change

```txt
OAuth login received (e.g., Google ID token with email X)
  → Verify ID token (existing logic, unchanged)
  → Lookup provider link by (provider, provider_user_id)
  → If link exists: login as usual (unchanged)
  → If link does NOT exist:
      → Lookup user by normalized email
      → If no user found: create new account + link (unchanged)
      → If user found (email already registered):
          ⚠️ DO NOT auto-link
          → Return a "linking_required" response with a short-lived linking token
          → Client must present the linking token + existing account password
          → After successful re-auth, create the provider link
```

#### 5.2 Auth Response Contract

Add a new `AuthStatus` value: `linking_required`.

The existing `AuthResult` / `AuthStatus` only supports `authenticated` and `onboarding_required`. A third status is needed so the handler can distinguish this case and return the linking token to the client.

```go
AuthStatusLinkingRequired AuthStatus = "linking_required"
```

The response for `linking_required` includes a linking token but no access/refresh tokens.

#### 5.3 Linking Token

Extend the existing JWT token pattern (same approach as onboarding token):

- **Token type**: `TokenTypeLinking = "linking"` (new constant in `token_service.go`).
- **TTL**: 10 minutes.
- **Claims**: Extend `Claims` struct or use a dedicated `LinkingClaims` with:
  - `UserID` — the existing account to link to.
  - `Provider` — the OAuth provider (e.g., `"google"`).
  - `ProviderUserID` — the OAuth provider's unique user ID (`sub`).
- **Signing**: Use `secretKey.linking` when configured. If omitted, derive a dedicated linking secret from the access secret using the token type, matching the onboarding token fallback strategy.
- Add `GenerateLinkingToken(userID, provider, providerUserID, requestedRole, storeName)` to `TokenService` interface and `jwtService`.

#### 5.4 New Endpoint

`POST /auth/link-provider`

This endpoint is intentionally registered under the public auth route group, not under `/api/v1`, because the caller is not authenticated with an access token yet. The request is authorized by the short-lived linking token plus existing account password verification.

Request:

```json
{
  "linking_token": "...",
  "password": "existing-account-password"
}
```

Response on success: normal login tokens (access + refresh), same as regular login.

**Error cases:**

| Scenario | HTTP | Error code | Notes |
|----------|------|------------|-------|
| Invalid or expired linking token | 401 | `INVALID_TOKEN` | Client should restart the OAuth flow. |
| Wrong password | 401 | `INVALID_CREDENTIALS` | Same generic error as login. |
| Provider already linked to this account | 200 | (success) | Idempotent — if the link already exists, treat as successful login. |
| Provider linked to a different account | 409 | `PROVIDER_ALREADY_LINKED` | The provider `sub` is already bound to another user. |

#### 5.5 Relationship with Existing `LinkGoogleAccount`

The project already has a post-login linking flow (`LinkGoogleAccount` in `user_service.go:633`, `user_usecase.go:105`). The two flows serve different purposes:

| | Existing `LinkGoogleAccount` | New `/auth/link-provider` |
|---|---|---|
| **Trigger** | User is already logged in and wants to add Google | User tries OAuth login and discovers email conflict |
| **Auth state** | Authenticated (has valid access token) | Unauthenticated (has only a linking token) |
| **Re-auth** | Not needed (already logged in) | Required (password verification) |

The underlying provider link creation logic (`ensureOAuthAuthLink` or similar) should be shared between both flows. The new flow adds the re-auth gate before calling the same link-creation logic.

#### 5.6 Files to Modify

| File | Changes |
|------|---------|
| `internal/usecase/impl/user_service_auth.go` | Remove auto-linking on email match (L228, L500). Return `AuthStatusLinkingRequired` with linking token instead. |
| `internal/usecase/user_usecase.go` | Add `AuthStatusLinkingRequired`. Add `LinkProviderInput/Output` DTO. Add `LinkProvider` method to usecase interface. |
| `internal/usecase/impl/user_service.go` | Implement `LinkProvider` — validate linking token, verify password, call shared link-creation logic, return login tokens. |
| `internal/delivery/api/router/handler/user_handler.go` | New `LinkProvider` handler. Handle `linking_required` status in login/register OAuth responses. |
| `internal/delivery/api/router/router.go` | Register `/auth/link-provider` route. |
| `internal/infra/auth/jwt_service.go` | Add `GenerateLinkingToken` with dedicated secret and extended claims. |
| `internal/domain/service/token_service.go` | Add `TokenTypeLinking` constant and `GenerateLinkingToken` to interface. |

---

## Implementation Order

```txt
#4 Argon2id  →  #5 Account Linking  →  #2 Login Throttling  →  #3 Token Rotation  →  #1 Device Health
```

**Rationale:**

1. **#4 Argon2id first**: Zero dependencies, isolated change (swap one file + config). Gets the security foundation right before building login throttling on top.
2. **#5 Account Linking next**: Security-critical fix that touches the auth flow. Should be done before login throttling since both modify `user_service_auth.go` — doing linking first avoids merge conflicts and keeps changes sequential.
3. **#2 Login Throttling**: Depends on the auth flow but not on other Tier 1 items. The lockout push notification uses existing push infrastructure (sends to all active devices, not filtered by health status yet). After #1 is completed, the push will automatically benefit from health-based filtering.
4. **#3 Token Rotation**: Requires careful changes to the refresh flow and session management. Best done after throttling is in place (reduces risk of brute-forcing refresh tokens).
5. **#1 Device Health last**: Largest scope (schema + push pipeline + scheduled job + new API). Benefits from #2 and #3 being stable. Once deployed, the lockout and reuse-detection push notifications from #2 and #3 will automatically use health-filtered device queries.

**Note on ordering dependency**: #2's lockout push notification initially could send to all active devices before #1. Once Tier 1 ships with Device Health, the push path must use `FindDevicesByUser` with `OnlyHealthy` filter (30-day filter), making user-paused, system-invalidated, and stale-token devices ineligible for security alerts.

---

## Verification Checklist

### #1 Device Health

- [ ] `token_refreshed_at` is set on device register and updated on FCM token update.
- [ ] Push send path (`FindDevicesForUsers` in subscription repo) filters by `is_active = true AND deleted_at IS NULL AND token_refreshed_at` within 30 days.
- [ ] Devices with `token_refreshed_at` > 270 days are soft-deleted by cleanup job.
- [ ] `GET /api/v1/devices/health` returns both `id` and `client_device_id`, with computed `health_status` and `requires_rebind`.
- [ ] Device health includes soft-deleted devices so `invalid` devices can be returned for client recovery.
- [ ] Rebind flow: client calls `POST /api/v1/devices` (register) with existing `device_id` → soft-deleted device is restored, `token_refreshed_at` reset, device receives pushes again.
- [ ] `DeactivateDevice` sets `IsActive = false` (not soft-delete); device can be reactivated by re-registering.
- [ ] Scheduled cleanup job correctly identifies and processes stale/expired devices.
- [ ] `token_refreshed_at` index supports push fanout query performance.
- [ ] FCM `INVALID_ARGUMENT` only triggers device soft-delete when the outgoing payload is known to be valid; otherwise only `UNREGISTERED` is treated as token invalid.

### #2 Login Throttling

- [ ] 5 consecutive failures trigger 15-minute lockout.
- [ ] Progressive backoff: 15min → 1hr → 4hr → 24hr cap.
- [ ] Successful login resets both `failed_count` and `lockout_count`.
- [ ] 7-day decay resets `lockout_count` when no further failures occur.
- [ ] Locked accounts return generic `INVALID_CREDENTIALS` error (not a distinct lockout code).
- [ ] `Retry-After` HTTP header is present in lockout rejection responses (verified at HTTP level, not just domain error).
- [ ] Non-existing email attempts are also throttled (pre-auth spray protection).
- [ ] Push notification sent to user on lockout trigger (not on every failed attempt).
- [ ] No notification sent if user has no active devices or account doesn't exist.
- [ ] Concurrent failed login attempts update `failed_count` atomically (no lost updates).
- [ ] OAuth login paths are not affected by email/password throttling.

### #3 Token Rotation

- [ ] Refresh endpoint returns both new access token and new refresh token.
- [ ] Old refresh token is marked `is_revoked` after rotation.
- [ ] Reuse of a revoked token invalidates the entire token family.
- [ ] Push notification sent on reuse detection (security alert).
- [ ] All rotation operations (lookup, revoke, insert) are within a single transaction.
- [ ] Concurrent refresh requests are serialized via `AcquireSessionMutex()` (blocking `FOR UPDATE`).
- [ ] Client refresh implementation uses single-flight behavior and does not blindly retry timed-out refresh requests with the same refresh token.
- [ ] `FindRefreshTokensByUserID` and `CountActiveSessionsByUserID` exclude revoked tokens.
- [ ] Revoked tokens do not count toward `maxActiveSessions` limit.
- [ ] Expired and revoked tokens are cleaned up by the existing cleanup job.
- [ ] `RefreshTokenOutput` DTO includes both access and refresh tokens.
- [ ] `family_id NOT NULL` migration succeeds (table is empty pre-launch; stale test data handled).

### #5 Account Linking

- [ ] OAuth login with an email that matches an existing account does NOT auto-link.
- [ ] Login response returns `linking_required` status with a short-lived linking token (10 min TTL).
- [ ] Linking token contains `user_id`, `provider`, `provider_user_id` in JWT claims.
- [ ] `POST /auth/link-provider` is public (not under authenticated `/api/v1`) and requires valid linking token + correct existing account password.
- [ ] After successful re-auth, the provider link is created and normal login tokens are issued.
- [ ] Expired linking token returns `INVALID_TOKEN` error.
- [ ] Duplicate link request is idempotent (returns success if link already exists for same account).
- [ ] Provider already linked to a different account returns `409 PROVIDER_ALREADY_LINKED`.
- [ ] OAuth login with a new email (no existing account) still creates the account automatically.
- [ ] OAuth login with an existing provider link still logs in directly (no re-auth needed).
- [ ] Existing `LinkGoogleAccount` (post-login) flow continues to work unchanged.
- [ ] Both linking flows share the same underlying link-creation logic.

### #4 Argon2id

- [ ] Benchmark: single hash latency <500ms on Cloud Run 512Mi.
- [ ] Benchmark: 4 concurrent hash operations do not OOM on 512Mi.
- [ ] New passwords are hashed with Argon2id (memory=64MB, iterations=3, parallelism=1; adjust per benchmark).
- [ ] Hash output uses PHC string format (`$argon2id$v=19$m=65536,t=3,p=1$...`).
- [ ] Password verification works correctly with Argon2id hashes.
- [ ] `bcrypt_hasher.go` is fully removed.
- [ ] Password strength validation is preserved (unchanged interface).
- [ ] Config uses new Argon2id parameters instead of `BcryptCost`.
- [ ] Concurrency semaphore limits concurrent hash operations to configured max (default 4).
- [ ] Under concurrent load, hasher blocks gracefully without OOM on 512Mi Cloud Run.

## Implementation Status

The checklist above remains the acceptance checklist for final verification. Current backend implementation status:

- Implemented in the current Tier 1 backend change set: Device Health schema/query flow, scheduled stale-device cleanup job, Argon2id hashing, OAuth provider linking re-authentication, login throttling, refresh token rotation, token family reuse detection, and token cleanup retention.
- Still requires runtime or client verification: Cloud Run device-cleanup scheduling, Firebase delivery behavior, Argon2id benchmark on the target Cloud Run shape, client refresh single-flight behavior, and client device rebind flow.
