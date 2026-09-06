#!/usr/bin/env bash
# Focused release contracts; all external services are replaced by local fakes.
set -euo pipefail
scripts=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/repo" "$tmp/runner"
export PATH="$tmp/bin:$PATH" RUNNER_TEMP="$tmp/runner"
export OWNER=Example GITHUB_REPOSITORY=example/project
export REGISTRY=registry/prod DEV_REGISTRY=registry/dev PROJECT_ID=prod DEV_PROJECT_ID=dev
export TARGET_ENVIRONMENT=prod RUN_MIGRATIONS=false ATTESTATION_RETRIES=1 ATTESTATION_RETRY_DELAY=0
export FAKE_DIR="$tmp" GITHUB_OUTPUT="$tmp/output"
cat > "$tmp/bin/gcloud" <<'FAKE'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FAKE_DIR/gcloud.calls"
[ "${QUERY_FAIL:-false}" != true ] || exit 3
case "$*" in
  *registry/dev*) [ "${DEV_FAIL:-false}" != true ] || exit 4; cat "$FAKE_DIR/dev.json" ;;
  *registry/prod*) cat "$FAKE_DIR/prod.json" ;;
  *) exit 5 ;;
esac
FAKE
cat > "$tmp/bin/gh" <<'FAKE'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FAKE_DIR/gh.calls"
[ "${ATTEST_FAIL:-false}" != true ] || exit 6
case "$*" in *"--source-digest $EXPECTED_SHA"*) ;; *) exit 7 ;; esac
case "$*" in *"--signer-workflow $GITHUB_REPOSITORY/.github/workflows/ci.yml"*) ;; *) exit 8 ;; esac
FAKE
chmod +x "$tmp/bin/"*
source "$scripts/candidate.sh"
passed=0
ok() { passed=$((passed + 1)); printf 'ok %s - %s\n' "$passed" "$1"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
reject() { if "$@" > "$tmp/rejected.out" 2>&1; then fail "unexpected success: $*"; fi; }
cd "$tmp/repo"
git init -q
git config user.email tests@example.invalid
git config user.name Tests
mkdir -p internal docs deploy database/migration
cp -R "$scripts/../../../deploy/cloud-run" deploy/
# Historical candidates may predate the explicit traffic policy.
yq -i 'del(.spec.traffic)' deploy/cloud-run/base/service-template.yaml
printf "%s\n" "-- migration fixture" > database/migration/fixture.sql
printf 'application A\n' > internal/app.txt
git add .
git commit -qm candidate
export EXPECTED_SHA=$(git rev-parse HEAD)
candidate=$EXPECTED_SHA
printf 'docs\n' > docs/readme.md
printf '## [0.3.0] - 2026-09-06\n' > CHANGELOG.md
git add .
git commit -qm documentation
export CONTROL_SHA=$(git rev-parse HEAD)
git tag v0.3.0
digest="sha256:$(printf 'a%.0s' {1..64})"
other="sha256:$(printf 'b%.0s' {1..64})"
make_tags() {
  jq -n --arg registry "$1" --arg sha "$candidate" --arg digest "$digest" \
    '["radar","geoworker","device-cleanup"] | map({tag:($sha[0:7]),image:($registry+"/example/"+.),version:("projects/p/locations/r/repositories/x/packages/y/versions/"+$digest)})' > "$2"
}
make_tags "$REGISTRY" "$tmp/prod.json"
make_tags "$DEV_REGISTRY" "$tmp/dev.json"
[ "$(candidate_image "$tmp/prod.json" "$REGISTRY" radar "$candidate")" = "$REGISTRY/example/radar@$digest" ] || fail digest
ok 'registry tag/image/version fields resolve exact digest'
jq --arg sha "$candidate" 'map(.tag=$sha)' "$tmp/prod.json" > "$tmp/legacy.json"
[ "$(candidate_image "$tmp/legacy.json" "$REGISTRY" radar "$candidate")" = "$REGISTRY/example/radar@$digest" ] || fail legacy
ok 'legacy full SHA-only tags remain usable'
printf '[]' > "$tmp/empty.json"
[ -z "$(candidate_image "$tmp/empty.json" "$REGISTRY" radar "$candidate")" ] || fail empty
reject env QUERY_FAIL=true bash -c 'source "$1"; list_tags "$REGISTRY" "$PROJECT_ID"' _ "$scripts/candidate.sh"
ok 'empty query differs from failed query'
jq --arg sha "$candidate" --arg digest "$other" '. + [{tag:$sha,image:.[0].image,version:$digest}]' "$tmp/prod.json" > "$tmp/conflict.json"
reject candidate_image "$tmp/conflict.json" "$REGISTRY" radar "$candidate"
reject candidate_image "$tmp/prod.json" "$REGISTRY" radar invalid
ok 'short/full conflict and invalid candidate SHA fail closed'
export RELEASE_REF="$candidate" DEV_FAIL=true
bash "$scripts/resolve.sh" >/dev/null
plan="$RUNNER_TEMP/release-plan/plan.json"
[ "$(jq -r .release_sha "$plan")" = "$candidate" ] || fail resolved
! grep -q registry/dev "$tmp/gcloud.calls" || fail 'prod consulted dev'
ok 'retained prod candidate does not require dev access'
reject env RELEASE_REF=v0.3.0 bash "$scripts/resolve.sh"
ok 'version selection requires images for the exact tag commit'
jq --arg digest "$other" '. + [{tag:"v0.3.0",image:.[0].image,version:$digest}]' "$tmp/prod.json" > "$tmp/conflicting-version.json"
cp "$tmp/prod.json" "$tmp/good-prod.json"
cp "$tmp/conflicting-version.json" "$tmp/prod.json"
reject env RELEASE_REF=v0.3.0 bash "$scripts/resolve.sh"
cp "$tmp/good-prod.json" "$tmp/prod.json"
bash "$scripts/resolve.sh" >/dev/null
ok 'version tag conflicts fail closed'
reject env ATTEST_FAIL=true bash "$scripts/resolve.sh"
reject env RELEASE_REF=main bash "$scripts/resolve.sh"
reject env RELEASE_REF="$CONTROL_SHA" bash "$scripts/resolve.sh"
ok 'invalid ref, missing exact candidate, and invalid attestation fail closed'
cp "$plan" "$tmp/retry.json"
export RETRY_BUNDLE_FILE="$tmp/retry.json" RELEASE_REF='' RETRY_CONTROL_SHA="$CONTROL_SHA"
bash "$scripts/resolve.sh" >/dev/null
[ "$(jq -c .images "$plan")" = "$(jq -c .images "$tmp/retry.json")" ] || fail retry
ok 'retry retains the saved exact digests'
reject env RETRY_CONTROL_SHA="$candidate" bash "$scripts/resolve.sh"
jq '.environment="dev"' "$tmp/retry.json" > "$tmp/bad-retry.json"
reject env RETRY_BUNDLE_FILE="$tmp/bad-retry.json" bash "$scripts/resolve.sh"
jq '.sources.radar="evil.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' "$tmp/retry.json" > "$tmp/bad-retry.json"
reject env RETRY_BUNDLE_FILE="$tmp/bad-retry.json" bash "$scripts/resolve.sh"
ok 'retry rejects mismatched environment and untrusted registry'
unset RETRY_BUNDLE_FILE
export TARGET_ENVIRONMENT=dev REGISTRY="$DEV_REGISTRY" PROJECT_ID="$DEV_PROJECT_ID" DEV_FAIL=false
bash "$scripts/resolve.sh" >/dev/null
[ "$(jq -r .release_sha "$plan")" = "$candidate" ] || fail docs
ok 'dev latest follows only documentation changes to a complete candidate'
cp "$tmp/dev.json" "$tmp/short-dev.json"
jq --arg sha "$candidate" 'map(.tag=$sha)' "$tmp/dev.json" > "$tmp/full-dev.json"
cp "$tmp/full-dev.json" "$tmp/dev.json"
bash "$scripts/resolve.sh" >/dev/null
[ "$(jq -r .release_sha "$plan")" = "$candidate" ] || fail 'legacy dev candidate'
jq -e '.images == .sources' "$plan" >/dev/null || fail 'dev requires copy'
cp "$tmp/short-dev.json" "$tmp/dev.json"
ok 'dev full-SHA-only candidate resolves without any registry mutation'
printf 'application B\n' > internal/app.txt
git add .
git commit -qm new-application
export CONTROL_SHA=$(git rev-parse HEAD)
reject bash "$scripts/resolve.sh"
ok 'dev cannot skip unpublished application changes'
git revert --no-edit HEAD >/dev/null
export CONTROL_SHA=$(git rev-parse HEAD)
reject bash "$scripts/resolve.sh"
ok 'candidate selection cannot cross release-impacting changes followed by a revert'

# The chosen commit supplies deployment data even when the worktree advances.
export TARGET_ENVIRONMENT=prod REGISTRY=registry/prod PROJECT_ID=prod
export BUNDLE_FILE="$tmp/retry.json" SOURCE_DIR="$tmp/source" MANIFEST_DIR="$tmp/manifests"
export PROJECT_NUMBER=123456 RUNTIME_SA_EMAIL=runtime@example.invalid ALLOWED_HOST='https://example.invalid'
export GOOGLE_OAUTH_CLIENT_ID='client: "quoted"' CLOUDFLARE_ORIGIN_SECRET='secret: "quoted"'
export RATE_LIMIT_ENABLED=false RATE_LIMIT_RATE=007
# A broken current worktree template must not affect the selected Git tree.
printf 'invalid: [\n' > deploy/cloud-run/base/service-template.yaml
mkdir -p "$SOURCE_DIR"
git archive "$candidate" deploy/cloud-run database/migration | tar -xf - -C "$SOURCE_DIR"
bash "$scripts/render.sh"
sh "$scripts/render-yaml.sh"
[ "$(find "$MANIFEST_DIR" -name '*.yaml' | wc -l | tr -d ' ')" = 3 ] || fail renders
yq -o=json '.' "$MANIFEST_DIR/radar.yaml" | jq -e --arg client "$GOOGLE_OAUTH_CLIENT_ID" '
  (.spec.template.spec.containers[0].env | map({key:.name,value:.value}) | from_entries) as $env
  | $env.HTTP_RATELIMIT_ENABLED == "false" and $env.HTTP_RATELIMIT_RATE == "007"
    and $env.GOOGLEOAUTH_CLIENTID == $client
    and .spec.traffic == [{latestRevision:true,percent:100}]
' >/dev/null || fail 'rendered strings or traffic'
ok 'selected-version manifests render all targets with exact quoted strings and traffic'
reject env PROJECT_NUMBER=invalid sh "$scripts/render-yaml.sh"
reject env ALLOWED_HOST=$'host\nsecret' sh "$scripts/render-yaml.sh"
! grep -q secret "$tmp/rejected.out" || fail 'render leaked value'
ok 'invalid configuration is rejected without exposing values'
git add deploy/cloud-run/base/service-template.yaml
git commit -qm incompatible-template
bad_sha=$(git rev-parse HEAD)
jq --arg sha "$bad_sha" '.release_sha=$sha | .tag_commit=$sha' "$BUNDLE_FILE" > "$tmp/bad-template-plan.json"
mkdir -p "$tmp/source-invalid-template"
git archive "$bad_sha" deploy/cloud-run database/migration | tar -xf - -C "$tmp/source-invalid-template"
reject env BUNDLE_FILE="$tmp/bad-template-plan.json" SOURCE_DIR="$tmp/source-invalid-template" MANIFEST_DIR="$tmp/manifests-invalid-template" bash "$scripts/render.sh"
ok 'incompatible selected template fails before deployment'

cat > "$tmp/bin/gcloud" <<'FAKE'
#!/usr/bin/env bash
set -eu
case "$2" in
 jobs) jq -n --arg sha "$EXPECTED_SHA" --arg image "$REGISTRY/example/device-cleanup@$TEST_DIGEST" '{metadata:{labels:{"release-sha":$sha}},spec:{template:{spec:{template:{spec:{containers:[{image:$image}]}}}}}}' ;;
 services) jq -n --arg sha "$EXPECTED_SHA" --arg target "$4" --arg ready "${READY:-True}" '{metadata:{labels:{"release-sha":$sha}},status:{conditions:[{type:"Ready",status:$ready}],traffic:[{revisionName:$target,percent:100}],url:"https://radar.example.invalid"}}' ;;
 revisions) jq -n --arg sha "$EXPECTED_SHA" --arg image "$REGISTRY/example/$4@${ACTIVE_DIGEST:-$TEST_DIGEST}" '{metadata:{labels:{"release-sha":$sha}},spec:{containers:[{image:$image}]},status:{conditions:[{type:"Ready",status:"True"}]}}' ;;
 *) exit 1 ;;
esac
FAKE
cat > "$tmp/bin/curl" <<'FAKE'
#!/usr/bin/env bash
set -eu
[ "${HEALTH_FAIL:-false}" != true ]
case "$*" in *https://radar.example.invalid/health*) ;; *) exit 1 ;; esac
FAKE
chmod +x "$tmp/bin/curl"
export RADAR_URL=https://radar.example.invalid REGION=region TEST_DIGEST="$digest" VERIFY_RETRIES=1 VERIFY_RETRY_DELAY=0
bash "$scripts/verify.sh"
reject env ACTIVE_DIGEST="$other" bash "$scripts/verify.sh"
reject env READY=False bash "$scripts/verify.sh"
reject env HEALTH_FAIL=true bash "$scripts/verify.sh"
ok 'verification requires active traffic digest, readiness, and successful health'
source "$scripts/tests/workflow_runtime.sh"
bash "$scripts/tests/version_test.sh"
bash "$scripts/tests/workflows.sh"
printf '%s checks passed\n' "$passed"
