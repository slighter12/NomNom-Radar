#!/usr/bin/env bash
# Execute real version workflow steps against temporary Git and service fakes.
set -euo pipefail
unset EXPECTED_SHA EXPECTED_TAG RETRY_BUNDLE_FILE RETRY_CONTROL_SHA RELEASE_REF
root=$(cd "$(dirname "$0")/../../../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/repo/.github/scripts/release" "$tmp/runner"
cp "$root/.github/scripts/release/candidate.sh" "$root/.github/scripts/release/targets.json" "$tmp/repo/.github/scripts/release/"
for entry in 'request request' 'request images' 'release images'; do
  read -r job id <<<"$entry"
  JOB="$job" ID="$id" yq -r '.jobs[strenv(JOB)].steps[] | select(.id == strenv(ID)) | .run' "$root/.github/workflows/version-images.yml" > "$tmp/$job-$id.sh"
done
for name in 'Revalidate fixed request' 'Establish Git tag'; do
  NAME="$name" yq -r '.jobs.release.steps[] | select(.name == strenv(NAME)) | .run' "$root/.github/workflows/version-images.yml" > "$tmp/$name.sh"
done
yq -r '.runs.steps[] | select(.id == "images") | .run' "$root/.github/actions/publish-images/action.yml" > "$tmp/publish.sh"
export PATH="$tmp/bin:$PATH" RUNNER_TEMP="$tmp/runner" FAKE_DIR="$tmp"
export GITHUB_REPOSITORY=example/project GITHUB_REF=refs/heads/main OWNER=example
export REGISTRY=registry/prod PROJECT_ID=prod DEV_REGISTRY=registry/dev DEV_PROJECT_ID=dev
export VERSION=v1.2.3 GITHUB_OUTPUT="$tmp/output" ATTESTATION_RETRIES=1
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
for name in refs prod dev; do printf '[]' > "$tmp/$name.json"; done
cd "$tmp/repo"
git init -q
git config user.name Tests
git config user.email tests@example.invalid
printf '## [1.2.3] - 2026-09-06\n' > CHANGELOG.md
git add .
git commit -qm source
export CONTROL_SHA=$(git rev-parse HEAD)
export RELEASE_SHA=$CONTROL_SHA EXISTING=false
reject() { if "$@" > "$tmp/rejected" 2>&1; then echo "unexpected success: $*" >&2; exit 1; fi; }
bash "$tmp/request-request.sh"
grep -qx "release_sha=$CONTROL_SHA" "$GITHUB_OUTPUT"
reject env VERSION=v01.2.3 bash "$tmp/request-request.sh"
reject env API_FAIL=true bash "$tmp/request-request.sh"
bash "$tmp/request-images.sh"
jq -e '(.missing | sort) == ["device-cleanup","geoworker","radar"]' "$RUNNER_TEMP/version-images.json" >/dev/null
reject bash "$tmp/release-images.sh"
[ ! -f "$tmp/git-writes" ]
echo 'ok - new request selects main and missing images cannot enter publication'
digest="sha256:$(printf 'a%.0s' {1..64})"
jq -n --arg sha "$RELEASE_SHA" --arg digest "$digest" '["device-cleanup","geoworker","radar"] | map({tag:$sha[0:7],image:("registry/prod/example/"+.),version:$digest})' > "$tmp/prod.json"
env DEV_FAIL=true bash "$tmp/release-images.sh"
reject env ATTEST_FAIL=true bash "$tmp/release-images.sh"
reject env QUERY_FAIL=true bash "$tmp/release-images.sh"
reject env REF_FAIL=true bash "$tmp/Establish Git tag.sh"
[ "$(cat "$tmp/refs.json")" = '[]' ]
bash "$tmp/Establish Git tag.sh"
jq -e --arg sha "$RELEASE_SHA" '.[0].object == {type:"commit",sha:$sha}' "$tmp/refs.json" >/dev/null
bash "$tmp/Revalidate fixed request.sh"
bash "$tmp/Establish Git tag.sh"
[ "$(wc -l < "$tmp/git-writes")" -eq 1 ]
echo 'ok - publication creates a lightweight tag and identical competing requests reuse it'
export BUNDLE_FILE="$RUNNER_TEMP/version-images.json"
export IMAGE_TAGS="$(jq -nc --arg sha "${RELEASE_SHA:0:7}" '[$sha,"v1.2.3"]')"
reject env FAIL_TAG_TARGET=radar bash "$tmp/publish.sh"
[ "$(wc -l < "$tmp/image-writes")" -eq 2 ]
echo newer > file
git add file
git commit -qm newer
export CONTROL_SHA=$(git rev-parse HEAD) EXISTING=true
bash "$tmp/request-request.sh"
grep -qx "release_sha=$RELEASE_SHA" "$GITHUB_OUTPUT"
bash "$tmp/release-images.sh"
bash "$tmp/Establish Git tag.sh"
bash "$tmp/publish.sh"
[ "$(wc -l < "$tmp/image-writes")" -eq 3 ]
env DENY_TAG_WRITES=true bash "$tmp/publish.sh"
reject env RELEASE_SHA="$CONTROL_SHA" bash "$tmp/Revalidate fixed request.sh"
echo 'ok - retry after main advances preserves SHA and completes only missing tags'
cp "$tmp/refs.json" "$tmp/good-refs.json"
cp "$tmp/prod.json" "$tmp/good-prod.json"
printf '[]' > "$tmp/refs.json"
reject bash "$tmp/Revalidate fixed request.sh"
reject bash "$tmp/Establish Git tag.sh"
bash "$tmp/request-request.sh"
grep -qx "release_sha=$CONTROL_SHA" "$GITHUB_OUTPUT"
cp "$tmp/good-refs.json" "$tmp/refs.json"
printf '[]' > "$tmp/prod.json"
reject bash "$tmp/request-images.sh"
jq 'map(if .tag == "v1.2.3" then .version="sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" else . end)' "$tmp/good-prod.json" > "$tmp/prod.json"
reject bash "$tmp/release-images.sh"
cp "$tmp/good-prod.json" "$tmp/prod.json"
echo 'ok - deleted tags, missing SHA images and conflicting version tags fail without rebuilding'
# Annotated tags are peeled to a commit; free text is not an application schema.
jq -n --arg sha "$RELEASE_SHA" '{object:{type:"commit",sha:$sha}}' > "$tmp/tag-object.json"
jq '.[0].object={type:"tag",sha:"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}' "$tmp/good-refs.json" > "$tmp/refs.json"
bash "$tmp/request-request.sh"
grep -qx "release_sha=$RELEASE_SHA" "$GITHUB_OUTPUT"
export TARGET_ENVIRONMENT=prod RUN_MIGRATIONS=false RELEASE_REF=v1.2.3
git tag v1.2.3 "$RELEASE_SHA"
bash "$root/.github/scripts/release/resolve.sh"
jq -e --arg sha "$RELEASE_SHA" '.release_sha == $sha' "$RUNNER_TEMP/release-plan/plan.json" >/dev/null
git tag -f -a v1.2.3 "$RELEASE_SHA" -m 'Release notes' >/dev/null
bash "$root/.github/scripts/release/resolve.sh"
git tag -f v1.2.3 "$CONTROL_SHA" >/dev/null
reject bash "$root/.github/scripts/release/resolve.sh"
echo 'ok - lightweight and annotated versions deploy their exact commit, never an older candidate'

# Construction remains limited to the caller main SHA, even through reusable CI.
yq -r '.jobs["candidate-changes"].steps[] | select(.id == "changes") | .run' "$root/.github/workflows/ci.yml" > "$tmp/changes.sh"
export REQUEST_SHA="$CONTROL_SHA" GITHUB_SHA="$CONTROL_SHA" BUILD_TARGETS='["radar"]'
export TARGETS_FILE="$root/.github/scripts/release/targets.json"
bash "$tmp/changes.sh"
reject env REQUEST_SHA="$RELEASE_SHA" bash "$tmp/changes.sh"
reject env BUILD_TARGETS='["unknown"]' bash "$tmp/changes.sh"
env REQUEST_SHA="$RELEASE_SHA" BUILD_TARGETS='[]' bash "$tmp/changes.sh"
echo 'ok - reusable CI permits historical checks but not historical rebuilds'
