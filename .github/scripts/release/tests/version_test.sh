#!/usr/bin/env bash
# Execute real version workflow steps against temporary Git and service fakes.
set -euo pipefail
unset EXPECTED_SHA EXPECTED_TAG RETRY_BUNDLE_FILE RETRY_CONTROL_SHA RELEASE_REF
root=$(cd "$(dirname "$0")/../../../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"
for entry in 'request request' 'request images' 'release images'; do
  read -r job id <<<"$entry"
  JOB="$job" ID="$id" yq -r '.jobs[strenv(JOB)].steps[] | select(.id == strenv(ID)) | .run' "$root/.github/workflows/version-images.yml" > "$tmp/$job-$id.sh"
done
for name in 'Revalidate fixed request' 'Establish Git tag'; do
  NAME="$name" yq -r '.jobs.release.steps[] | select(.name == strenv(NAME)) | .run' "$root/.github/workflows/version-images.yml" > "$tmp/$name.sh"
done
yq -r '.runs.steps[] | select(.id == "images") | .run' "$root/.github/actions/publish-images/action.yml" > "$tmp/publish.sh"
export PATH="$tmp/bin:$PATH"
export GITHUB_REPOSITORY=example/project GITHUB_REF=refs/heads/main OWNER=example
export REGISTRY=registry/prod PROJECT_ID=prod DEV_REGISTRY=registry/dev DEV_PROJECT_ID=dev
export VERSION=v1.2.3 ATTESTATION_RETRIES=1
cat > "$tmp/bin/gh" <<'FAKE'
#!/usr/bin/env bash
set -eu
[ "${API_FAIL:-false}" != true ] || exit 1
case "$*" in
 *'attestation verify'*) [ "${ATTEST_FAIL:-false}" != true ] ;;
 *'--method POST'*)
   while [ "$1" != --input ]; do shift; done
   [ "${REF_FAIL:-false}" != true ] || exit 1
   jq '[{ref:.ref,object:{type:"commit",sha:.sha}}]' "$2" > "$FAKE_DIR/refs.json"
   echo tag >> "$FAKE_DIR/git-writes" ;;
 *git/matching-refs/*) cat "$FAKE_DIR/refs.json" ;;
 *git/tags/*) jq -c .object "$FAKE_DIR/tag-object.json" ;;
 *) exit 1 ;;
esac
FAKE
cat > "$tmp/bin/gcloud" <<'FAKE'
#!/usr/bin/env bash
set -eu
[ "${QUERY_FAIL:-false}" != true ] || exit 1
case "$*" in
 *'tags add'*)
   [ "${DENY_TAG_WRITES:-false}" != true ]
   case "$6" in *"${FAIL_TAG_TARGET:-never}"*) exit 1 ;; esac
   printf '%s %s\n' "$5" "$6" >> "$FAKE_DIR/image-writes"
   jq --arg base "${6%:*}" --arg tag "${6##*:}" --arg digest "${5##*@}" \
     '. + [{image:$base,tag:$tag,version:$digest}]' "$FAKE_DIR/prod.json" > "$FAKE_DIR/next.json"
   mv "$FAKE_DIR/next.json" "$FAKE_DIR/prod.json" ;;
 *registry/prod*) cat "$FAKE_DIR/prod.json" ;;
 *registry/dev*) [ "${DEV_FAIL:-false}" != true ]; cat "$FAKE_DIR/dev.json" ;;
 *) exit 1 ;;
esac
FAKE
chmod +x "$tmp/bin/"*
yq -r '.jobs["candidate-changes"].steps[] | select(.id == "changes") | .run' "$root/.github/workflows/ci.yml" > "$tmp/changes.sh"
steps=$tmp

fail() { echo "not ok - $scenario: $*" >&2; exit 1; }
reject() {
  if "$@" > "$tmp/rejected" 2>&1; then fail "unexpected success: $*"; fi
}
run_step() {
  step_output=$(mktemp "$tmp/step-output.XXXXXX") || return
  GITHUB_OUTPUT="$step_output" "$@"
}
read_output() {
  local key=$1 count
  count=$(grep -c "^${key}=" "$step_output") || true
  [ "$count" = 1 ] || fail "expected exactly one $key output, found $count"
  sed -n "s/^${key}=//p" "$step_output"
}
assert_output() {
  local actual
  actual=$(read_output "$1") || return
  [ "$actual" = "$2" ] || fail "$1: expected $2, got $actual"
}
# Capture the request contract before another step replaces step_output.
request() {
  run_step bash "$steps/request-request.sh"
  assert_output release_sha "$1"
  assert_output existing "$2"
  RELEASE_SHA=$(read_output release_sha)
  EXISTING=$(read_output existing)
  export RELEASE_SHA EXISTING
}
fixture() {
  scenario=$1
  tmp=$(mktemp -d "$steps/scenario.XXXXXX")
  trap 'echo "not ok - $scenario: line $LINENO" >&2' ERR
  set -E
  unset RELEASE_SHA EXISTING BUNDLE_FILE IMAGE_TAGS
  export RUNNER_TEMP="$tmp/runner" FAKE_DIR="$tmp"
  mkdir -p "$tmp/repo/.github/scripts/release" "$RUNNER_TEMP"
  cp "$root/.github/scripts/release/candidate.sh" "$root/.github/scripts/release/targets.json" "$tmp/repo/.github/scripts/release/"
  for name in refs prod dev; do printf '[]' > "$tmp/$name.json"; done
  cd "$tmp/repo"
  git init -q
  git config user.name Tests
  git config user.email tests@example.invalid
  printf '## [1.2.3] - 2026-09-06\n' > CHANGELOG.md
  git add .
  git commit -qm source
  CONTROL_SHA=$(git rev-parse HEAD)
  export CONTROL_SHA
  source_sha=$CONTROL_SHA
  digest="sha256:$(printf 'a%.0s' {1..64})"
}
seed_images() {
  jq -n --arg sha "$source_sha" --arg digest "$digest" '["device-cleanup","geoworker","radar"] | map({tag:$sha[0:7],image:("registry/prod/example/"+.),version:$digest})' > "$tmp/prod.json"
}
seed_version_tag() {
  jq -n --arg sha "$source_sha" '[{ref:"refs/tags/v1.2.3",object:{type:"commit",sha:$sha}}]' > "$tmp/refs.json"
}
advance_main() {
  echo newer > file
  git add file
  git commit -qm newer
  CONTROL_SHA=$(git rev-parse HEAD)
  export CONTROL_SHA
}

(
  fixture 'new request validates inputs and refuses publication with missing images'
  request "$source_sha" false
  reject run_step env VERSION=v01.2.3 bash "$steps/request-request.sh"
  reject run_step env API_FAIL=true bash "$steps/request-request.sh"
  run_step bash "$steps/request-images.sh"
  jq -e '(.missing | sort) == ["device-cleanup","geoworker","radar"]' "$RUNNER_TEMP/version-images.json" >/dev/null
  reject run_step bash "$steps/release-images.sh"
  [ ! -f "$tmp/git-writes" ]
  echo "ok - $scenario"
)
(
  fixture 'partial publication retries its fixed SHA after main advances and then becomes a no-op'
  request "$source_sha" false
  seed_images
  run_step env DEV_FAIL=true bash "$steps/release-images.sh"
  reject run_step env ATTEST_FAIL=true bash "$steps/release-images.sh"
  reject run_step env QUERY_FAIL=true bash "$steps/release-images.sh"
  reject run_step env REF_FAIL=true bash "$steps/Establish Git tag.sh"
  [ "$(cat "$tmp/refs.json")" = '[]' ]
  run_step bash "$steps/Establish Git tag.sh"
  jq -e --arg sha "$RELEASE_SHA" '.[0].object == {type:"commit",sha:$sha}' "$tmp/refs.json" >/dev/null
  run_step bash "$steps/Revalidate fixed request.sh"
  run_step bash "$steps/Establish Git tag.sh"
  [ "$(wc -l < "$tmp/git-writes")" -eq 1 ]
  export BUNDLE_FILE="$RUNNER_TEMP/version-images.json"
  IMAGE_TAGS=$(jq -nc --arg sha "${RELEASE_SHA:0:7}" '[$sha,"v1.2.3"]')
  export IMAGE_TAGS
  reject run_step env FAIL_TAG_TARGET=radar bash "$steps/publish.sh"
  [ "$(wc -l < "$tmp/image-writes")" -eq 2 ]
  advance_main
  request "$source_sha" true
  run_step bash "$steps/release-images.sh"
  run_step bash "$steps/Establish Git tag.sh"
  run_step bash "$steps/publish.sh"
  [ "$(wc -l < "$tmp/image-writes")" -eq 3 ]
  request "$source_sha" true
  run_step bash "$steps/release-images.sh"
  run_step bash "$steps/Establish Git tag.sh"
  run_step env DENY_TAG_WRITES=true bash "$steps/publish.sh"
  [ "$(wc -l < "$tmp/git-writes")" -eq 1 ]
  reject run_step env RELEASE_SHA="$CONTROL_SHA" bash "$steps/Revalidate fixed request.sh"
  echo "ok - $scenario"
)
(
  fixture 'deleted tags invalidate an existing request but allow a new request on current main'
  seed_version_tag
  request "$source_sha" true
  advance_main
  printf '[]' > "$tmp/refs.json"
  # Keep the captured request while checking deletion; only then request anew.
  reject run_step bash "$steps/Revalidate fixed request.sh"
  reject run_step bash "$steps/Establish Git tag.sh"
  request "$CONTROL_SHA" false
  echo "ok - $scenario"
)
(
  fixture 'existing versions with missing SHA images fail recovery'
  seed_version_tag
  request "$source_sha" true
  reject run_step bash "$steps/request-images.sh"
  [ ! -f "$tmp/git-writes" ]
  [ ! -f "$tmp/image-writes" ]
  echo "ok - $scenario"
)
(
  fixture 'conflicting version images fail before publication'
  seed_version_tag
  seed_images
  request "$source_sha" true
  jq --arg digest "sha256:$(printf 'c%.0s' {1..64})" '. + map(.tag="v1.2.3" | .version=$digest)' "$tmp/prod.json" > "$tmp/conflicting.json"
  cp "$tmp/conflicting.json" "$tmp/prod.json"
  reject run_step bash "$steps/release-images.sh"
  cmp "$tmp/conflicting.json" "$tmp/prod.json"
  [ ! -f "$tmp/git-writes" ]
  [ ! -f "$tmp/image-writes" ]
  echo "ok - $scenario"
)
(
  fixture 'annotated version requests use the peeled commit'
  seed_version_tag
  seed_images
  advance_main
  # Annotation text is not an application schema.
  jq -n --arg sha "$source_sha" '{object:{type:"commit",sha:$sha}}' > "$tmp/tag-object.json"
  jq '.[0].object={type:"tag",sha:"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}' "$tmp/refs.json" > "$tmp/annotated.json"
  cp "$tmp/annotated.json" "$tmp/refs.json"
  request "$source_sha" true
  run_step bash "$steps/request-images.sh"
  echo "ok - $scenario"
)
(
  fixture 'lightweight and annotated versions deploy their exact commit, never an older candidate'
  seed_images
  advance_main
  export TARGET_ENVIRONMENT=prod RUN_MIGRATIONS=false RELEASE_REF=v1.2.3
  git tag v1.2.3 "$source_sha"
  run_step bash "$root/.github/scripts/release/resolve.sh"
  jq -e --arg sha "$source_sha" '.release_sha == $sha' "$RUNNER_TEMP/release-plan/plan.json" >/dev/null
  git tag -f -a v1.2.3 "$source_sha" -m 'Release notes' >/dev/null
  run_step bash "$root/.github/scripts/release/resolve.sh"
  jq -e --arg sha "$source_sha" '.release_sha == $sha' "$RUNNER_TEMP/release-plan/plan.json" >/dev/null
  git tag -f v1.2.3 "$CONTROL_SHA" >/dev/null
  reject run_step bash "$root/.github/scripts/release/resolve.sh"
  echo "ok - $scenario"
)
(
  fixture 'reusable CI permits historical checks but not historical rebuilds'
  advance_main
  export REQUEST_SHA="$CONTROL_SHA" GITHUB_SHA="$CONTROL_SHA" BUILD_TARGETS='["radar"]'
  export TARGETS_FILE="$root/.github/scripts/release/targets.json"
  run_step bash "$steps/changes.sh"
  reject run_step env REQUEST_SHA="$source_sha" bash "$steps/changes.sh"
  reject run_step env BUILD_TARGETS='["unknown"]' bash "$steps/changes.sh"
  run_step env REQUEST_SHA="$source_sha" BUILD_TARGETS='[]' bash "$steps/changes.sh"
  echo "ok - $scenario"
)
