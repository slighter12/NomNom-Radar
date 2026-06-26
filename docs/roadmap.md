# Roadmap

This document is the current product and engineering status source. Historical plan files are background only.

## Current V1 Direction

V1 is reliable notifications plus discovery:

- Consumers can subscribe to merchants, manage device/location state, and discover publicly visible merchants through authenticated APIs.
- Merchants can maintain lightweight operational data, publish location notifications, and appear in discovery when eligible.
- The backend supports route-aware notification delivery, device health, and discovery search.

## Completed Backend Foundation

Completed or substantially implemented backend areas:

- Email/password auth, Google OAuth ID-token flow, onboarding, provider linking, session management, refresh-token rotation, and login throttling.
- Argon2id password hashing.
- Device registration, token updates, device health, stale-device cleanup job, and rebind support.
- Merchant verification, merchant locations, menu management, QR subscription, and notification history.
- Platform-defined discovery categories, subcategories, hubs, merchant discovery profile, and authenticated consumer search over publicly visible merchants.
- Async geospatial notification flow with Pub/Sub/local publisher, geo worker, PMTiles routing, and Haversine fallback.
- Supabase/PostgreSQL migration support and Cloud Run service/job deployment workflows.

## Remaining V1 Verification

These are verification or integration risks, not new product scope:

- Cloud Run device-cleanup scheduling in each target environment.
- Firebase push delivery behavior with real app credentials and real invalid-token responses.
- Client device rebind flow for stale or invalid devices.
- Client refresh-token single-flight behavior.
- Argon2id benchmark on the target Cloud Run shape.
- PMTiles source, road layer, and production route data availability.

## Next Product Directions

### Hub and Market Discovery

Near-term product direction after v1 should continue strengthening hubs and markets:

- Hub pages that reuse merchant search.
- Better hub seed data and platform operations for maintaining active hubs.
- Bot-assisted hub review as suggestions or review flags only, not automatic public assignment changes.

### Merchant Operations

Merchant operations should evolve only from user feedback. Candidate areas:

- Scheduled notification, draft, and preview workflow.
- Notification analytics and subscriber operations view.
- Merchant onboarding checklist.

Do not turn the backend into a full POS, CRM, or campaign-management product without a separate decision.

### Market Partnerships and Passes

Electronic passes and redemption are future market-partnership directions. Keep them out of v1 implementation:

- Future campaigns or passes should attach to stable hub identity or explicit participation records.
- Redemption must have clear validity windows, fraud controls, and audit records before API work starts.
- Do not add placeholder redemption endpoints.

### Auth and Account Completeness

Deferred account features:

- Email verification and password reset after email infrastructure is selected.
- Session/device sign-out center.
- OAuth nonce verification when the mobile client can coordinate nonce generation.
- Post-login provider linking/unlinking UX.

## Deferred or Likely Unnecessary

These should stay deferred unless user feedback proves demand:

- User-side merchant status/current-location snapshot outside discovery and notifications.
- Direct-share QR flow beyond the existing QR scan to merchant subscription path.
- TOTP MFA before payment or high-sensitivity workflows exist.
- User notification inbox/history for time-sensitive mobile-vendor notifications.
