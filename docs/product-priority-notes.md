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

Device Health current state:
- Device management APIs exist for register / token update / deactivate.
- Invalid tokens are cleaned up during push processing, but there is no user-facing health or recovery flow.
- Relevant files:
  - [internal/delivery/api/router/handler/device_handler.go](../internal/delivery/api/router/handler/device_handler.go#L33)
  - [internal/delivery/worker/handler/push_handler.go](../internal/delivery/worker/handler/push_handler.go#L357)
  - [internal/usecase/impl/notification_service.go](../internal/usecase/impl/notification_service.go#L119)

### Tier 2 — Merchant Operational Experience

| # | Item | Rationale |
|---|------|-----------|
| 5 | Scheduled Notification / Draft / Preview | Biggest merchant pain point. Food trucks need to prepare notifications before departure. |
| 6 | Merchant Notification Analytics / Subscriber Operations View | Key to merchant retention. Merchants need to see whether notifications are working and how many subscribers they have. |
| 7 | Merchant Profile Maintenance API | Basic CRUD for merchant self-service. Low cost, can be interleaved with other work. |

Scheduled Notification current state:
- Current notification publishing flow is immediate-send oriented.
- Relevant files:
  - [internal/delivery/api/router/handler/notification_handler.go](../internal/delivery/api/router/handler/notification_handler.go#L57)
  - [internal/usecase/impl/notification_service.go](../internal/usecase/impl/notification_service.go#L73)

### Tier 3 — Auth Completeness + Email Infrastructure

| # | Item | Rationale |
|---|------|-----------|
| 8 | Evaluate Amazon SES | Must select email infrastructure before building email-dependent features. Avoids a later migration. |
| 9 | Email Verification | Depends on email infrastructure. Required before password reset can work. |
| 10 | Forgot / Reset / Change Password | Depends on email verification. Standard self-service recovery path. |
| 11 | Session Management / Device Sign-out Center | Natural extension of refresh-token rotation work from Tier 1. |

### Tier 4 — Product Expansion (requires clarification first)

| # | Item | Rationale |
|---|------|-----------|
| 12 | Subscription Pause / Resume vs Unsubscribe | Worth doing to prevent subscriber loss from users who just want to mute temporarily. Semantics need to be defined first. Notification radius remains intentionally fixed to the default value for now. |
| 13 | Merchant Onboarding Setup Checklist | Most valuable after Tier 2 features are in place, otherwise the checklist has little content. |
| 14 | Google Account Linking / Unlinking | Edge case. Most users choose their login method at registration time. |

### Tier 5 — Deferred or Likely Unnecessary

| # | Item | Rationale |
|---|------|-----------|
| 15 | User-side Merchant Status / Current Location Snapshot | Likely already covered by notification content. Observe whether users actually need a separate query page. |
| 16 | Direct-share QR Flow | If the existing QR scan -> merchant ID -> subscribe path works, no additional development needed. |
| 17 | Step-up Auth (TOTP MFA) | No payment or high-sensitivity data at this stage. Not needed. |
| 18 | User Notification Inbox / History | Food truck notifications are time-sensitive. Yesterday's location has no value today. |
| 19 | Merchant Discovery / Nearby Search | Intentionally deferred roadmap decision. Current acquisition is offline + social. Revisit after subscription flow is mature. |
