# Product Priority Notes

This document records the current product decisions, deferred feature priorities, and implementation order for NomNom-Radar.

## Confirmed Product Assumptions

- The product is a notification and discovery service for mobile vendors, not only food trucks.
- Mobile vendor discovery should be completed before the merchant operational feature set.
- Discovery is organized around platform-defined categories, subcategories, and hubs.
- A hub represents a market, gathering, tourism area, transit area, or other vendor aggregation point.
- Merchants may self-select from platform-defined discovery values; free-form public tags are not supported.
- Pass / redemption features are a future direction and should be supported by the discovery model, but not implemented in the discovery phase.

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
- Refresh token rotation write path: rotation stores the replacement token and then marks the old token revoked with `replaced_by` inside the same transaction. Keep this write order for clarity; revisit only if refresh throughput makes the extra round-trip measurable.
- Linking token payload exposure: linking JWT claims currently include onboarding draft fields such as `storeName`. This is acceptable for the short-lived signed-token flow, but move to server-side pending-link state if those fields are later treated as sensitive or compliance-restricted data.
- Logout idempotency audit: logout with an already revoked refresh token may revoke the same token family again. This has no additional security impact, but audit logging can be added later if session incident investigation needs it.
- Stale-device soft delete implementation: `SoftDeleteStaleDevices` uses a bulk `deleted_at` update instead of the single-row GORM `Delete` path used by `DeleteDevice`. Keep the bulk update for the cleanup job; add a short code comment or test if the difference becomes confusing during maintenance.

### Tier 2 — Mobile Vendor Discovery

Detailed implementation plan: [tier2-plan.md](./tier2-plan.md)

| # | Item | Rationale |
|---|------|-----------|
| 6 | Mobile Vendor Category Taxonomy | Discovery needs platform-defined main categories and subcategories before search pages or hub pages can be stable. The taxonomy should support mobile vendors broadly, not only food trucks. |
| 7 | Merchant Discovery Profile Fields | Merchant profiles need public discovery fields before consumers can browse or search. Include category, subcategory, public visibility, and a single active hub reference. |
| 8 | Merchant Search / Nearby Discovery | Search is the consumer entry point. Support keyword, category, subcategory, hub, and coordinate-radius filters. Default to distance sorting when coordinates are provided. |
| 9 | Category Pages | Category pages should reuse the merchant search contract so the UI does not create a parallel data path. |

Discovery interface notes:

- Use stable IDs and slugs for discovery values: `category_id`, `category_slug`, `subcategory_id`, `subcategory_slug`, `hub_id`, and `hub_slug`.
- Recommended initial main categories: `meal`, `snack`, `beverage`, `dessert`, `goods`, `experience`, `other`.
- Subcategories are platform-defined and extensible, for example `coffee`, `tea`, `fried_food`, `grill`, `bakery`, `handmade`, `accessory`, and `workshop`.
- Merchants can choose only existing platform-defined categories and subcategories.
- Search responses should include merchant summary data, discovery category data, the primary merchant location, and distance when the request includes coordinates.

### Tier 3 — Hub / Market Aggregation

| # | Item | Rationale |
|---|------|-----------|
| 10 | Hub Model and Seed Data | Hubs represent places such as markets, gatherings, tourism areas, or temporary aggregation points. They should be platform-defined rather than merchant-created. |
| 11 | Merchant Hub Selection | Merchants may select zero or one active hub from the platform-defined list. This is self-service initially; bot review can be added later. |
| 12 | Hub Pages | Hub pages list public merchants attached to the hub and support category filtering. This creates the aggregation surface needed before any pass / redemption feature. |
| 13 | Bot-assisted Hub Review | Bot review should produce suggestions or review flags only. It must not directly change the public hub assignment. |

Hub interface notes:

- Treat `Hub` as a first-class domain concept, not as a generic free-form tag.
- A hub should have `id`, `slug`, `name`, `type`, `city`, `area_name`, optional `starts_at`, optional `ends_at`, and `status`.
- Hub `type` should support at least `market`, `event`, `tourism_area`, `transit_area`, and `other`.
- Merchant search responses should return a hub object, for example `hub: { id, slug, name, type }`, instead of a plain string.
- Keep the v1 merchant rule as zero or one active hub. Future campaigns can use a separate participation table without changing that merchant-profile rule.

### Tier 4 — Merchant Operational Experience

| # | Item | Rationale |
|---|------|-----------|
| 14 | Merchant Profile Maintenance API | Merchant self-service must support discovery fields early, and can later expand into broader operational profile management. |
| 15 | Scheduled Notification / Draft / Preview | Biggest merchant operational pain point. Mobile vendors need to prepare notifications before departure or event opening. |
| 16 | Merchant Notification Analytics / Subscriber Operations View | Key to merchant retention. Merchants need to see whether notifications are working and how many subscribers they have. |

Scheduled Notification current state:

- Current notification publishing flow is immediate-send oriented.
- Relevant files:
  - [internal/delivery/api/router/handler/notification_handler.go](../internal/delivery/api/router/handler/notification_handler.go#L57)
  - [internal/usecase/impl/notification_service.go](../internal/usecase/impl/notification_service.go#L73)

### Tier 5 — Auth Completeness + Email Infrastructure

| # | Item | Rationale |
|---|------|-----------|
| 17 | Evaluate Amazon SES | Must select email infrastructure before building email-dependent features. Avoids a later migration. |
| 18 | Email Verification | Depends on email infrastructure. Required before password reset can work. |
| 19 | Forgot / Reset / Change Password | Depends on email verification. Standard self-service recovery path. |
| 20 | Session Management / Device Sign-out Center | Natural extension of refresh-token rotation work from Tier 1. |
| 21 | OAuth Nonce Verification | Google [strongly recommends](https://developer.android.com/identity/legacy/one-tap/idtoken-auth) nonce for mobile native ID-token flows to prevent replay attacks. Requires client-side changes (React Native must request nonce before initiating Google Sign-In). Deferred because it needs frontend coordination and the risk is lower than other Tier 1 auth items. |

### Tier 6 — Product Expansion

| # | Item | Rationale |
|---|------|-----------|
| 22 | Campaign / Pass / Redemption Foundation | Future Inuyama Castle town ticket-style features need campaigns, passes, partner vendors, validity windows, redemption counts, and redemption audit records. Do not implement no-op redemption endpoints before the feature is real. |
| 23 | Subscription Pause / Resume vs Unsubscribe | Worth doing to prevent subscriber loss from users who just want to mute temporarily. Semantics need to be defined first. The backend supports custom notification radius values, but the current product flow intentionally keeps the radius fixed to the default value for now. |
| 24 | Merchant Onboarding Setup Checklist | Most valuable after discovery and merchant operations are in place, otherwise the checklist has little content. |
| 25 | Post-login Google Account Linking / Unlinking UX | Separate from Tier 1 OAuth login conflict handling. Tier 1 only prevents unsafe auto-linking when an OAuth login email matches an existing account. This item is about a user-facing settings flow for already-authenticated users to add or remove Google login later. Edge case; most users choose their login method at registration time. |

Campaign / pass reservation notes:

- Discovery and hub models should make future campaigns easy to attach through `hub_id` or explicit merchant participation records.
- Future pass / redemption work should use dedicated resources such as `/api/v1/campaigns` or `/api/v1/passes` only when redemption behavior is implemented.
- Do not add placeholder redemption APIs in the discovery phase. Placeholder APIs create client coupling before redemption rules, fraud controls, and audit requirements are defined.

### Tier 7 — Deferred or Likely Unnecessary

| # | Item | Rationale |
|---|------|-----------|
| 26 | User-side Merchant Status / Current Location Snapshot | Likely already covered by notification content and discovery results. Observe whether users actually need a separate query page. |
| 27 | Direct-share QR Flow | If the existing QR scan -> merchant ID -> subscribe path works, no additional development needed. |
| 28 | Step-up Auth (TOTP MFA) | No payment or high-sensitivity data at this stage. Not needed. |
| 29 | User Notification Inbox / History | Mobile vendor notifications are time-sensitive. Yesterday's location usually has little value today. |
