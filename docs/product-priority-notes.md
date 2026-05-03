# Product Priority Notes

This document records the current product decisions, deferred feature priorities, and implementation order for NomNom-Radar.

## Confirmed Product Assumptions

- The product is a notification service for mobile merchants such as food trucks.
- Merchant discovery is not a current P0 requirement.
- The current intended acquisition path is:
  - user visits the merchant in person, or
  - user gets the merchant ID / QR from social platforms or community channels.
- Public discovery can be enabled later, after the subscription flow is considered complete enough.

## Launch Strategy

The app will go through a 30-day testing period (individual developer app store limitation) before public release. Tier 1 through Tier 3 should be completed during the testing period, so that all items are ready by the time the app is publicly available.

## Implementation Priority

### Tier 1 — Core Reliability (must complete before public release)

| # | Item | Rationale |
|---|------|-----------|
| 1 | Device Health / Rebind Flow | Push notifications failing silently is the biggest trust problem. Users have no recovery path when tokens become invalid. |
| 2 | Login Throttling / Brute-force Protection | Security baseline. Low implementation cost, high risk if missing. |
| 3 | Refresh-token Rotation | Security baseline. Without rotation, a stolen refresh token grants permanent access. |
| 4 | Password Hashing Upgrade (Argon2id) | No existing users yet, so switching from bcrypt is a one-time change with zero migration cost. Deferring would require dual-hash verification and gradual rehashing. |
| 5 | OAuth Account Linking — Require Re-auth | [Google official guidance](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token) requires re-authentication before linking an OAuth provider to an existing account. Current implementation auto-links by email match, which allows account takeover if an attacker controls an OAuth account with the victim's email. |

Device Health current state:

- Device management APIs exist for register / token update / deactivate.
- Invalid tokens are cleaned up during push processing, but there is no user-facing health or recovery flow.
- Relevant files:
  - [internal/delivery/api/router/handler/device_handler.go](../internal/delivery/api/router/handler/device_handler.go#L33)
  - [internal/delivery/worker/handler/push_handler.go](../internal/delivery/worker/handler/push_handler.go#L357)
  - [internal/usecase/impl/notification_service.go](../internal/usecase/impl/notification_service.go#L119)

Tier 1 post-implementation follow-ups:

- Login throttling decay: `checkLoginThrottle` currently calls `DecayLockoutCounts` inline. Keep the current implementation for launch, but revisit if `login_attempts` grows or login traffic becomes high; likely options are lazy per-row decay, a scheduled cleanup job, or metrics-driven throttling of the decay query.
- Provider linking reads: `resolveLinkProviderInput` currently performs read-only validation in a transaction before the write transaction in `executeProviderLinking`. This is acceptable for the low-frequency linking flow; consider removing the read transaction later if connection pressure appears. The write path remains protected by the provider unique constraint.
- Argon2id invalid hash observability: `VerifyWithContext` intentionally returns `(false, nil)` for malformed hashes to avoid leaking credential-state details. If legacy hash migration or data corruption becomes a concern, prefer an offline audit or migration check rather than logging from the password verification hot path.
- Refresh token rotation write count: rotation currently updates the old token before and after the new token is stored. This is functionally correct inside the transaction; consider combining the writes only if refresh throughput makes the extra round-trip measurable.
- Linking token payload exposure: linking JWT claims currently include onboarding draft fields such as `storeName`. This is acceptable for the short-lived signed-token flow, but move to server-side pending-link state if those fields are later treated as sensitive or compliance-restricted data.
- Logout idempotency audit: logout with an already revoked refresh token may revoke the same token family again. This has no additional security impact, but audit logging can be added later if session incident investigation needs it.
- Stale-device soft delete implementation: `SoftDeleteStaleDevices` uses a bulk `deleted_at` update instead of the single-row GORM `Delete` path used by `DeleteDevice`. Keep the bulk update for the cleanup job; add a short code comment or test if the difference becomes confusing during maintenance.

### Tier 2 — Merchant Operational Experience

| # | Item | Rationale |
|---|------|-----------|
| 6 | Scheduled Notification / Draft / Preview | Biggest merchant pain point. Food trucks need to prepare notifications before departure. |
| 7 | Merchant Notification Analytics / Subscriber Operations View | Key to merchant retention. Merchants need to see whether notifications are working and how many subscribers they have. |
| 8 | Merchant Profile Maintenance API | Basic CRUD for merchant self-service. Low cost, can be interleaved with other work. |

Scheduled Notification current state:

- Current notification publishing flow is immediate-send oriented.
- Relevant files:
  - [internal/delivery/api/router/handler/notification_handler.go](../internal/delivery/api/router/handler/notification_handler.go#L57)
  - [internal/usecase/impl/notification_service.go](../internal/usecase/impl/notification_service.go#L73)

### Tier 3 — Auth Completeness + Email Infrastructure

| # | Item | Rationale |
|---|------|-----------|
| 9 | Evaluate Amazon SES | Must select email infrastructure before building email-dependent features. Avoids a later migration. |
| 10 | Email Verification | Depends on email infrastructure. Required before password reset can work. |
| 11 | Forgot / Reset / Change Password | Depends on email verification. Standard self-service recovery path. |
| 12 | Session Management / Device Sign-out Center | Natural extension of refresh-token rotation work from Tier 1. |
| 13 | OAuth Nonce Verification | Google [strongly recommends](https://developer.android.com/identity/legacy/one-tap/idtoken-auth) nonce for mobile native ID-token flows to prevent replay attacks. Requires client-side changes (React Native must request nonce before initiating Google Sign-In). Deferred because it needs frontend coordination and the risk is lower than other Tier 1 auth items. |

### Tier 4 — Product Expansion (requires clarification first)

| # | Item | Rationale |
|---|------|-----------|
| 14 | Subscription Pause / Resume vs Unsubscribe | Worth doing to prevent subscriber loss from users who just want to mute temporarily. Semantics need to be defined first. The backend supports custom notification radius values, but the current product flow intentionally keeps the radius fixed to the default value for now. |
| 15 | Merchant Onboarding Setup Checklist | Most valuable after Tier 2 features are in place, otherwise the checklist has little content. |
| 16 | Post-login Google Account Linking / Unlinking UX | Separate from Tier 1 OAuth login conflict handling. Tier 1 only prevents unsafe auto-linking when an OAuth login email matches an existing account. This item is about a user-facing settings flow for already-authenticated users to add or remove Google login later. Edge case; most users choose their login method at registration time. |

### Tier 5 — Deferred or Likely Unnecessary

| # | Item | Rationale |
|---|------|-----------|
| 17 | User-side Merchant Status / Current Location Snapshot | Likely already covered by notification content. Observe whether users actually need a separate query page. |
| 18 | Direct-share QR Flow | If the existing QR scan -> merchant ID -> subscribe path works, no additional development needed. |
| 19 | Step-up Auth (TOTP MFA) | No payment or high-sensitivity data at this stage. Not needed. |
| 20 | User Notification Inbox / History | Food truck notifications are time-sensitive. Yesterday's location has no value today. |
| 21 | Merchant Discovery / Nearby Search | Intentionally deferred roadmap decision. Current acquisition is offline + social. Revisit after subscription flow is mature. |
