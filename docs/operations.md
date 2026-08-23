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

The release compares the target environment's consistent baseline to the
selected immutable `main` candidate. Changed directories run in this order:

1. `database/migration/supabase/pre/`
2. `database/migration/postgres/`
3. `database/migration/supabase/post/`

`radar`, `geoworker`, and `device-cleanup` use the `release-sha` Cloud Run
label. A normal release requires one valid baseline SHA shared by all three
resources and ancestral to the selected candidate. A pinned release requires
the selected SHA to be an ancestor of current `main`; it permits a forward or
rollback move only when the current baseline and selected SHA are comparable
ancestors of current `main`. An unpinned release to a new environment with no
resources bootstraps by checking every migration phase. A partial retry is
accepted only when labels contain that consistent baseline and the current
target, or when missing/unlabeled resources are paired only with the target.

Pinned releases do not run migrations. Because the schema is forward-only, a
pin that sits on the other side of a migration change from the running
baseline would put old code in front of a schema it has never seen. The
release refuses that case: `migration-phases` compares
`database/migration/**` between the pin and the baseline and fails closed
unless the dispatch also sets `acknowledge_schema_drift`. Handle the schema
first, then re-dispatch with the acknowledgement. A pin that crosses no
migration change needs no acknowledgement.

Divergent history, a third SHA, invalid labels, and label/image drift fail
closed. A same-target retry relies on goose to no-op versions already applied.

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
`device-cleanup` to the dev Artifact Registry. Images use the full commit SHA
as their tag; `latest`, mutable aliases, and rebuilt images are not release
inputs. Documentation and other non-impacting commits do not rebuild images.

### Tag mutability

Neither registry enables Artifact Registry's `--immutable-tags`, and that is
deliberate rather than an oversight. That setting also blocks deletion of
tagged images, which would break the cleanup policies both repositories rely
on and would make release tags permanent once the planned `v*` scheme exists.

A commit-SHA tag is therefore movable in principle. What stops a moved tag from
reaching a deployment is attestation, not the registry: a re-pointed tag
resolves to a digest with no valid provenance for that commit, and the resolver
fails closed. Attestation is the single control here. Do not weaken it on the
assumption that registry immutability is also holding the line, and do not
enable immutable tags without first redesigning retention.

The checked-in target catalog at `.github/scripts/release/targets.json` is the
single source for the three release targets and their deployment order. The
impact path list in `impact_path_args()` in `.github/scripts/release/release.sh`
is the single source for paths that require a new candidate image. Adding a
path only requires changing that function; its output determines whether an
older candidate remains compatible with current release automation.

### Where release logic lives

Every step the workflows perform lives in `.github/scripts/release/release.sh`
as a subcommand, so that `release_test.sh` can reach it. A workflow step is a
call to one of these plus the GitHub Actions that cannot be expressed in bash
(authentication, buildx, `actions/attest`). Do not inline release logic in
YAML: anything written there is untested by construction.

| Subcommand | Used by |
|---|---|
| `impact-paths` | documentation and manual inspection |
| `check-error-contract` | release, before anything reads resource state |
| `candidate-changes` | CI, to decide whether to publish |
| `stage-candidate` / `verify-attestation` / `finalize-candidate` | CI candidate publication |
| `resolve-candidate` | release, to select and verify the candidate |
| `preflight` | release, to validate fleet state and pick the baseline |
| `migration-phases` | release, to select goose phases |
| `promote` | prod release, to copy digests into the prod registry |
| `deploy` / `verify` | release |

Functions above the `# === dispatch ===` marker are sourced directly by
`release_test.sh`. Keep that marker line exactly as written.

Missing-resource detection is concentrated in `not_found.sh`. Every matcher
there accepts only a real `NOT_FOUND` status token or one complete, exactly
observed error line; whole-line matching is what makes a status denylist
unnecessary. Those literals are re-probed against the live API by
`check-error-contract` on every release, because a fixture can only prove that
the parsing still reads the old string, never that gcloud still emits it.

The resolver checks at most the newest 50 first-parent commits. If no complete
compatible candidate is found in that window, publish a new release-impacting
candidate rather than relying on an unbounded registry scan.

CI pushes each image to a run-scoped staging tag, resolves and attests its exact
digest, verifies the attestation, and only then adds the SHA tag. The staging
tag is left in place afterwards; it aliases the same digest, is never a release
input, and expires with its image version. A release resolves the selected
candidate's three SHA tags once and writes a run-local JSON bundle:

```json
{
  "release_sha": "<40-character-main-commit-sha>",
  "images": {
    "radar": "<registry>/<repository>/radar@sha256:<64-hex-digest>",
    "geoworker": "<registry>/<repository>/geoworker@sha256:<64-hex-digest>",
    "device-cleanup": "<registry>/<repository>/device-cleanup@sha256:<64-hex-digest>"
  }
}
```

The bundle lasts only for that workflow run; there is no OCI manifest marker.
GitHub attestations bind each exact image digest to this repository, the
candidate workflow, and the `main` commit. Release fails unless all three
digests resolve and pass verification. An existing image with an invalid or
missing attestation fails closed; the resolver never silently falls back to an
older candidate in that case.

### Release Cloud Run

`Release Cloud Run` has a required `environment` input, either `dev` or `prod`,
and an optional `release_sha` input containing a commit SHA, abbreviated to at
least 7 hexadecimal characters or given in full. An empty `release_sha` makes
the workflow execute only current protected automation (`CONTROL_SHA`) and walk
first-parent history to select the newest ancestor with all three complete,
attested candidate images (`RELEASE_SHA`). When `release_sha` is supplied, the
resolver expands it to the full SHA, selects that commit directly, still
requires a complete set of digest-pinned images and valid attestations, and
skips the impact-path freshness check.

An abbreviation is expanded with `git rev-parse --verify`, which rejects both
unknown and ambiguous prefixes, so widening the accepted input does not widen
what can be deployed. Image tags are always the full 40 characters; everything
downstream of the resolver, including the bundle, carries the expanded form.

This separation allows a docs-only commit after a verified dev release to
promote that same candidate without rebuilding it. The resolver rejects any
candidate whose range to `CONTROL_SHA` changes a release-impacting path. If no
compatible complete candidate exists, the release fails closed and a new
release-impacting commit must produce one.

The checked-in workflow verifies the control SHA against remote `main` once,
before any read or mutation, and rejects all reruns. Dispatch a new release run
for every retry. Those checks do not protect against someone dispatching a
historical workflow definition that predates them.

### What the review gates actually enforce

Two different controls are both called "review". Keep them apart.

**Merge review** decides what reaches `main`. The `main` ruleset requires a
pull request with one approving review, and bypass is granted to the repository
admin role only. In practice that means a non-admin collaborator's pull request
needs an approval, and the admin's own merges do not. Dependabot pull requests
are authored by the bot, so an admin merging them is also merging without a
second pair of eyes: the protection against a poisoned action bump is reading
the diff, not the ruleset.

**Deployment review** decides whether a dispatched prod run may proceed. It is
the `prod` GitHub Environment's `required_reviewers` rule, and it is unrelated
to pull requests. This repository has one maintainer, `prevent_self_review` is
`false`, and the maintainer is the only listed reviewer. The gate is therefore
a deliberate pause before prod is touched, not separation of duties, and it
does not defend against the dispatcher. Use the pause to confirm that the run's
`github.sha` is the current remote `main` HEAD. Dev has no required reviewer.

Turning this into real separation of duties requires setting
`prevent_self_review` to `true` and adding a second reviewer with access to the
`prod` environment. Until that happens, do not write automation whose safety
argument depends on a second person existing.

A release processes the complete bundle in this order:

0. Re-probe the gcloud not-found error contract against the live API.
1. For prod, verify dev and copy exact digests into the prod registry.
2. Run required migration phases; pinned releases skip migrations and must
   clear the schema-drift check described above.
3. Deploy `geoworker`, `device-cleanup`, then `radar`.
4. Verify `release-sha`, exact digest, readiness, and Radar `/health`.

Prod requires dev to be currently running the same selected SHA and three source
digests. Promotion copies without rebuilding and verifies every destination.
An existing prod SHA tag is accepted only when its digest is identical. A dev
release that has already been replaced cannot be promoted.

If a candidate SHA tag exists but its attestation is missing or invalid, CI
fails closed and never re-attests the existing digest. Do not rerun the failed
candidate workflow. The standard recovery is to quarantine the affected SHA and
publish a new release-impacting commit; CI must create a new SHA tag and verify
its attestation before release.

Removing the bad tag instead is possible but is not CI's to do: the candidate
identity holds `roles/artifactregistry.writer`, which grants
`artifactregistry.tags.create` and `.update` but not `.delete`. Tag removal
therefore requires a registry admin and is an explicitly approved break-glass
procedure. Do not manually attest an unknown image.

Release and operational workflows share the non-canceling
`cloud-run-release` concurrency group across both environments. It covers
migrations, deployment, job execution, scheduler changes, and Cloudflare
synchronization. Use these workflows rather than routine direct `gcloud`
deployment commands.

`Cloud Run Operations` separately supports:

- `execute-device-cleanup`
- `configure-device-cleanup-scheduler`
- prod-only `sync-cloudflare-origin-secret`

Operations dispatches are limited to the current `main` head. Reruns are
permitted while `main` is unchanged; if `main` advances, dispatch a new
operation run after reviewing the current configuration.

The scheduler name is fixed as `device-cleanup-daily`; schedule and time zone
remain explicit inputs with defaults `0 3 * * *` and `Asia/Taipei`. Operations
does not publish candidates, deploy images, or run migrations.

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

No secret moves in this phase. Role-specific JSON keys remain in their current
GitHub locations, and existing GCP secrets remain in Secret Manager. GitHub
OIDC and Google Cloud Workload Identity Federation are deferred until
exact-digest promotion is stable.

## Prerequisites and Readiness

The workflow files alone do not make the system production-ready. Complete and
record this rollout before declaring readiness:

- [ ] Dev dispatches are restricted to current `main`; each Environment
  contains the correct non-overlapping secrets and variables, including the
  numeric `GCP_PROJECT_NUMBER` required by the Cloud Run Job manifest.
- [ ] The prod Environment requires an external reviewer, prevents the
  triggering actor from self-approving, and its approval procedure verifies
  the run's `github.sha` against the current remote `main` HEAD. This gate is
  required even though the current workflow performs its own remote-main
  check, because a historical workflow definition may not contain that check.
- [ ] Candidate, release, operations, runtime, and scheduler identities exist
  with the scopes above.
- [ ] Neither Artifact Registry repository enables immutable tags, and the
  posture in "Tag mutability" above still holds. Staging tags are never
  deleted: they alias a digest that already carries its SHA tag and expire with
  their image version, so the accumulation is bounded by the cleanup policy. If
  retention is ever changed to keep versions indefinitely, revisit this.
- [ ] Required Google APIs are enabled and all three resources implement the
  `release-sha` label contract.
- [ ] A documentation-only commit after a verified candidate promotes the
  newest compatible ancestor without rebuilding; a release-impacting commit
  without a complete candidate fails closed and is recovered by publishing a
  new candidate.
- [ ] A pinned release SHA is a current-main ancestor with complete attested
  images; pinned releases skip migrations and are used for approved rollback
  or version pinning.
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

1. Inspect all three resources. Do not infer a baseline from one label or tag.
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
