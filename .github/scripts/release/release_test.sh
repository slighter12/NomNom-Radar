#!/usr/bin/env bash

set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "${script_dir}/../../.." && pwd)
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nomnom-release-test.XXXXXX")
trap 'rm -rf "${temp_dir}"' EXIT HUP INT TERM

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
# shellcheck source=.github/scripts/release/not_found.sh
source "${repo_root}/.github/scripts/release/not_found.sh"

# release.sh impact-paths prints the manifest without requiring release env
impact_paths_output=$(bash "${script_dir}/release.sh" impact-paths) \
  || fail impact-paths-command
test -n "${impact_paths_output}" \
  && printf '%s\n' "${impact_paths_output}" | grep -Fx 'Dockerfile' >/dev/null \
  && printf '%s\n' "${impact_paths_output}" | grep -Fx 'internal/**' >/dev/null \
  || fail impact-paths-output

status_fixture="${temp_dir}/not-found-status.txt"
printf 'ERROR: (gcloud.artifacts.docker.images.describe) NOT_FOUND: image was not found.\n' > "${status_fixture}"
not_found_status "${status_fixture}" || fail not-found-status
printf 'ERROR: (gcloud...) PERMISSION_DENIED: Could not find valid credentials.\n' > "${status_fixture}"
if not_found_status "${status_fixture}"; then
  fail permission-error-not-not-found
fi
printf 'ERROR: (gcloud...) INVALID_ARGUMENT: The requested location does not exist.\n' > "${status_fixture}"
if not_found_status "${status_fixture}"; then
  fail invalid-argument-not-not-found
fi

# Every literal below was captured from gcloud on 2026-08-23 (project
# radar-dev-491902, asia-east1) by describing resources that do not exist.
# These fixtures can only prove that the matchers still read the strings the
# way they did then; they cannot notice gcloud changing its wording. That is
# what `release.sh check-error-contract` does against the live API on every
# release, and these cases exist to keep the parsing honest in between.
image_fixture="${temp_dir}/image-not-found.txt"
printf 'ERROR: (gcloud.artifacts.docker.images.describe) Image not found.\n' > "${image_fixture}"
image_not_found "${image_fixture}" || fail image-not-found-observed

# A missing repository, as opposed to a missing tag, answers with a status.
printf 'ERROR: (gcloud.artifacts.docker.images.describe) NOT_FOUND: Requested entity was not found.\n' > "${image_fixture}"
image_not_found "${image_fixture}" || fail image-not-found-status

# Whole-line matching is the reason no status denylist is needed: a failure
# that merely contains the not-found wording is not the not-found line.
printf 'ERROR: (gcloud.artifacts.docker.images.describe) PERMISSION_DENIED: Image not found.\n' > "${image_fixture}"
if image_not_found "${image_fixture}"; then
  fail image-not-found-status-collision
fi

run_fixture="${temp_dir}/cloud-run-not-found.txt"
# Observed punctuation differs per surface: jobs end in a period, services and
# revisions do not. Both renderings must be accepted for every surface.
printf 'ERROR: (gcloud.run.services.describe) Cannot find service [radar]\n' > "${run_fixture}"
cloud_run_not_found service radar "${run_fixture}" || fail run-service-not-found-observed
printf 'ERROR: (gcloud.run.jobs.describe) Cannot find job [device-cleanup].\n' > "${run_fixture}"
cloud_run_not_found job device-cleanup "${run_fixture}" || fail run-job-not-found-observed
printf 'ERROR: (gcloud.run.revisions.describe) Cannot find revision [radar-00017-b77]\n' > "${run_fixture}"
cloud_run_not_found revision radar-00017-b77 "${run_fixture}" || fail run-revision-not-found-observed

# A different resource's not-found line must not answer for this one.
printf 'ERROR: (gcloud.run.services.describe) Cannot find service [geoworker]\n' > "${run_fixture}"
if cloud_run_not_found service radar "${run_fixture}"; then
  fail run-not-found-wrong-resource
fi
# A status-qualified failure is never a missing resource.
printf 'ERROR: (gcloud.run.services.describe) PERMISSION_DENIED: Cannot find service [radar]\n' > "${run_fixture}"
if cloud_run_not_found service radar "${run_fixture}"; then
  fail run-not-found-status-collision
fi

git_dir="${temp_dir}/git"
git init -q "${git_dir}"
git -C "${git_dir}" config user.email test@example.invalid
git -C "${git_dir}" config user.name 'Release Test'
git -C "${git_dir}" commit -q --allow-empty -m baseline
parent=$(git -C "${git_dir}" rev-parse HEAD)
git -C "${git_dir}" commit -q --allow-empty -m target
sha=$(git -C "${git_dir}" rev-parse HEAD)
git -C "${git_dir}" commit -q --allow-empty -m future
future=$(git -C "${git_dir}" rev-parse HEAD)
digest_a="registry.example/repo/radar@sha256:$(printf 'a%.0s' {1..64})"
digest_b="registry.example/repo/geoworker@sha256:$(printf 'b%.0s' {1..64})"
digest_c="registry.example/repo/device-cleanup@sha256:$(printf 'c%.0s' {1..64})"
digest_d="registry.example/repo/extra-worker@sha256:$(printf 'e%.0s' {1..64})"
bundle="${temp_dir}/bundle.json"
jq -cn --arg sha "${sha}" --arg a "${digest_a}" --arg b "${digest_b}" --arg c "${digest_c}" \
  '{release_sha:$sha,images:{radar:$a,geoworker:$b,"device-cleanup":$c}}' > "${bundle}"
baseline_digest="sha256:$(printf 'd%.0s' {1..64})"
baseline_map="${temp_dir}/baseline-digests.json"
jq -cn --arg parent "${parent}" --arg future "${future}" --arg digest "${baseline_digest}" '
  def refs: {
    radar:("registry.example/repo/radar@" + $digest),
    geoworker:("registry.example/repo/geoworker@" + $digest),
    "device-cleanup":("registry.example/repo/device-cleanup@" + $digest)
  };
  {($parent):refs,($future):refs}
' > "${baseline_map}"

resource() {
  target=$1 label=$2 exists=$3 desired=${label}
  case "${target}" in
    radar) candidate=${digest_a} ;;
    geoworker) candidate=${digest_b} ;;
    device-cleanup) candidate=${digest_c}; desired= ;;
    extra-worker) candidate=${digest_d} ;;
    *) printf 'unknown fixture target: %s\n' "${target}" >&2; return 1 ;;
  esac
  if [ "${exists}" = false ] || [ -z "${label}" ]; then
    image=
  elif [ "${label}" = "${sha}" ]; then
    image=${candidate}
  else
    image="registry.example/repo/${target}@sha256:$(printf 'd%.0s' {1..64})"
  fi
  jq -cn --arg label "${label}" --arg desired "${desired}" --arg image "${image}" --argjson exists "${exists}" \
    '{exists:$exists,label:$label,desired_label:$desired,ready:true,image:$image}'
}
state() {
  jq -cn --argjson radar "$(resource radar "$1" "$4")" --argjson geo "$(resource geoworker "$2" "$5")" --argjson job "$(resource device-cleanup "$3" "$6")" \
    '{radar:$radar,geoworker:$geo,"device-cleanup":$job}' > "${temp_dir}/state.json"
}
preflight() {
  (cd "${git_dir}" && RELEASE_SHA="${sha}" BUNDLE_FILE="${bundle}" RELEASE_STATE_FILE="${temp_dir}/state.json" \
    REGISTRY="${TEST_REGISTRY:-registry.example}" OWNER=repo \
    RELEASE_BASELINE_DIGESTS_FILE="${baseline_map}" ALLOW_RELEASE_FIXTURES=true \
    bash "${script_dir}/release.sh" preflight)
}
four_catalog="${temp_dir}/targets-four.json"
jq '. + {"extra-worker": {kind:"service", overlay:"geoworker", order:4}}' \
  "${repo_root}/.github/scripts/release/targets.json" > "${four_catalog}"
four_bundle="${temp_dir}/bundle-four.json"
jq -cn --arg sha "${sha}" --arg a "${digest_a}" --arg b "${digest_b}" --arg c "${digest_c}" --arg d "${digest_d}" \
  '{release_sha:$sha,images:{radar:$a,geoworker:$b,"device-cleanup":$c,"extra-worker":$d}}' > "${four_bundle}"
four_baseline_map="${temp_dir}/baseline-digests-four.json"
jq -cn --arg parent "${parent}" --arg digest "${baseline_digest}" '
  def refs: {
    radar:("registry.example/repo/radar@" + $digest),
    geoworker:("registry.example/repo/geoworker@" + $digest),
    "device-cleanup":("registry.example/repo/device-cleanup@" + $digest),
    "extra-worker":("registry.example/repo/extra-worker@" + $digest)
  };
  {($parent):refs}
' > "${four_baseline_map}"
state_four() {
  jq -cn --argjson radar "$(resource radar "$1" "$5")" \
    --argjson geo "$(resource geoworker "$2" "$6")" \
    --argjson job "$(resource device-cleanup "$3" "$7")" \
    --argjson extra "$(resource extra-worker "$4" "$8")" \
    '{radar:$radar,geoworker:$geo,"device-cleanup":$job,"extra-worker":$extra}' > "${temp_dir}/state-four.json"
}
preflight_four() {
  (cd "${git_dir}" && RELEASE_SHA="${sha}" BUNDLE_FILE="${four_bundle}" RELEASE_STATE_FILE="${temp_dir}/state-four.json" \
    TARGETS_FILE="${four_catalog}" REGISTRY="${TEST_REGISTRY:-registry.example}" OWNER=repo \
    RELEASE_BASELINE_DIGESTS_FILE="${four_baseline_map}" ALLOW_RELEASE_FIXTURES=true \
    bash "${script_dir}/release.sh" preflight)
}
expect_empty_baseline() {
  local name=$1 output
  output=$(preflight) || fail "${name}-preflight-exit"
  output=$(tail -n 1 <<<"${output}")
  [ -z "$(jq -r .baseline_sha <<<"${output}")" ] || fail "${name}"
}

state '' '' '' false false false
if (
  cd "${git_dir}"
  RELEASE_SHA="${sha}" BUNDLE_FILE="${bundle}" RELEASE_STATE_FILE="${temp_dir}/state.json" \
    REGISTRY="${TEST_REGISTRY:-registry.example}" OWNER=repo \
    RELEASE_BASELINE_DIGESTS_FILE="${baseline_map}" ALLOW_RELEASE_FIXTURES=false \
    bash "${script_dir}/release.sh" preflight >/dev/null 2>&1
); then
  fail release-state-fixture-guard
fi
expect_empty_baseline bootstrap
state "${parent}" "${parent}" "${parent}" true true true
[ "$(preflight | tail -n 1 | jq -r .baseline_sha)" = "${parent}" ] || fail forward
state "${sha}" "${sha}" "${sha}" true true true
[ "$(preflight | tail -n 1 | jq -r .baseline_sha)" = "${sha}" ] || fail complete
state "${sha}" "${sha}" "${sha}" true true true
jq 'with_entries(.value.image |= sub("^registry.example/"; "prod.example/"))' \
  "${temp_dir}/state.json" > "${temp_dir}/prod-target.json"
mv "${temp_dir}/prod-target.json" "${temp_dir}/state.json"
[ "$(TEST_REGISTRY=prod.example preflight | tail -n 1 | jq -r .baseline_sha)" = "${sha}" ] || fail prod-cross-registry-target
state "${parent}" "${sha}" "${parent}" true true true
[ "$(preflight | tail -n 1 | jq -r .baseline_sha)" = "${parent}" ] || fail partial
state_four "${sha}" "${parent}" "${parent}" "${parent}" true true true true
[ "$(preflight_four | tail -n 1 | jq -r .baseline_sha)" = "${parent}" ] || fail partial-four-target-catalog
state "${sha}" '' '' true false true
expect_empty_baseline target-missing-unlabeled
state "${parent}" '' '' true false true
expect_empty_baseline stale-label-with-missing
state '' '' '' true true true
expect_empty_baseline handover-from-unlabeled-fleet
state "${future}" "${future}" "${future}" true true true
preflight >/dev/null 2>&1 && fail rollback
state "${future}" "${future}" "${future}" true true true
pinned_rollback_output="${temp_dir}/pinned-rollback-preflight.txt"
if ! (
  cd "${git_dir}"
  RELEASE_SHA="${sha}" REQUESTED_SHA="${sha}" CONTROL_SHA="${future}" BUNDLE_FILE="${bundle}" RELEASE_STATE_FILE="${temp_dir}/state.json" \
    REGISTRY="${TEST_REGISTRY:-registry.example}" OWNER=repo \
    RELEASE_BASELINE_DIGESTS_FILE="${baseline_map}" ALLOW_RELEASE_FIXTURES=true \
    bash "${script_dir}/release.sh" preflight
) >"${pinned_rollback_output}" 2>&1; then
  fail pinned-rollback-preflight
fi
[ "$(tail -n 1 "${pinned_rollback_output}" | jq -r .baseline_sha)" = "${future}" ] \
  || fail pinned-rollback-baseline
state broken broken broken true true true
preflight >/dev/null 2>&1 && fail malformed-label
state "${parent}" "${parent}" "${sha}" true true true
jq --arg unexpected "${future}" '.radar.desired_label = $unexpected' "${temp_dir}/state.json" > "${temp_dir}/drift.json"
mv "${temp_dir}/drift.json" "${temp_dir}/state.json"
preflight >/dev/null 2>&1 && fail desired-label-drift
state "${sha}" "${sha}" "${sha}" true true true
jq '.radar.image = "registry.example/repo/radar@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"' \
  "${temp_dir}/state.json" > "${temp_dir}/wrong-image.json"
mv "${temp_dir}/wrong-image.json" "${temp_dir}/state.json"
preflight >/dev/null 2>&1 && fail target-image-drift
state "${sha}" "${sha}" "${sha}" true true true
jq '.radar.image = "registry.example/repo/not-radar@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' \
  "${temp_dir}/state.json" > "${temp_dir}/wrong-target-repo.json"
mv "${temp_dir}/wrong-target-repo.json" "${temp_dir}/state.json"
preflight >/dev/null 2>&1 && fail target-repository-drift
state "${parent}" "${parent}" "${parent}" true true true
jq '.geoworker.image = "registry.example/repo/not-geoworker@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"' \
  "${temp_dir}/state.json" > "${temp_dir}/wrong-repo.json"
mv "${temp_dir}/wrong-repo.json" "${temp_dir}/state.json"
preflight >/dev/null 2>&1 && fail baseline-repository-drift
# Exercise secure manifest rendering without calling real deployment tools.
mkdir -p "${temp_dir}/bin"
cat > "${temp_dir}/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
cat <<'YAML'
metadata:
  labels:
    release-sha: RELEASE_SHA_PLACEHOLDER
spec:
  serviceAccountName: SERVICE_ACCOUNT_PLACEHOLDER
  containers:
    - image: IMAGE_PLACEHOLDER
      env:
        - name: HOST
          value: HTTP_ALLOWEDHOST_PLACEHOLDER
        - name: OAUTH
          value: GOOGLEOAUTH_CLIENTID_PLACEHOLDER
        - name: PROJECT
          value: PROJECT_ID_PLACEHOLDER
        - name: HTTP_CLOUDFLARESECRET
          value: HTTP_CLOUDFLARESECRET_PLACEHOLDER
        - name: HTTP_RATELIMIT_ENABLED
          value: HTTP_RATELIMIT_ENABLED_PLACEHOLDER
        - name: HTTP_RATELIMIT_RATE
          value: HTTP_RATELIMIT_RATE_PLACEHOLDER
        - name: HTTP_RATELIMIT_BURST
          value: HTTP_RATELIMIT_BURST_PLACEHOLDER
        - name: HTTP_RATELIMIT_EXPIRESIN
          value: HTTP_RATELIMIT_EXPIRESIN_PLACEHOLDER
YAML
MOCK
cat > "${temp_dir}/bin/gcloud" <<'MOCK'
#!/usr/bin/env bash
if [ -n "${MOCK_REPLACE_FAIL_TARGET:-}" ]; then
  case "$*" in
    *replace*"/${MOCK_REPLACE_FAIL_TARGET}.yaml"*)
      printf 'mock: replace failed for %s\n' "${MOCK_REPLACE_FAIL_TARGET}" >&2
      exit 1 ;;
  esac
fi
if [ "${1:-}" = run ] && [ "${2:-}" = jobs ] && [ "${3:-}" = replace ]; then
  : > "${MOCK_MUTATION_MARKER}"
  for arg in "$@"; do
    case "${arg}" in
      *.yaml) cp "${arg}" "${MOCK_JOB_MANIFEST}" ;;
    esac
  done
fi
if [ "${1:-}" = run ] && [ "${2:-}" = services ] && [ "${3:-}" = replace ]; then
  : > "${MOCK_MUTATION_MARKER}"
fi
if [ "${MOCK_REVISION_NOT_FOUND_ONCE:-false}" = true ] \
  && [ "${1:-}" = run ] && [ "${2:-}" = revisions ] && [ "${3:-}" = describe ] \
  && [ "${4:-}" = radar-ready ] && [ ! -e "${MOCK_REVISION_MARKER}" ]; then
  : > "${MOCK_REVISION_MARKER}"
  printf 'ERROR: (gcloud.run.revisions.describe) Cannot find revision [%s].\n' "${4:-}" >&2
  exit 1
fi
if [ -n "${MOCK_MISSING_TARGET:-}" ] \
  && [ "${1:-}" = run ] \
  && { [ "${2:-}" = jobs ] || [ "${2:-}" = services ]; } \
  && [ "${3:-}" = describe ] \
  && [ "${4:-}" = "${MOCK_MISSING_TARGET}" ]; then
  resource_kind=${2%s}
  printf 'ERROR: (gcloud.run.%s.describe) Cannot find %s [%s].\n' \
    "${2}" "${resource_kind}" "${4}" >&2
  exit 1
fi
if [ -n "${MOCK_STATUS_COLLISION_TARGET:-}" ] \
  && [ "${1:-}" = run ] \
  && { [ "${2:-}" = jobs ] || [ "${2:-}" = services ]; } \
  && [ "${3:-}" = describe ] \
  && [ "${4:-}" = "${MOCK_STATUS_COLLISION_TARGET}" ]; then
  resource_kind=${2%s}
  printf 'ERROR: (gcloud.run.%s.describe) PERMISSION_DENIED: Cannot find %s [%s].\n' \
    "${2}" "${resource_kind}" "${4}" >&2
  exit 1
fi
if [ -n "${MOCK_DESCRIBE_ERROR_TARGET:-}" ] \
  && [ "${1:-}" = run ] \
  && { [ "${2:-}" = jobs ] || [ "${2:-}" = services ]; } \
  && [ "${3:-}" = describe ] \
  && [ "${4:-}" = "${MOCK_DESCRIBE_ERROR_TARGET}" ]; then
  printf 'ERROR: (gcloud.run.%s.describe) PERMISSION_DENIED: Could not find valid credentials for %s.\n' \
    "${2}" "${4}" >&2
  exit 1
fi
if [ -n "${MOCK_NO_IMAGE_TARGET:-}" ]; then
  case "$*" in
    "run jobs describe ${MOCK_NO_IMAGE_TARGET} "*)
      jq -cn --arg sha "${MOCK_SHA}" '{metadata:{labels:{"release-sha":$sha}},spec:{template:{spec:{template:{spec:{containers:[{}]}}}}}}'
      exit 0 ;;
    "run revisions describe ${MOCK_NO_IMAGE_TARGET} "*)
      jq -cn --arg sha "${MOCK_SHA}" '{metadata:{labels:{"release-sha":$sha}},spec:{containers:[{}]}}'
      exit 0 ;;
  esac
fi
if [ -n "${MOCK_BASELINE_MISSING_TARGET:-}" ]; then
  case "$*" in
    *"artifacts docker images describe registry.example/repo/${MOCK_BASELINE_MISSING_TARGET}:${MOCK_SHA}"*)
      printf 'ERROR: (gcloud.artifacts.docker.images.describe) Image not found.\n' >&2
      exit 1 ;;
  esac
fi
if [ -n "${MOCK_BASELINE_ERROR_TARGET:-}" ]; then
  case "$*" in
    *"artifacts docker images describe registry.example/repo/${MOCK_BASELINE_ERROR_TARGET}:${MOCK_SHA}"*)
      printf 'ERROR: (gcloud.artifacts.docker.images.describe) PERMISSION_DENIED: Could not inspect baseline image.\n' >&2
      exit 1 ;;
  esac
fi
case "$*" in
  *"artifacts docker images describe registry.example/repo/radar:${MOCK_SHA}"*)
    printf '%s\n' "${MOCK_RADAR}" ;;
  *"artifacts docker images describe registry.example/repo/geoworker:${MOCK_SHA}"*)
    printf '%s\n' "${MOCK_GEOWORKER}" ;;
  *"artifacts docker images describe registry.example/repo/device-cleanup:${MOCK_SHA}"*)
    printf '%s\n' "${MOCK_CLEANUP}" ;;
  'run services describe radar '*'--format=json'*)
    jq -cn --arg sha "${MOCK_SHA}" '{metadata:{labels:{"release-sha":$sha}},status:{latestReadyRevisionName:"radar-ready",conditions:[{type:"Ready",status:"True"}]}}'
    ;;
  'run services describe geoworker '*'--format=json'*)
    jq -cn --arg sha "${MOCK_SHA}" '{metadata:{labels:{"release-sha":$sha}},status:{latestReadyRevisionName:"geoworker-ready",conditions:[{type:"Ready",status:"True"}]}}'
    ;;
  'run revisions describe radar-ready '*)
    jq -cn --arg sha "${MOCK_SHA}" --arg image "${MOCK_RADAR}" '{metadata:{labels:{"release-sha":$sha}},spec:{containers:[{image:$image}]}}'
    ;;
  'run revisions describe geoworker-ready '*)
    jq -cn --arg sha "${MOCK_SHA}" --arg image "${MOCK_GEOWORKER}" '{metadata:{labels:{"release-sha":$sha}},spec:{containers:[{image:$image}]}}'
    ;;
  'run jobs describe device-cleanup '*)
    jq -cn --arg sha "${MOCK_SHA}" --arg image "${MOCK_CLEANUP}" '{metadata:{labels:{"release-sha":$sha}},spec:{template:{spec:{template:{spec:{containers:[{image:$image}]}}}}}}'
    ;;
esac
exit 0
MOCK
chmod +x "${temp_dir}/bin/kubectl" "${temp_dir}/bin/gcloud"
release_functions="${temp_dir}/release-functions.sh"
# The marker must exist, or the extraction below would silently produce a copy
# of the whole script and every sourced-function test would change meaning.
grep -Fxq '# === dispatch ===' "${script_dir}/release.sh" || fail dispatch-marker-missing
sed '/^# === dispatch ===$/,$d' "${script_dir}/release.sh" > "${release_functions}"
common_env=(PATH="${temp_dir}/bin:${PATH}" RELEASE_SHA="${sha}" BUNDLE_FILE="${bundle}" PROJECT_ID=test PROJECT_NUMBER=123456 REGION=region REGISTRY=registry.example OWNER=repo RUNTIME_SA_EMAIL=runtime@example.invalid ALLOWED_HOST=radar.example.invalid CORS_ALLOWED_ORIGINS=https://app.example.invalid RATE_LIMIT_ENABLED=true RATE_LIMIT_RATE=12 RATE_LIMIT_BURST=40 RATE_LIMIT_EXPIRES_IN=5m GOOGLE_OAUTH_CLIENT_ID=oauth MOCK_JOB_MANIFEST="${temp_dir}/captured-job.yaml" MOCK_MUTATION_MARKER="${temp_dir}/mutation.marker")
run_live_baseline_preflight() {
  local stdout_file=$1 stderr_file=$2
  shift 2
  (
    cd "${git_dir}"
    env "${common_env[@]}" \
      MOCK_SHA="${parent}" MOCK_RADAR="registry.example/repo/radar@${baseline_digest}" \
      MOCK_GEOWORKER="registry.example/repo/geoworker@${baseline_digest}" \
      MOCK_CLEANUP="registry.example/repo/device-cleanup@${baseline_digest}" \
      RELEASE_STATE_FILE="${temp_dir}/state.json" ALLOW_RELEASE_FIXTURES=true \
      "$@" bash "${script_dir}/release.sh" preflight
  ) >"${stdout_file}" 2>"${stderr_file}"
}

# A reclaimed baseline tag is diagnostic only: preflight keeps the fleet
# baseline and skips only the tag-to-running-digest equality check.
state "${parent}" "${parent}" "${parent}" true true true
baseline_missing_stdout="${temp_dir}/baseline-missing-stdout.txt"
baseline_missing_stderr="${temp_dir}/baseline-missing-stderr.txt"
run_live_baseline_preflight "${baseline_missing_stdout}" "${baseline_missing_stderr}" \
  MOCK_BASELINE_MISSING_TARGET=radar \
  || fail baseline-missing-preflight
jq -e --arg baseline "${parent}" '.baseline_sha == $baseline' <(tail -n 1 "${baseline_missing_stdout}") >/dev/null \
  || fail baseline-missing-baseline-sha
grep -F '::warning::Baseline tag for target radar' "${baseline_missing_stdout}" >/dev/null \
  || fail baseline-missing-warning-target
grep -F "label ${parent}" "${baseline_missing_stdout}" >/dev/null \
  || fail baseline-missing-warning-label
grep -F 'no longer present in the registry (most likely reclaimed by the repository cleanup policy); skipping the tag-to-running-digest check.' \
  "${baseline_missing_stdout}" >/dev/null \
  || fail baseline-missing-warning-policy

# A non-not-found baseline lookup error must remain a hard failure.
baseline_error_stdout="${temp_dir}/baseline-error-stdout.txt"
baseline_error_stderr="${temp_dir}/baseline-error-stderr.txt"
if run_live_baseline_preflight "${baseline_error_stdout}" "${baseline_error_stderr}" \
  MOCK_BASELINE_ERROR_TARGET=radar; then
  fail baseline-error-preflight-open
fi
grep -F 'PERMISSION_DENIED: Could not inspect baseline image.' "${baseline_error_stderr}" >/dev/null \
  || fail baseline-error-source-message
grep -F "cannot resolve baseline tag radar:${parent}" "${baseline_error_stderr}" >/dev/null \
  || fail baseline-error-message

# A present baseline tag that resolves to a different digest must still fail.
state "${parent}" "${parent}" "${parent}" true true true
jq '.geoworker.image = "registry.example/repo/geoworker@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"' \
  "${temp_dir}/state.json" > "${temp_dir}/baseline-mismatch.json"
mv "${temp_dir}/baseline-mismatch.json" "${temp_dir}/state.json"
baseline_mismatch_stdout="${temp_dir}/baseline-mismatch-stdout.txt"
baseline_mismatch_stderr="${temp_dir}/baseline-mismatch-stderr.txt"
if run_live_baseline_preflight "${baseline_mismatch_stdout}" "${baseline_mismatch_stderr}"; then
  fail baseline-mismatch-preflight-open
fi
grep -F 'baseline geoworker label does not resolve to its running digest' \
  "${baseline_mismatch_stderr}" >/dev/null || fail baseline-mismatch-message

state "${parent}" "${parent}" "${parent}" true true true
jq '.geoworker.image = "registry.example/repo/geoworker@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"' \
  "${temp_dir}/state.json" > "${temp_dir}/wrong-baseline-digest.json"
mv "${temp_dir}/wrong-baseline-digest.json" "${temp_dir}/state.json"
preflight >/dev/null 2>&1 && fail baseline-label-image-drift

assert_missing_cloud_run_target() {
  local target=$1 snapshot_output preflight_output
  snapshot_output=$(
    cd "${git_dir}"
    env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
      MOCK_MISSING_TARGET="${target}" \
      bash -c 'source "$1"; snapshot' "${script_dir}/release.sh" "${release_functions}"
  )
  jq -e --arg target "${target}" '.[$target].exists == false' <<<"${snapshot_output}" >/dev/null \
    || fail "${target}-not-found-snapshot"
  preflight_output="${temp_dir}/${target}-not-found-preflight.txt"
  if ! env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
    MOCK_MISSING_TARGET="${target}" \
    bash "${script_dir}/release.sh" preflight >"${preflight_output}" 2>&1; then
    fail "${target}-not-found-preflight"
  fi
}
assert_missing_cloud_run_target device-cleanup
assert_missing_cloud_run_target geoworker

# A describe that answers without a container image is a response-shape change,
# not a resource running an empty image. Snapshotting it must fail rather than
# emit a null that only surfaces later as a confusing digest mismatch.
for no_image_target in device-cleanup radar-ready; do
  if (
    cd "${git_dir}"
    env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
      MOCK_NO_IMAGE_TARGET="${no_image_target}" \
      bash -c 'source "$1"; snapshot' "${script_dir}/release.sh" "${release_functions}"
  ) >/dev/null 2>&1; then
    fail "snapshot-accepted-missing-image-${no_image_target}"
  fi
done
describe_error_output="${temp_dir}/describe-error-preflight.txt"
if env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
  MOCK_DESCRIBE_ERROR_TARGET=device-cleanup \
  bash "${script_dir}/release.sh" preflight >"${describe_error_output}" 2>&1; then
  fail describe-error-preflight-open
fi
grep -F 'cannot snapshot device-cleanup' "${describe_error_output}" >/dev/null \
  || fail describe-error-source-message
if grep -F 'invalid Cloud Run state' "${describe_error_output}" >/dev/null; then
  fail describe-error-reached-jq
fi
status_collision_output="${temp_dir}/status-collision-preflight.txt"
if env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
  MOCK_STATUS_COLLISION_TARGET=device-cleanup \
  bash "${script_dir}/release.sh" preflight >"${status_collision_output}" 2>&1; then
  fail status-collision-preflight-open
fi
grep -F 'cannot snapshot device-cleanup' "${status_collision_output}" >/dev/null \
  || fail status-collision-source-message
if grep -F 'invalid Cloud Run state' "${status_collision_output}" >/dev/null; then
  fail status-collision-reached-jq
fi
env "${common_env[@]}" TARGET_ENVIRONMENT=dev bash "${script_dir}/release.sh" deploy
env "${common_env[@]}" TARGET_ENVIRONMENT=prod CLOUDFLARE_ORIGIN_SECRET='safe/+value=' bash "${script_dir}/release.sh" deploy
[ -s "${temp_dir}/captured-job.yaml" ] || fail job-manifest-capture
assert_rejected_runtime_line() {
  local label=$1
  local expected=$2
  local output_file="${temp_dir}/${label}.txt"
  shift 2
  rm -f "${temp_dir}/mutation.marker"
  if env "${common_env[@]}" TARGET_ENVIRONMENT=dev "$@" bash "${script_dir}/release.sh" deploy >"${output_file}" 2>&1; then
    fail "${label}-accepted"
  fi
  grep -F -- "${expected}" "${output_file}" >/dev/null || fail "${label}-reason"
  [ ! -e "${temp_dir}/mutation.marker" ] || fail "${label}-mutated"
}
assert_rejected_runtime_line cors-crlf 'CORS_ALLOWED_ORIGINS must not contain CR or LF' \
  $'CORS_ALLOWED_ORIGINS=https://app.example.invalid\r\nhttps://evil.example.invalid'
assert_rejected_runtime_line rate-enabled-crlf 'RATE_LIMIT_ENABLED must not contain CR or LF' \
  $'RATE_LIMIT_ENABLED=true\nfalse'
assert_rejected_runtime_line rate-crlf 'RATE_LIMIT_RATE must not contain CR or LF' \
  $'RATE_LIMIT_RATE=12\n13'
assert_rejected_runtime_line burst-crlf 'RATE_LIMIT_BURST must not contain CR or LF' \
  $'RATE_LIMIT_BURST=40\n41'
assert_rejected_runtime_line expires-crlf 'RATE_LIMIT_EXPIRES_IN must not contain CR or LF' \
  $'RATE_LIMIT_EXPIRES_IN=5m\n6m'
rm -f "${temp_dir}/mutation.marker"
if env "${common_env[@]}" PROJECT_NUMBER= TARGET_ENVIRONMENT=dev bash "${script_dir}/release.sh" deploy >/dev/null 2>&1; then
  fail deploy-project-number-guard
fi
[ ! -e "${temp_dir}/mutation.marker" ] || fail deploy-mutated-before-project-number
ruby -ryaml -e '
  job = YAML.load_file(ARGV.fetch(0))
  template_metadata = job.fetch("spec").fetch("template").fetch("metadata")
  spec = job.fetch("spec").fetch("template").fetch("spec").fetch("template").fetch("spec")
  container = spec.fetch("containers").first
  raise "job placeholder" if Marshal.dump(job).include?("PLACEHOLDER")
  raise "job label" unless job.fetch("metadata").fetch("labels").fetch("release-sha") == ARGV.fetch(1)
  raise "job template label" unless template_metadata.fetch("labels").fetch("release-sha") == ARGV.fetch(1)
  raise "job image" unless container.fetch("image").include?("@sha256:")
  raise "job secret" unless container.fetch("env").any? { |env| env.fetch("name") == "POSTGRES_MASTER_DSN" }
' "${temp_dir}/captured-job.yaml" "${sha}"
rm -f "${temp_dir}/captured-job.yaml" "${temp_dir}/mutation.marker"
if env "${common_env[@]}" TARGET_ENVIRONMENT=dev MOCK_REPLACE_FAIL_TARGET=geoworker \
  bash "${script_dir}/release.sh" deploy >/dev/null 2>&1; then
  fail deploy-replace-failure-ignored
fi
[ ! -e "${temp_dir}/captured-job.yaml" ] || fail deploy-continued-after-replace-failure
env "${common_env[@]}" TARGET_ENVIRONMENT=prod CLOUDFLARE_ORIGIN_SECRET=$'bad\nvalue' bash "${script_dir}/release.sh" deploy >/dev/null 2>&1 \
  && fail cloudflare-newline
env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
  SKIP_HEALTH=true bash "${script_dir}/release.sh" verify || fail exact-digest-verify-fixture
revision_marker="${temp_dir}/revision-missing.marker"
env "${common_env[@]}" MOCK_SHA="${sha}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
  MOCK_REVISION_NOT_FOUND_ONCE=true MOCK_REVISION_MARKER="${revision_marker}" \
  VERIFY_RETRIES=2 VERIFY_RETRY_DELAY=0 SKIP_HEALTH=true \
  bash "${script_dir}/release.sh" verify || fail revision-describe-retry
# A fleet that never reaches the release SHA must exhaust its retries and fail
# rather than report success. The retry schedule itself is a tuning knob, not
# a contract, so it is deliberately not asserted here.
cat > "${temp_dir}/bin/sleep" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
chmod +x "${temp_dir}/bin/sleep"
verify_output="${temp_dir}/verify-nonconverge.txt"
if env "${common_env[@]}" MOCK_SHA="${parent}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
  SKIP_HEALTH=true \
  bash "${script_dir}/release.sh" verify >"${verify_output}" 2>&1; then
  fail verify-retry-exhausted
fi
grep -F 'released labels, digests, or readiness did not converge' "${verify_output}" >/dev/null \
  || fail verify-retry-message

# Candidate resolution must select the newest complete ancestor when the
# current control commit only changes non-release paths.
resolver_dir="${temp_dir}/resolver-git"
git init -q "${resolver_dir}"
git -C "${resolver_dir}" config user.email test@example.invalid
git -C "${resolver_dir}" config user.name 'Release Resolver Test'
git -C "${resolver_dir}" commit -q --allow-empty -m candidate-base
resolver_candidate=$(git -C "${resolver_dir}" rev-parse HEAD)
git -C "${resolver_dir}" commit -q --allow-empty -m docs-only
resolver_control=$(git -C "${resolver_dir}" rev-parse HEAD)
resolver_bin="${temp_dir}/resolver-bin"
resolver_runner="${temp_dir}/resolver-runner"
mkdir -p "${resolver_bin}" "${resolver_runner}"
cat > "${resolver_bin}/gcloud" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [ "${MOCK_GCLOUD_ERROR:-false}" = true ]; then
  printf 'PERMISSION_DENIED\n' >&2
  exit 1
fi
if [ "${MOCK_INVALID_DIGEST:-false}" = true ]; then
  printf 'registry.example/repo/radar:latest\n'
  exit 0
fi
if [ -n "${MOCK_MISSING_IMAGE_TARGET:-}" ]; then
  case "$*" in
    *"artifacts docker images describe registry.example/repo/${MOCK_MISSING_IMAGE_TARGET}:${MOCK_CANDIDATE_SHA}"*)
      printf 'ERROR: (gcloud.artifacts.docker.images.describe) Image not found.\n' >&2
      exit 1 ;;
  esac
fi
case "$*" in
  *"artifacts docker images describe registry.example/repo/radar:${MOCK_CANDIDATE_SHA}"*)
    printf 'registry.example/repo/radar@sha256:%s\n' "$(printf 'a%.0s' {1..64})" ;;
  *"artifacts docker images describe registry.example/repo/geoworker:${MOCK_CANDIDATE_SHA}"*)
    printf 'registry.example/repo/geoworker@sha256:%s\n' "$(printf 'b%.0s' {1..64})" ;;
  *"artifacts docker images describe registry.example/repo/device-cleanup:${MOCK_CANDIDATE_SHA}"*)
    printf 'registry.example/repo/device-cleanup@sha256:%s\n' "$(printf 'c%.0s' {1..64})" ;;
  *) printf 'ERROR: (gcloud.artifacts.docker.images.describe) Image not found.\n' >&2; exit 1 ;;
esac
MOCK
cat > "${resolver_bin}/gh" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
chmod +x "${resolver_bin}/gcloud" "${resolver_bin}/gh"
run_resolver() {
  local control_sha=$1 output_file=$2 requested_sha=${3:-} missing_image_target=${4:-}
  (
    cd "${resolver_dir}"
    PATH="${resolver_bin}:${PATH}" \
      CONTROL_SHA="${control_sha}" REQUESTED_SHA="${requested_sha}" \
      DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
      GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${output_file}" \
      RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
      MOCK_MISSING_IMAGE_TARGET="${missing_image_target}" \
      TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
      bash "${script_dir}/release.sh" resolve-candidate
  )
}
resolver_output="${temp_dir}/resolver-output"
resolver_bundle="${temp_dir}/resolver-bundle.json"
(
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${resolver_output}" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    bash "${script_dir}/release.sh" resolve-candidate
)
grep -F "release_sha=${resolver_candidate}" "${resolver_output}" >/dev/null || fail resolver-selected-ancestor
resolver_bundle=$(sed -n 's/^path=//p' "${resolver_output}")
[ -f "${resolver_bundle}" ] || fail resolver-bundle
expected_targets=$(jq -c 'keys | sort' "${repo_root}/.github/scripts/release/targets.json")
jq -e --arg sha "${resolver_candidate}" --argjson expected "${expected_targets}" \
  '.release_sha == $sha and (.images | keys | sort) == $expected' "${resolver_bundle}" >/dev/null \
  || fail resolver-bundle-content
resolver_unrelated=$(git -C "${resolver_dir}" commit-tree "$(git -C "${resolver_dir}" rev-parse "${resolver_control}^{tree}")" -m unrelated)
resolver_pin_non_ancestor_output="${temp_dir}/resolver-pin-non-ancestor-output"
if run_resolver "${resolver_control}" "${temp_dir}/resolver-pin-non-ancestor-env" "${resolver_unrelated}" \
  >"${resolver_pin_non_ancestor_output}" 2>&1; then
  fail resolver-pin-non-ancestor-open
fi
grep -F 'not an ancestor' "${resolver_pin_non_ancestor_output}" >/dev/null \
  || fail resolver-pin-non-ancestor-message
# A pin may be abbreviated, because it is typed by hand during a rollback. The
# resolver must expand it to the full SHA the image tags actually use.
resolver_short=$(git -C "${resolver_dir}" rev-parse --short=8 "${resolver_candidate}")
resolver_short_output="${temp_dir}/resolver-short-output"
run_resolver "${resolver_control}" "${resolver_short_output}" "${resolver_short}"
grep -Fx "release_sha=${resolver_candidate}" "${resolver_short_output}" >/dev/null \
  || fail resolver-pin-short-sha-expansion
# Too short to be meaningful.
if run_resolver "${resolver_control}" "${temp_dir}/resolver-pin-tiny-env" "$(printf '%s' "${resolver_candidate}" | cut -c1-4)" \
  >/dev/null 2>&1; then
  fail resolver-pin-too-short-accepted
fi
# A prefix that matches no commit must fail rather than resolve to anything.
if run_resolver "${resolver_control}" "${temp_dir}/resolver-pin-unknown-env" deadbeef \
  >/dev/null 2>&1; then
  fail resolver-pin-unknown-prefix-accepted
fi
# Non-hex input must never reach git.
if run_resolver "${resolver_control}" "${temp_dir}/resolver-pin-ref-env" HEAD \
  >/dev/null 2>&1; then
  fail resolver-pin-ref-name-accepted
fi

resolver_pin_incomplete_output="${temp_dir}/resolver-pin-incomplete-output"
if run_resolver "${resolver_control}" "${temp_dir}/resolver-pin-incomplete-env" "${resolver_candidate}" device-cleanup \
  >"${resolver_pin_incomplete_output}" 2>&1; then
  fail resolver-pin-incomplete-open
fi
grep -F "pinned candidate ${resolver_candidate} has no complete attested image set" \
  "${resolver_pin_incomplete_output}" >/dev/null \
  || fail resolver-pin-incomplete-message

resolver_git_fail_bin="${temp_dir}/resolver-git-fail-bin"
mkdir -p "${resolver_git_fail_bin}"
cp "${resolver_bin}/gcloud" "${resolver_git_fail_bin}/gcloud"
cp "${resolver_bin}/gh" "${resolver_git_fail_bin}/gh"
real_git=$(command -v git)
cat > "${resolver_git_fail_bin}/git" <<'MOCK'
#!/usr/bin/env bash
if [ "${1:-}" = "${MOCK_GIT_FAILURE:-}" ]; then
  printf 'mock git failure: %s\n' "${1}" >&2
  exit 128
fi
exec "${REAL_GIT}" "$@"
MOCK
chmod +x "${resolver_git_fail_bin}/gcloud" "${resolver_git_fail_bin}/gh" "${resolver_git_fail_bin}/git"
for git_failure in diff rev-list; do
  resolver_git_output="${temp_dir}/resolver-${git_failure}-output"
  resolver_git_env="${temp_dir}/resolver-${git_failure}-env"
  if (
    cd "${resolver_dir}"
    PATH="${resolver_git_fail_bin}:${PATH}" REAL_GIT="${real_git}" MOCK_GIT_FAILURE="${git_failure}" \
      CONTROL_SHA="${resolver_control}" \
      DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
      GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${resolver_git_env}" \
      RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
      TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
      bash "${script_dir}/release.sh" resolve-candidate >"${resolver_git_output}" 2>&1
  ); then
    fail "resolver-${git_failure}-failure-open"
  fi
  grep -F "git ${git_failure} failed" "${resolver_git_output}" >/dev/null \
    || fail "resolver-${git_failure}-error-message"
done

resolver_fail_bin="${temp_dir}/resolver-fail-bin"
mkdir -p "${resolver_fail_bin}"
cp "${resolver_bin}/gcloud" "${resolver_fail_bin}/gcloud"
cat > "${resolver_fail_bin}/gh" <<'MOCK'
#!/usr/bin/env bash
exit 1
MOCK
chmod +x "${resolver_fail_bin}/gh"
if (
  cd "${resolver_dir}"
  PATH="${resolver_fail_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-attestation-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    ATTESTATION_RETRIES=1 \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail resolver-attestation-fail-closed
fi

if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-integrity-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    MOCK_INVALID_DIGEST=true \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail resolver-integrity-fallback
else
  integrity_status=$?
  [ "${integrity_status}" -eq 3 ] || fail resolver-integrity-status
fi

if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-error-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    MOCK_GCLOUD_ERROR=true \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail resolver-inspection-fallback
else
  inspection_status=$?
  [ "${inspection_status}" -eq 3 ] || fail resolver-inspection-status
fi

git -C "${resolver_dir}" checkout -q -b impact
printf 'release-impact\n' > "${resolver_dir}/Dockerfile"
git -C "${resolver_dir}" add Dockerfile
git -C "${resolver_dir}" commit -q -m release-impact
resolver_impact_control=$(git -C "${resolver_dir}" rev-parse HEAD)
resolver_pinned_output="${temp_dir}/resolver-pinned-output"
run_resolver "${resolver_impact_control}" "${resolver_pinned_output}" "${resolver_candidate}"
grep -F "release_sha=${resolver_candidate}" "${resolver_pinned_output}" >/dev/null \
  || fail resolver-pinned-sha
resolver_pinned_bundle=$(sed -n 's/^path=//p' "${resolver_pinned_output}")
jq -e --arg sha "${resolver_candidate}" '.release_sha == $sha' "${resolver_pinned_bundle}" >/dev/null \
  || fail resolver-pinned-bundle
if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_impact_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-impact-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail resolver-impact-path-guard
fi

invalid_catalog="${temp_dir}/targets-invalid.json"
jq '.radar.overlay = "missing"' "${repo_root}/.github/scripts/release/targets.json" > "${invalid_catalog}"
if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-invalid-catalog-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    TARGETS_FILE="${invalid_catalog}" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail catalog-path-validation
fi
empty_catalog="${temp_dir}/targets-empty.json"
printf '{}\n' > "${empty_catalog}"
if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-empty-catalog-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    TARGETS_FILE="${empty_catalog}" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail empty-catalog-guard
fi

# --- candidate change detection -------------------------------------------
# A push that touches nothing in the release contract must not republish, and
# a push that cannot be diffed at all must republish rather than assume.
changes_dir="${temp_dir}/changes-git"
git init -q "${changes_dir}"
git -C "${changes_dir}" config user.email test@example.invalid
git -C "${changes_dir}" config user.name 'Release Changes Test'
git -C "${changes_dir}" commit -q --allow-empty -m base
changes_base=$(git -C "${changes_dir}" rev-parse HEAD)
mkdir -p "${changes_dir}/docs"
printf 'docs\n' > "${changes_dir}/docs/note.md"
git -C "${changes_dir}" add docs/note.md
git -C "${changes_dir}" commit -q -m docs-only
changes_docs=$(git -C "${changes_dir}" rev-parse HEAD)
mkdir -p "${changes_dir}/internal"
printf 'code\n' > "${changes_dir}/internal/app.go"
git -C "${changes_dir}" add internal/app.go
git -C "${changes_dir}" commit -q -m code
changes_code=$(git -C "${changes_dir}" rev-parse HEAD)

run_changes() {
  local before=$1 head=$2 catalog=${3:-${repo_root}/.github/scripts/release/targets.json}
  (
    cd "${changes_dir}"
    BEFORE_SHA="${before}" GITHUB_SHA="${head}" TARGETS_FILE="${catalog}" \
      GITHUB_OUTPUT=/dev/stdout bash "${script_dir}/release.sh" candidate-changes
  )
}
run_changes "${changes_base}" "${changes_docs}" | grep -Fx 'candidate=false' >/dev/null \
  || fail changes-docs-only-republished
run_changes "${changes_docs}" "${changes_code}" | grep -Fx 'candidate=true' >/dev/null \
  || fail changes-code-not-detected
run_changes "$(printf '0%.0s' {1..40})" "${changes_code}" | grep -Fx 'candidate=true' >/dev/null \
  || fail changes-force-push-not-republished
run_changes "$(printf 'f%.0s' {1..40})" "${changes_code}" | grep -Fx 'candidate=true' >/dev/null \
  || fail changes-unreachable-before-not-republished
if run_changes "${changes_base}" "${changes_docs}" "${empty_catalog}" >/dev/null 2>&1; then
  fail changes-empty-catalog-guard
fi

# --- migration phase selection --------------------------------------------
migration_dir="${temp_dir}/migration-git"
git init -q "${migration_dir}"
git -C "${migration_dir}" config user.email test@example.invalid
git -C "${migration_dir}" config user.name 'Release Migration Test'
mkdir -p "${migration_dir}/database/migration/postgres" \
  "${migration_dir}/database/migration/supabase/pre" \
  "${migration_dir}/database/migration/supabase/post"
printf 'base\n' > "${migration_dir}/database/migration/postgres/0001.sql"
git -C "${migration_dir}" add -A
git -C "${migration_dir}" commit -q -m base
migration_base=$(git -C "${migration_dir}" rev-parse HEAD)
printf 'pre\n' > "${migration_dir}/database/migration/supabase/pre/0001.sql"
git -C "${migration_dir}" add -A
git -C "${migration_dir}" commit -q -m schema
migration_schema=$(git -C "${migration_dir}" rev-parse HEAD)
printf 'docs\n' > "${migration_dir}/README.md"
git -C "${migration_dir}" add -A
git -C "${migration_dir}" commit -q -m docs
migration_docs=$(git -C "${migration_dir}" rev-parse HEAD)

run_migrations() {
  local release=$1 baseline=$2 requested=${3:-} ack=${4:-false}
  (
    cd "${migration_dir}"
    RELEASE_SHA="${release}" BASELINE_SHA="${baseline}" REQUESTED_SHA="${requested}" \
      ACKNOWLEDGE_SCHEMA_DRIFT="${ack}" GITHUB_OUTPUT=/dev/stdout \
      bash "${script_dir}/release.sh" migration-phases
  )
}
# No baseline at all: bootstrap must check every phase.
run_migrations "${migration_schema}" '' | grep -Fx 'any=true' >/dev/null \
  || fail migrations-bootstrap
# Nothing moved.
run_migrations "${migration_schema}" "${migration_schema}" | grep -Fx 'any=false' >/dev/null \
  || fail migrations-same-sha
# Only the pre directory changed between baseline and release.
migration_out=$(run_migrations "${migration_schema}" "${migration_base}")
printf '%s\n' "${migration_out}" | grep -Fx 'pre=true' >/dev/null || fail migrations-pre
printf '%s\n' "${migration_out}" | grep -Fx 'shared=false' >/dev/null || fail migrations-shared-false
# A docs-only advance must not select any phase.
run_migrations "${migration_docs}" "${migration_schema}" | grep -Fx 'any=false' >/dev/null \
  || fail migrations-docs-only
# A pinned release that does not cross a migration change is allowed through.
run_migrations "${migration_docs}" "${migration_schema}" "${migration_docs}" \
  | grep -Fx 'any=false' >/dev/null || fail migrations-pinned-clean
# A pinned release that moves back across a migration change must stop: the
# schema is forward-only, so this would run old code against a newer schema.
pinned_drift_output="${temp_dir}/pinned-drift.txt"
if run_migrations "${migration_base}" "${migration_schema}" "${migration_base}" \
  >"${pinned_drift_output}" 2>&1; then
  fail migrations-pinned-drift-allowed
fi
grep -F 'crosses a migration change' "${pinned_drift_output}" >/dev/null \
  || fail migrations-pinned-drift-message
# ...and must proceed once that is explicitly acknowledged.
run_migrations "${migration_base}" "${migration_schema}" "${migration_base}" true \
  | grep -Fx 'any=false' >/dev/null || fail migrations-pinned-drift-acknowledged

goose_version=$(make -C "${repo_root}" -s --no-print-directory print-goose-version)
printf '%s\n' "${goose_version}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail goose-version-format

workflow="${repo_root}/.github/workflows/release-cloud-run.yml"
ci_workflow="${repo_root}/.github/workflows/ci.yml"
operations_workflow="${repo_root}/.github/workflows/cloud-run-operations.yml"

# Execute the production promotion with mocked registry tools so a permission
# error cannot silently authorize a gcrane copy.
promote_targets="${temp_dir}/promote-targets.json"
printf '{"radar":{"kind":"service","overlay":"radar","order":1}}\n' > "${promote_targets}"
promote_runner="${temp_dir}/promote-runner"
promote_bin="${temp_dir}/promote-bin"
mkdir -p "${promote_runner}/gcrane" "${promote_bin}"
cat > "${promote_bin}/gcloud" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = auth ]; then
  exit 0
fi
if [ "${1:-}" = artifacts ] && [ "${2:-}" = docker ] && [ "${3:-}" = images ] && [ "${4:-}" = describe ]; then
  count=0
  if [ -f "${MOCK_PROMOTE_DESCRIBE_COUNT}" ]; then
    count=$(<"${MOCK_PROMOTE_DESCRIBE_COUNT}")
  fi
  count=$((count + 1))
  printf '%s\n' "${count}" > "${MOCK_PROMOTE_DESCRIBE_COUNT}"
  if [ "${count}" -eq 1 ]; then
    case "${MOCK_PROMOTE_ERROR:-}" in
      permission)
        printf 'ERROR: (gcloud.artifacts.docker.images.describe) PERMISSION_DENIED: Could not find valid credentials.\n' >&2
        exit 1
        ;;
      not-found)
        printf 'ERROR: (gcloud.artifacts.docker.images.describe) Image not found.\n' >&2
        exit 1
        ;;
    esac
  fi
  printf '%s\n' "${MOCK_PROMOTE_IMAGE}"
  exit 0
fi
printf 'unexpected gcloud invocation: %s\n' "$*" >&2
exit 1
MOCK
cat > "${promote_runner}/gcrane/gcrane" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" != copy ] \
  || [ "${2:-}" != "${MOCK_PROMOTE_EXPECTED_SOURCE}" ] \
  || [ "${3:-}" != "${MOCK_PROMOTE_EXPECTED_DESTINATION}" ] \
  || [ "$#" -ne 3 ]; then
  printf 'unexpected gcrane invocation: %s\n' "$*" >&2
  exit 1
fi
: > "${MOCK_PROMOTE_COPY_MARKER}"
MOCK
chmod +x "${promote_bin}/gcloud" "${promote_runner}/gcrane/gcrane"
promote_image="prod.example/repo/radar@sha256:$(printf 'a%.0s' {1..64})"
promote_source="${digest_a}"
promote_destination="prod.example/repo/radar:${sha}"
run_promote() {
  local error_mode=$1 copy_marker=$2 describe_count=$3 output_file=$4
  (
    cd "${repo_root}"
    PATH="${promote_bin}:${PATH}" \
      OWNER=repo DEV_REGISTRY=dev.example REGISTRY=prod.example PROJECT_ID=test \
      SOURCE_BUNDLE="${bundle}" RELEASE_SHA="${sha}" TARGETS_FILE="${promote_targets}" \
      RUNNER_TEMP="${promote_runner}" GITHUB_OUTPUT="${output_file}" \
      MOCK_PROMOTE_ERROR="${error_mode}" MOCK_PROMOTE_COPY_MARKER="${copy_marker}" \
      MOCK_PROMOTE_DESCRIBE_COUNT="${describe_count}" MOCK_PROMOTE_IMAGE="${promote_image}" \
      MOCK_PROMOTE_EXPECTED_SOURCE="${promote_source}" MOCK_PROMOTE_EXPECTED_DESTINATION="${promote_destination}" \
      GCRANE="${promote_runner}/gcrane/gcrane" \
      bash "${script_dir}/release.sh" promote
  )
}
promote_permission_marker="${temp_dir}/promote-permission-copy.marker"
if run_promote permission "${promote_permission_marker}" "${temp_dir}/promote-permission-count" "${temp_dir}/promote-permission-output" >/dev/null 2>&1; then
  fail prod-promote-permission-accepted
fi
[ ! -e "${promote_permission_marker}" ] || fail prod-promote-copy-after-permission
promote_not_found_marker="${temp_dir}/promote-not-found-copy.marker"
run_promote not-found "${promote_not_found_marker}" "${temp_dir}/promote-not-found-count" "${temp_dir}/promote-not-found-output" >/dev/null 2>&1 \
  || fail prod-promote-not-found-fallback
[ -e "${promote_not_found_marker}" ] || fail prod-promote-copy-missing

grep -F 'group: candidate-${{ github.sha }}-${{ matrix.target }}' "${ci_workflow}" >/dev/null || fail matrix-concurrency-scope
! grep -F 'verified=false' "${repo_root}/.github/scripts/release/release.sh" >/dev/null || fail attestation-fallback
grep -F 'has no valid attestation for' "${repo_root}/.github/scripts/release/release.sh" >/dev/null || fail attestation-fail-closed
! grep -F 'RELEASE_SHA: ${{ github.sha }}' "${workflow}" >/dev/null || fail workflow-release-sha-coupling

ruby -ryaml -e '
  ci = YAML.load_file(ARGV.fetch(0))
  release = YAML.load_file(ARGV.fetch(1))
  job = YAML.load_file(ARGV.fetch(2))
  operations = YAML.load_file(ARGV.fetch(3))
  raise "ci permissions" unless ci.fetch("permissions").fetch("contents") == "read"
  publish = ci.fetch("jobs").fetch("publish-candidate")
  attest = publish.fetch("steps").find { |step| step["name"] == "Attest staged image digest" }
  verify = publish.fetch("steps").find { |step| step["name"] == "Verify candidate attestation" }
  finalize = publish.fetch("steps").find { |step| step["name"] == "Publish verified candidate SHA tag" }
  publish_names = publish.fetch("steps").map { |step| step.fetch("name") }
  raise "attestation finalization order" unless publish_names.index(attest.fetch("name")) < publish_names.index(verify.fetch("name")) && publish_names.index(verify.fetch("name")) < publish_names.index(finalize.fetch("name"))
  raise "existing attestation fail-closed" unless verify.fetch("run").include?("Existing SHA tag")
  # The only path from the release_sha input into the script. Break this wiring
  # and a pinned rollback silently resolves the newest candidate and runs
  # migrations instead, because every REQUESTED_SHA branch downstream is
  # guarded by [ -n "${REQUESTED_SHA:-}" ] and simply stops applying.
  release_env = release.fetch("jobs").fetch("release").fetch("env")
  raise "requested sha env" unless release_env.fetch("REQUESTED_SHA") == "${{ inputs.release_sha }}"
  # Same silent mode switch from the other end: a non-empty default pins every
  # dispatch that accepts it, so releases stop advancing and stop migrating. The
  # sibling `required` and `type` fields are not asserted because breaking them
  # makes the intended dispatch unavailable or fail before mutation, rather
  # than changing what it does.
  release_sha_input = release.fetch(true).fetch("workflow_dispatch").fetch("inputs").fetch("release_sha")
  raise "release sha input default" unless release_sha_input.fetch("default") == ""
  steps = release.fetch("jobs").fetch("release").fetch("steps")
  names = steps.map { |step| step.fetch("name") }
  raise "release sha inline" if steps.any? { |step| step.fetch("run", "").include?("${{ inputs.release_sha }}") }
  raise "registry auth order" unless names.index("Configure dev Registry auth") < names.index("Resolve and verify candidate digests")
  preflight = steps.find { |step| step.fetch("name") == "Preflight target state" }
  promote = steps.find { |step| step.fetch("name") == "Promote exact digests to prod" }
  raise "preflight unconditional" unless preflight["if"].nil?
  raise "preflight order" unless names.index(preflight.fetch("name")) < names.index(promote.fetch("name"))
  # The not-found matchers decide whether a resource is absent, and that answer
  # authorizes creates and copies. Proving the wording still holds is only
  # useful before anything acts on it.
  contract = steps.find { |step| step.fetch("name") == "Verify gcloud error contract" }
  raise "error contract unconditional" unless contract["if"].nil?
  raise "error contract order" unless names.index(contract.fetch("name")) < names.index(preflight.fetch("name"))
  # A pinned release must be able to refuse itself, so the acknowledgement has
  # to reach the script rather than only existing as a dispatch input.
  raise "schema drift ack env" unless release_env.fetch("ACKNOWLEDGE_SCHEMA_DRIFT") == "${{ inputs.acknowledge_schema_drift }}"
  task = job.fetch("spec").fetch("template").fetch("spec").fetch("template").fetch("spec")
  container = task.fetch("containers").first
  raise "job secret" unless container.fetch("env").any? { |env| env.fetch("name") == "POSTGRES_MASTER_DSN" }
  operation = operations.fetch("jobs").fetch("operate")
  # The operations job blocks on `gcloud run jobs execute --wait`, so its timeout
  # must exceed the worst-case Cloud Run task runtime including retries.
  worst_case_minutes = (task.fetch("maxRetries") + 1) * task.fetch("timeoutSeconds") / 60.0
  raise "operations timeout below worst-case job runtime" unless operation.fetch("timeout-minutes") > worst_case_minutes
  operation_steps = operation.fetch("steps")
  validate = operation_steps.find { |step| step.fetch("name") == "Validate operation configuration" }
  raise "operation current-main guard" unless validate.fetch("run").include?("git/ref/heads/main")
  checkout = operation_steps.find { |step| step.fetch("name") == "Checkout trusted operations automation" }
  raise "operation checkout pinned" unless checkout.fetch("uses") =~ %r{\Aactions/checkout@[0-9a-f]{40}\z}
  raise "operation checkout ref" unless checkout.fetch("with").fetch("ref") == "${{ github.sha }}"
  raise "operation checkout credentials" unless checkout.fetch("with").fetch("persist-credentials") == false
  operation_names = operation_steps.map { |step| step.fetch("name") }
  auth = operation_steps.find { |step| step.fetch("name") == "Google Auth" }
  raise "operation checkout order" unless operation_names.index(validate.fetch("name")) < operation_names.index(checkout.fetch("name")) && operation_names.index(checkout.fetch("name")) < operation_names.index(auth.fetch("name"))
' "${repo_root}/.github/workflows/ci.yml" "${workflow}" "${repo_root}/deploy/cloud-run/jobs/device-cleanup.yaml" "${operations_workflow}"

printf 'release checks: PASS\n'
