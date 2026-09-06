# Roadmap

This document is the source of truth for current implementation status, remaining verification risks, and decided next directions. Product scope and long-term principles belong in `docs/product.md`; historical files are background only.

## Current V1 Direction

The product definition and complete v1 scope live in `docs/product.md`. Current backend delivery is aligned around reliable notifications plus discovery:

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

## Verification Status

The maintainer has confirmed functional validation of the APIs currently exposed
to the app and the app itself. App/API functional acceptance is no longer an
outstanding v1 milestone.

The revised CI/CD workflows require separate hosted CI and live release/retry
acceptance; see [operations readiness](operations.md#prerequisites-and-readiness).

Environment-specific operational evidence, such as cleanup scheduling,
Argon2id benchmarks on the target Cloud Run shape, and production PMTiles data
availability, is separate from app/API functional acceptance. This confirmation
does not record individual results for those checks.

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

Electronic passes and redemption are not scheduled for implementation. The product constraints for any future market-partnership work remain in `docs/product.md`; a separate product decision is required before this becomes active roadmap scope.

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
