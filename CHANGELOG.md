# Changelog

User-visible changes and release-operation requirements are recorded here.
Stable versions use `## [X.Y.Z] - YYYY-MM-DD` headings and matching `vX.Y.Z`
Git tags. A version entry identifies content, not a successful prod deployment.

## [Unreleased]

Initial documented backend baseline, assembled from the current
[implementation status](docs/roadmap.md) and [architecture](docs/architecture.md).
A release version and date have not yet been assigned.

### Added

- Email/password registration and sign-in, Google mobile ID-token sign-in,
  merchant onboarding, and provider linking after existing-account
  re-authentication.
- Session management, refresh-token rotation, logout, Argon2id password hashing,
  and login throttling.
- Configurable in-process rate limiting for credential/OAuth, refresh/logout,
  and authenticated API routes. Browser clients use explicit CORS origins;
  wildcard origins are rejected.
- Consumer profile and location APIs, merchant subscriptions, and QR-based
  subscription to merchants.
- Device registration, push-token updates, health reporting, and rebind support
  for stale or invalid devices. Stale-device cleanup runs as a Cloud Run Job;
  push delivery filters unhealthy or inactive device records.
- Merchant verification, location and menu management, QR generation, discovery
  profiles, location-notification publishing, and merchant notification history.
- Platform-defined categories, subcategories, and hubs, with authenticated
  consumer search over publicly visible merchants by category, hub, keyword,
  and nearby location.
- Asynchronous location notifications through Pub/Sub and a geo worker, with
  Firebase Cloud Messaging delivery, PostGIS subscriber filtering, PMTiles
  route-aware distance checks, and Haversine fallback. Local HTTP publishing
  supports development without Pub/Sub.
- Shared API response envelopes with request correlation IDs and client-safe
  errors.
- PostgreSQL/PostGIS storage, Supabase pre/shared/post migrations, and Cloud Run
  service/job deployment and operational workflows.
- Failed-release retries using retained nonsecret release plans and fixed image
  digests, plus manual stable-version image tagging without rebuilding or
  deploying applications.

### Changed

- Simplify candidate publication and Cloud Run releases around GitHub Actions,
  exact image digests, and verified build attestations. Candidate tags use seven
  SHA characters while existing full-SHA tags remain supported.
- Make migrations an explicit, disabled-by-default release option. Selected
  candidates supply SQL and deployment templates; current release tooling and
  environment configuration drive execution.
- Allow prod deployment without a version tag and independently of dev's
  currently deployed version.

### Release Notes

- Initial environment setup requires the existing database migrations. For
  later releases, operators explicitly enable pending migrations and confirm
  that application rollback is compatible with the current schema. See
  [migration and recovery procedures](docs/operations.md).
- Registry cleanup remains unchanged. Recovery requires retained images and
  valid provenance; release-plan artifacts are retained for 30 days.
- The maintainer has confirmed functional validation of the APIs exposed to the
  app and the app itself. Current validation status is recorded in the
  [roadmap](docs/roadmap.md#verification-status).
- The revised CI/CD workflows still require hosted CI and live release/retry
  acceptance, tracked separately in
  [operations](docs/operations.md#prerequisites-and-readiness).
