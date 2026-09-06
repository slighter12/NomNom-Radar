# Operations

This document owns local operation, candidate release, promotion, migration,
and operational workflow policy. Focused references own only target-specific
commands and settings and link here for shared semantics.

## Runtime Units

- `cmd/radar`: main API service.
- `cmd/geoworker`: Pub/Sub/local HTTP push worker.
- `cmd/device-cleanup`: scheduled Cloud Run Job for stale device cleanup.

## Local Development

Use `config/config_demo.yaml` as the local configuration template:

```sh
cp config/config_demo.yaml config/local.yaml
ENV=local go run ./cmd/radar
```

Run the local geo worker through Docker Compose when testing async delivery:

```sh
docker compose --profile dev up --build geoworker
```

Runtime PMTiles routing uses the `pmtiles` config block. When it is disabled or
unavailable, routing falls back to straight-line Haversine behavior.

## PMTiles Data Preparation

Prepare road PMTiles outside git and provide them through deployment storage
or local bind mounts.

```sh
brew install osmium-tool tippecanoe
osmium tags-filter taiwan-latest.osm.pbf w/highway -o filtered-roads.osm.pbf --overwrite
osmium export filtered-roads.osm.pbf -o roads.geojson --overwrite
tippecanoe -o map.pmtiles -z15 -Z15 --buffer=100 --no-clipping --layer=transportation roads.geojson
```

Set `pmtiles.source`, `pmtiles.roadLayer`, and `pmtiles.zoomLevel` to match.
Do not commit generated PMTiles or intermediate OSM/GeoJSON files.

## Focused References

- Cloud Run release and operations workflows are under `.github/workflows/`;
  service manifests are under `deploy/cloud-run/`.
- Cloud Run Job deployment and scheduling are in
  [`docs/reference/cloud-run-jobs.md`](reference/cloud-run-jobs.md).
- Shared HTTP response envelopes are in
  [`docs/reference/api-conventions.md`](reference/api-conventions.md).
- Google OAuth mobile ID-token behavior is in
  [`docs/reference/google-oauth-api.md`](reference/google-oauth-api.md).
- Device health and rebind behavior is in
  [`docs/reference/device-health-api.md`](reference/device-health-api.md).

## Database Migrations

### Local PostgreSQL

Start the repository PostgreSQL/PostGIS service, install goose, and apply or
inspect shared migrations:

```sh
docker compose up -d postgres-master
make db-postgres-install-goose
make db-postgres-up POSTGRES_PORT=5432
make db-postgres-status POSTGRES_PORT=5432
```

The Makefile builds the DSN from `POSTGRES_HOST`, `POSTGRES_PORT`,
`POSTGRES_DB_NAME`, `POSTGRES_DB_USER`, `POSTGRES_DB_PASSWORD`, and
`POSTGRES_SSLMODE`. Pass `PG_URI` to use a complete DSN. Only shared PostgreSQL
migrations have a local apply target.

### Deployment

Migration execution is an explicit release input, `run_migrations`, defaulting
to `false`. When disabled, the workflow skips Go setup, Goose installation,
migration-secret retrieval, and database connections. There is no automatic
baseline/schema inference or SQL-diff warning. The operator checks the changelog
and decides whether the selected release needs migrations.

When enabled, current release tooling applies the selected candidate's SQL in
this order, letting Goose track already-applied versions:

1. `database/migration/supabase/pre/`
2. `database/migration/postgres/`
3. `database/migration/supabase/post/`

Specifying a SHA or version does not change this policy. A failed migration can
be retried with the same candidate and the input enabled; successfully applied
versions are not rerun. Application rollback leaves the database forward-only.
The operator confirms compatibility with the current schema before deploying
older images.

The dedicated GCP Secret Manager secret `postgres-migration-dsn` is required.
There is no fallback to `postgres-master-dsn`. Supabase migrations must use a
direct or Supavisor session-mode connection on port `5432`; transaction-pooler
port `6543` is rejected. Runtime services may continue using
`postgres-master-dsn` with `POSTGRES_PRESET=supabase_transaction`.

Add a migration instead of rewriting one applied to a shared environment.
Never run `DROP EXTENSION postgis CASCADE` on a migrated database.

## Release Flow

### Candidate images and attestation

Release-impacting merges to `main` publish `radar`, `geoworker`, and
`device-cleanup` to the dev Artifact Registry. New candidate tags use the first
seven hexadecimal characters of the commit SHA; internal identity and the
`release-sha` label retain the full SHA. Existing full-SHA tags remain valid.
Ambiguous prefixes, short-tag collisions, and conflicting short/full tags fail
closed. Documentation-only main pushes do not rebuild images; a new version request builds
its exact commit when images are missing, including changelog-only commits.

CI remains in `ci.yml`, also callable by Version Images as a reusable workflow,
preserving the attestation signer used by older images.
Each target is built through `docker/build-push-action` to a run-scoped staging
tag. CI attests the action's exact digest, verifies it, and only then adds the
short SHA tag. Staging tags expire under the existing cleanup policy. Existing
valid images are reused; interrupted publication can build missing targets.
An existing tag without valid provenance is rejected rather than re-attested.
All three targets must have valid images before a candidate can be released.

### Tag mutability and retention

Neither registry enables Artifact Registry's `--immutable-tags`: that setting
would block deletion of tagged images and conflict with existing cleanup.
Attestation binds each digest to this repository, `ci.yml`, and the complete
candidate commit. Tag text alone is never sufficient evidence. Automation
refuses to replace an existing candidate or version tag with different content.

Stable `vX.Y.Z` tags in the prod registry use the existing GCP version-retention
rule. Untagged-by-version releases remain subject to current cleanup. Recovery
is available only while the exact images and valid provenance remain retained;
retention is not changed by these workflows. See the
[attestation decision](adr/0001-attested-candidate-images.md) for the staging
and credential threat model.

### Workflow ownership

Actions owns triggering, permissions, authentication, tool setup, conditions,
and stage order. A shared local composite action owns digest copying and tag
publication for prod promotion and version preservation. Dev deployment never
calls that action. Callers provide an exact image mapping and an explicit JSON
array of tags; deployment supplies only the short SHA, while Version Images adds
the stable version after creating its Git tag. A plan's version is
selection metadata, including in historical retry plans, and never authorizes
publication. The workflow visibly owns request selection, CI, preparation, and tagging.
Shared data helpers resolve remote tag commits and exact-SHA image mappings.
Simple CLI calls stay in named steps; no gcloud error-text parser is used. When existence matters,
a successful scoped JSON list is matched exactly; a failed query fails the step.
The target catalog lists the three target names; deployment order is
explicit in the release workflow. Scheduler lookup produces an existence output;
separate create/update steps use Actions conditions.

Selected manifests and SQL are checked out by `actions/checkout` into a separate
directory at the resolved full candidate SHA. Kustomize prepares the manifests;
the pinned yq action applies current configuration and validates all targets
before publication or deployment. YAML expressions are shared with local tests,
including the 100% latest-revision traffic policy for older templates. Rendered
files stay private to the runner and are removed at the end of the release.

Focused release tests require Bash, Git, jq, kubectl (with Kustomize), and yq
v4.53.6. Run `bash .github/scripts/release/release_test.sh`. CI obtains the test
yq binary from the same digest-pinned action image used for rendering; Ruby and
Go are not needed for these checks. The application CI still uses Go.

External actions are pinned to full commit SHAs; tool versions follow their
purpose. Govulncheck uses the official action's latest scanner, while
golangci-lint and actionlint also track latest tool releases. These checks remain
required: updated findings or tool failures can fail CI, with no automatic
fallback to an older tool. GitHub supports concurrency `queue: max`, but the
current actionlint schema does not. CI excludes only its unknown-`queue`
diagnostic; release semantic tests validate queue values and shared
serialization. Remove that narrow exception when actionlint supports the key.
The scanner action reuses the existing checkout and
selects Go from `go.mod`, overriding its default `stable` selection.

Keep the existing gcloud latest default, Buildx/BuildKit defaults, and
`ubuntu-latest` runners. Go setup continues to follow `go.mod`. Keep yq, gcrane,
and Goose pinned because they render deployment data, copy release content, or
execute SQL; validate behavior when upgrading them. Published applications
remain identified by exact image digests, independently of CI tool updates.

### Release Cloud Run

Dispatch `Release Cloud Run` from current remote `main`:

| Input | Contract |
|-------|----------|
| `environment` | Required: `dev` or `prod`. |
| `release_ref` | SHA (at least seven characters) or stable `vX.Y.Z`; required for prod unless retrying. Empty for dev selects the newest compatible candidate. |
| `run_migrations` | Default `false`; explicitly enables all migration phases. |
| `retry_run_id` | Optional failed release run; mutually exclusive with `release_ref`. |

A supplied SHA must resolve uniquely to an ancestor of current `main`. A version
Git tag must have its matching dated changelog heading and resolve to images
for that exact commit. Dev's
automatic selection examines at most 50 first-parent commits and does not cross
release-impacting changes. Missing or invalid attestation stops resolution;
publishing a fresh candidate is preferable to an unbounded search.

The current main commit supplies workflow and helper code (`CONTROL_SHA`). The
selected candidate supplies deployment templates and migration SQL
(`RELEASE_SHA`). Environment variables and secrets use their current values;
rollback does not restore their historical values. Incompatible old templates
stop before deployment. Select a new dispatch for every retry; GitHub's rerun
button is rejected so recovery executes current automation.

Prod eligibility is an operator decision: confirm dev testing and schema
compatibility before dispatch. Automation does not require dev to currently run
the candidate and does not maintain a historical dev qualification system.
Prod first uses retained valid prod images, otherwise copies exact verified dev
digests without rebuilding. Dev tags need not survive for a retained prod image
to be used.

A release performs these stages:

1. Resolve and verify all three digests and required configuration, then render
   all selected-version templates before changing the environment.
2. Upload a nonsecret release-plan Actions artifact with the full SHA, digests,
   environment, and migration choice; retain it for 30 days. Never upload
   rendered templates, DSNs, or secret values.
3. Dev performs no registry mutations, even for legacy full-SHA-only images.
   For prod, copy missing exact digests, verify destinations, and establish
   missing short-SHA tags. Deployment never publishes version image tags,
   including when a version selects content or remains in a retry plan.
4. If requested, prepare migration tools, read the DSN using the Secret Manager
   action with masked output, and run pre/shared/post. With migrations disabled,
   skip all migration setup, DSN access, and database operations.
5. Deploy `geoworker`, `device-cleanup`, then `radar` through the official Cloud
   Run deploy action. Deploying cleanup does not execute the job.
6. Verify full release labels and exact digests, service readiness and actual
   traffic routing, and Radar `/health` using the deploy action's URL output. This is deployment verification, not
   end-to-end notification or database business acceptance.
7. Write a summary with candidate SHA, exact digests, migration selection, and
   recovery information. Step results are available in the Actions UI.

### Retry and rollback

Use a new current-main dispatch with `retry_run_id` to recover a failed,
cancelled, or timed-out main `workflow_dispatch` run of this repository's release
workflow. The artifact must belong to that run and match the target environment.
The workflow revalidates image provenance and availability, then uses the
original full SHA and digests without selecting a newer candidate. Choose
`run_migrations` anew; the summary displays the original and current choice.
If the artifact has expired or was never uploaded, fixed-plan retry is
unavailable; explicitly select and verify a release reference instead.

A rollback is an explicit release of older retained images and their templates.
Confirm current configuration and schema compatibility; migration execution
remains an independent choice and never performs Goose `down`. Partial
application deployment is recovered by converging all three targets, not by an
automatic database rollback.

If an existing candidate tag has missing or invalid attestation, quarantine the
SHA and publish a fresh release-impacting commit. The candidate identity has
`artifactregistry.tags.create` and `.update`, but not `.delete`. Removing a bad
tag requires registry-admin authority and explicitly approved break-glass
handling. Never manually attest an unknown image.

### Manual versions and changelog

Maintain [`CHANGELOG.md`](../CHANGELOG.md) in English. Move intended notes from
`Unreleased` into `## [X.Y.Z] - YYYY-MM-DD` and merge to main. Dispatch **Version
Images** from main with the intended canonical `vX.Y.Z`. Versions and changelogs
remain operator-maintained; do not push a Git tag to initiate publication.

A new request fixes the dispatch main SHA. It inspects exact-SHA prod images
before dev, runs its own checks through reusable Go CI, and builds only missing
targets. A changelog-only commit requires its own images. All three verified
images and SHA tags are saved to prod before a lightweight Git tag is created;
stable image tags follow. This workflow never deploys or runs migrations.

Version Images uses the existing `prod` Environment, its approval rules,
`GCP_RELEASE_SA_KEY`, and registry variables. Request inspection and final
publication both use this Environment and may require approval as jobs become
eligible. No additional Environment, service account or secret is needed.
The shared candidate job still uses the existing dev candidate credential.
Only the final publication job has GitHub contents write permission for Git tags.

Before a Git tag exists, a failed attempt may be restarted with the same version
and the then-current main SHA. Once the tag exists, dispatch the same version to
retry its fixed commit. Lightweight and annotated tags are both supported; an
annotation is not interpreted as an application data format. Existing verified
SHA images are required for recovery: missing images, failed queries or
conflicting version tags fail instead of triggering a rebuild or moving the tag.
Partially published image tags are completed, and matching tags are not rewritten.
There is no independent durable digest record; do not move or rewrite SHA/version
tags outside the publication workflows. If recovery cannot establish consistent
existing content, it stops for operator investigation.

The final publication job holds the shared release lock, rechecks Git tags and
images after waiting, and rejects a competing request selecting a different SHA.
A competing request that already published the same SHA can be verified and
completed. Git and registry writes are separate operations, so interrupted
publication may leave a Git tag awaiting image tagging; use a fresh dispatch to
recover with current automation. Quality checks and builds do not hold this lock.

Deployment selecting a version also requires images for its exact tag commit.
Older tags that previously selected a compatible ancestor must instead be deployed
by the explicit candidate SHA; automatic historical lookup remains only for
`dev latest`. Existing full-SHA image tags remain supported.

Success summaries list the version, full Release SHA and all three prod digest
references, explicitly stating no deployment or migration occurred. Actions status
and existing notifications report failures. Retention policies remain unchanged;
missing content cannot be reconstructed by recovery. See the
[exact-commit decision](adr/0002-version-images-match-tag-commit.md).

### Review and operations

Merge review controls what reaches main; deployment review is the prod
Environment's separate required-reviewers pause. With a single maintainer and
self-review permitted, this is deliberate confirmation, not separation of
duties. Confirm the dispatch SHA is current remote main during the pause;
a historical workflow may predate the workflow's own guard. Version publication deliberately shares the existing prod approval gate;
do not remove its reviewers to bypass that approval.

Release, version marking, and operations share the `cloud-run-release`
concurrency group across environments with `queue: max` and
`cancel-in-progress: false`. Up to 100 pending requests wait without replacing
each other; excess requests are cancelled visibly and require a new dispatch.
Version Images, deployment and operations acquire this group at publication
or operation job level. Version checks and candidate builds do not hold it. Candidate construction uses a separate
SHA/target group with the same queue policy, shared by both CI entry points.
Ordering follows when jobs or workflows begin waiting, not necessarily event order. A
preservation request can wait behind active release/operations work; separation
from deployment review does not remove this shared serialization.
See [GitHub concurrency limits](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency).

The shared group serializes cooperating workflows, not external registry
writers or cleanup. List-then-add is not atomic. Do not run external tag writers
concurrently; obtain explicit break-glass authorization when needed. Conflict
preflight and final verification do not claim to provide registry transactions.

`Cloud Run Operations` separately supports:

- `execute-device-cleanup`
- `configure-device-cleanup-scheduler`
- prod-only `sync-cloudflare-origin-secret`

Operations dispatches are limited to current main. Reruns are permitted while
main is unchanged; otherwise dispatch anew after reviewing configuration. The
scheduler remains `device-cleanup-daily` with default schedule `0 3 * * *` and
time zone `Asia/Taipei`. Operations does not publish images, deploy targets, or
run migrations.

## Identities and Configuration

The current phase uses separate role-specific JSON keys:

| Role | GitHub secret | Scope |
|------|---------------|-------|
| Candidate publisher | `GCP_CANDIDATE_SA_KEY` repository secret | Publish/read dev candidate images; no deploy, migration, operations, or Cloudflare authority. |
| Release deployer | `GCP_RELEASE_SA_KEY` in each Environment | Read candidates, deploy all resources, and migrate the target; prod also reads dev and writes its own registry. |
| Operations operator | `GCP_OPERATIONS_SA_KEY` in each Environment | Execute cleanup and manage its scheduler; no image, deployment, or migration authority. |

Candidate variables are `GCP_DEV_PROJECT_ID`, `GCP_DEV_REGISTRY`, and
`GCP_DEV_REGION`. Each Environment provides `GCP_PROJECT_ID`,
`GCP_PROJECT_NUMBER`, `GCP_REGION`,
`GCP_REGISTRY`, `GCP_SA_EMAIL`, `GCP_SCHEDULER_SA_EMAIL`,
`GOOGLEOAUTH_CLIENTID`, `HTTP_ALLOWEDHOST`, and
`HTTP_CORSALLOWEDORIGINS`, `HTTP_RATELIMIT_ENABLED`,
`HTTP_RATELIMIT_RATE`, `HTTP_RATELIMIT_BURST`, and
`HTTP_RATELIMIT_EXPIRESIN`.

`HTTP_CORSALLOWEDORIGINS` is a comma-separated list of complete browser
origins such as `https://app.example.com`. Wildcards are rejected. An empty
value disables CORS instead of allowing every origin. Cloud Run releases may
leave this variable empty when no browser cross-origin client is required;
browser clients must otherwise be configured explicitly in each environment.

Credential authentication endpoints and `/oauth/*` use one configurable
in-process Echo rate limiter. It covers registration, login, onboarding, and
provider-linking routes under `/auth`, plus OAuth callbacks. The defaults are
10 requests/second, a burst of 30, and three-minute visitor cleanup. The
limiter is keyed by the client IP, is local to each Cloud Run instance, and is
not a global distributed limiter. `HTTP_RATELIMIT_ENABLED=false` disables it
explicitly. In Cloudflare-backed environments the client IP comes from the
authenticated `CF-Connecting-IP` header; direct environments ignore forwarded
IP headers and use the network peer address. This application-layer limiter
does not replace a Cloudflare-wide rate-limiting rule.

`/auth/refresh` and `/auth/logout` use a separate configurable in-process Echo
session rate limiter. When the refresh token is valid, it is keyed by the user
ID in that token; invalid or missing tokens fall back to the client IP. Its
defaults are 2 requests/second, a burst of 20, and three-minute visitor
cleanup. It is local to each Cloud Run instance and is intended to contain
runaway clients rather than replace credential lockout or a Cloudflare-wide
rate-limiting rule. This phase does not add separate environment-variable
overrides.

Authenticated `/api/v1/*` endpoints use a separate in-process Echo rate limiter
keyed by the authenticated user ID. Its defaults are 20 requests/second, a
burst of 60, and three-minute visitor cleanup. If the authentication context is
missing, the middleware defensively falls back to the client IP. The limiter is
local to each Cloud Run instance and does not replace a Cloudflare-wide
rate-limiting rule. Its defaults are used when no API-specific configuration is
provided; this phase does not add separate environment-variable overrides.

`GCP_SA_EMAIL` is the Cloud Run runtime identity.
`GCP_SCHEDULER_SA_EMAIL` must be a different identity, and its permission to
invoke Cloud Run must be limited to `device-cleanup` alone. Nothing in this
repository grants that permission, so the scope has to be established outside
it.

The prod GitHub Environment continues to hold `CLOUDFLARE_API_TOKEN` and
`CLOUDFLARE_ORIGIN_SECRET`; `CLOUDFLARE_ZONE_ID` and
`CLOUDFLARE_TRANSFORM_RULE_ID` remain Environment variables. The origin secret
is rendered into Radar and used by the operations workflow. It must not appear
in repository files, workflow inputs, or logs, and must not contain CR or LF.

Cloud Run receives the origin secret as a plaintext environment value. Actors
who can inspect service or revision configuration can retrieve it, and older
revisions retain earlier values. This is an accepted current risk: restrict
describe permissions, rotate GitHub and Cloudflare values together after
suspected exposure, and remove obsolete revisions after verified releases.

Existing keys, variables and GCP secrets remain in their current locations.
Version publication reuses the prod release identity; its registry authority is
already needed for promotion. No new key, service account or Environment is
required. OIDC and Workload Identity Federation remain deferred.

### Live version acceptance

- [ ] Confirm prod approval and existing registry variables/credential work for
  both inspection and publication jobs; do not change Environment settings.
- [ ] Verify the publication token can create Git tags under existing tag rules.
- [ ] Dispatch a new version on main. Confirm checks and missing-image builds
  use the exact SHA, including a changelog-only source commit.
- [ ] Verify provenance from main CI and reusable CI, including cross-registry
  verification after copying into prod.
- [ ] Confirm failures before Git tag creation leave the version available for
  a new request; interrupt after creation and retry using existing SHA images.
- [ ] Verify all three SHA/version image tags and the summary, repeat without tag
  writes, and exercise competing requests. No deployment or migration occurs.
- [ ] Confirm retained prod content remains deployable when dev is unavailable.
  Do not delete images or change access to simulate cleanup without authorization.

Local tests cannot prove live IAM, review gates, attestation claims or cleanup.
Existing registry retention is unchanged. External registry writers are outside
GitHub serialization; list-then-add is not an atomic registry transaction.

## Prerequisites and Readiness

The workflow files alone do not make the system production-ready. Complete and
record this rollout before declaring readiness:

- [ ] Dev dispatches are restricted to current `main`; each Environment
  contains the correct non-overlapping secrets and variables, including the
  numeric `GCP_PROJECT_NUMBER` required by the Cloud Run Job manifest.
- [ ] The prod Environment retains the single-maintainer confirmation pause,
  and the operator verifies the dispatch SHA against current remote main.
- [ ] Candidate, release, operations, runtime, and scheduler identities exist
  with the scopes above.
- [ ] Neither Artifact Registry repository enables immutable tags, and the
  posture in "Tag mutability and retention" above still holds. CI leaves staging
  tags for cleanup, including those from interrupted publication; retention
  bounds their lifetime unless the image also carries a retained version tag. If
  retention is ever changed to keep versions indefinitely, revisit this.
- [ ] Required Google APIs are enabled and all three resources implement the
  `release-sha` label contract.
- [ ] Dev latest after a documentation-only commit selects the
  newest compatible ancestor without rebuilding; a release-impacting commit
  without a complete candidate fails closed and is recovered by publishing a
  new candidate.
- [ ] Explicit SHA, old full-SHA tags, version tags, retained-prod recovery,
  and failed-run artifact retry resolve the intended attested digests.
- [ ] Disabled migrations skip tool setup, secret retrieval, and DB access;
  enabled migrations use selected-version SQL and the documented phase order.
- [ ] Version tagging tolerates partial completion without overwriting content;
  actual GCP retention and cross-registry provenance have been checked.
- [ ] `postgres-migration-dsn` is release-only and verified as the intended
  direct/session-mode port `5432` database.
- [ ] Prod Cloudflare secrets and identifiers match the deployed origin rule;
  the origin secret is absent from repository content and logs.
- [ ] `device-cleanup-daily` uses the approved schedule and the separate
  scheduler caller can invoke only the cleanup job.
- [ ] Acceptance covers candidate publication, dev release, same-SHA partial
  retry, exact prod promotion, migration failure, scheduler execution, and
  Cloudflare synchronization.

Until GitHub/GCP configuration and live acceptance are complete, release
readiness remains unverified.

## Manual Break-glass Recovery

Break-glass is not a second normal deployment path. Obtain approval, record the
operator and reason, confirm no release or operation is active, and preserve
attestation evidence, current labels, and running digests.

1. Inspect all three resources. Record actual labels, digests, and traffic
   rather than trusting one target.
2. Keep the database forward-only; never run goose `down` or rewrite applied
   migrations. Use only `postgres-migration-dsn` and the documented phase order.
3. Retry the same SHA and exact attested digests first. Never substitute
   `latest` or rebuild while recovering a partial release.
4. Holding the application on older exact digests requires retained
   attestation, schema compatibility evidence, and explicit incident approval;
   it is never a database rollback.
5. Restore the workflow path, converge all resources, verify labels, digests,
   readiness, and `/health`, then record and reconcile out-of-band actions.

Stop and escalate when exact digests or compatibility evidence are unavailable.

## Configuration Notes

Important runtime areas include `postgres`, `secretKey`, `googleOAuth.clientId`,
`auth`, `loginThrottle`, `firebase`, `pubsub`, `pmtiles`, and `deviceCleanup`.
Prefer environment overrides and Secret Manager for deployed secrets. Do not
commit local credentials.

## Operational Checks

Before notification/device-health changes, confirm Firebase, Pub/Sub, PMTiles,
the cleanup image, and any intentional scheduler change. Before schema changes,
confirm the migration secret points to the intended database, uses direct or
session-mode port `5432` for Supabase, and every migration is compatible with
both prior and new application revisions.
