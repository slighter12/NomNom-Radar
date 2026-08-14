#!/usr/bin/env bash

set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "${script_dir}/../../.." && pwd)
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nomnom-release-test.XXXXXX")
trap 'rm -rf "${temp_dir}"' EXIT HUP INT TERM

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
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
[ "$(preflight | jq -r .baseline_sha)" = "${parent}" ] || fail forward
state "${sha}" "${sha}" "${sha}" true true true
[ "$(preflight | jq -r .baseline_sha)" = "${sha}" ] || fail complete
state "${sha}" "${sha}" "${sha}" true true true
jq 'with_entries(.value.image |= sub("^registry.example/"; "prod.example/"))' \
  "${temp_dir}/state.json" > "${temp_dir}/prod-target.json"
mv "${temp_dir}/prod-target.json" "${temp_dir}/state.json"
[ "$(TEST_REGISTRY=prod.example preflight | jq -r .baseline_sha)" = "${sha}" ] || fail prod-cross-registry-target
state "${parent}" "${sha}" "${parent}" true true true
[ "$(preflight | jq -r .baseline_sha)" = "${parent}" ] || fail partial
state_four "${sha}" "${parent}" "${parent}" "${parent}" true true true true
[ "$(preflight_four | jq -r .baseline_sha)" = "${parent}" ] || fail partial-four-target-catalog
state "${sha}" '' '' true false true
expect_empty_baseline target-missing-unlabeled
state "${parent}" '' '' true false true
preflight >/dev/null 2>&1 && fail baseline-with-missing
state "${future}" "${future}" "${future}" true true true
preflight >/dev/null 2>&1 && fail rollback
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
state "${parent}" "${parent}" "${parent}" true true true
jq '.geoworker.image = "registry.example/repo/geoworker@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"' \
  "${temp_dir}/state.json" > "${temp_dir}/wrong-baseline-digest.json"
mv "${temp_dir}/wrong-baseline-digest.json" "${temp_dir}/state.json"
preflight >/dev/null 2>&1 && fail baseline-label-image-drift

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
  printf 'NOT_FOUND\n' >&2
  exit 1
fi
case "$*" in
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
common_env=(PATH="${temp_dir}/bin:${PATH}" RELEASE_SHA="${sha}" BUNDLE_FILE="${bundle}" PROJECT_ID=test PROJECT_NUMBER=123456 REGION=region RUNTIME_SA_EMAIL=runtime@example.invalid ALLOWED_HOST=radar.example.invalid GOOGLE_OAUTH_CLIENT_ID=oauth MOCK_JOB_MANIFEST="${temp_dir}/captured-job.yaml" MOCK_MUTATION_MARKER="${temp_dir}/mutation.marker")
env "${common_env[@]}" TARGET_ENVIRONMENT=dev bash "${script_dir}/release.sh" deploy
env "${common_env[@]}" TARGET_ENVIRONMENT=prod CLOUDFLARE_ORIGIN_SECRET='safe/+value=' bash "${script_dir}/release.sh" deploy
[ -s "${temp_dir}/captured-job.yaml" ] || fail job-manifest-capture
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
sleep_log="${temp_dir}/sleep.log"
cat > "${temp_dir}/bin/sleep" <<'MOCK'
#!/usr/bin/env bash
: "${MOCK_SLEEP_LOG:=/dev/null}"
printf '%s\n' "$1" >> "${MOCK_SLEEP_LOG}"
MOCK
chmod +x "${temp_dir}/bin/sleep"
verify_output="${temp_dir}/verify-nonconverge.txt"
if env "${common_env[@]}" MOCK_SHA="${parent}" MOCK_RADAR="${digest_a}" MOCK_GEOWORKER="${digest_b}" MOCK_CLEANUP="${digest_c}" \
  MOCK_SLEEP_LOG="${sleep_log}" VERIFY_RETRIES=5 VERIFY_RETRY_DELAY=5 SKIP_HEALTH=true \
  bash "${script_dir}/release.sh" verify >"${verify_output}" 2>&1; then
  fail verify-retry-exhausted
fi
grep -F 'released labels, digests, or readiness did not converge' "${verify_output}" >/dev/null \
  || fail verify-retry-message
printf '5\n10\n20\n30\n' | diff -u - "${sleep_log}" || fail verify-retry-backoff

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
case "$*" in
  *"artifacts docker images describe registry.example/repo/radar:${MOCK_CANDIDATE_SHA}"*)
    printf 'registry.example/repo/radar@sha256:%s\n' "$(printf 'a%.0s' {1..64})" ;;
  *"artifacts docker images describe registry.example/repo/geoworker:${MOCK_CANDIDATE_SHA}"*)
    printf 'registry.example/repo/geoworker@sha256:%s\n' "$(printf 'b%.0s' {1..64})" ;;
  *"artifacts docker images describe registry.example/repo/device-cleanup:${MOCK_CANDIDATE_SHA}"*)
    printf 'registry.example/repo/device-cleanup@sha256:%s\n' "$(printf 'c%.0s' {1..64})" ;;
  *) echo 'NOT_FOUND' >&2; exit 1 ;;
esac
MOCK
cat > "${resolver_bin}/gh" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
chmod +x "${resolver_bin}/gcloud" "${resolver_bin}/gh"
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
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
    bash "${script_dir}/release.sh" resolve-candidate
)
grep -F "release_sha=${resolver_candidate}" "${resolver_output}" >/dev/null || fail resolver-selected-ancestor
resolver_bundle=$(sed -n 's/^path=//p' "${resolver_output}")
[ -f "${resolver_bundle}" ] || fail resolver-bundle
expected_targets=$(jq -c 'keys | sort' "${repo_root}/.github/scripts/release/targets.json")
jq -e --arg sha "${resolver_candidate}" --argjson expected "${expected_targets}" \
  '.release_sha == $sha and (.images | keys | sort) == $expected' "${resolver_bundle}" >/dev/null \
  || fail resolver-bundle-content

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
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
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
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
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
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
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
if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_impact_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-impact-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail resolver-impact-path-guard
fi

missing_impact_paths="${temp_dir}/impact-paths-missing.txt"
grep -v '^Dockerfile$' "${repo_root}/.github/scripts/release/impact-paths.txt" > "${missing_impact_paths}"
if (
  cd "${resolver_dir}"
  PATH="${resolver_bin}:${PATH}" \
    CONTROL_SHA="${resolver_control}" \
    DEV_PROJECT_ID=test DEV_REGISTRY=registry.example OWNER=repo \
    GITHUB_REPOSITORY=repo/test GITHUB_OUTPUT="${temp_dir}/resolver-missing-impact-output" \
    RUNNER_TEMP="${resolver_runner}" MOCK_CANDIDATE_SHA="${resolver_candidate}" \
    TARGETS_FILE="${repo_root}/.github/scripts/release/targets.json" \
    IMPACT_PATHS_FILE="${missing_impact_paths}" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail impact-path-manifest-guard
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
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
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
    IMPACT_PATHS_FILE="${repo_root}/.github/scripts/release/impact-paths.txt" \
    bash "${script_dir}/release.sh" resolve-candidate >/dev/null 2>&1
); then
  fail empty-catalog-guard
fi

goose_version=$(make -C "${repo_root}" -s --no-print-directory print-goose-version)
printf '%s\n' "${goose_version}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail goose-version-format

workflow="${repo_root}/.github/workflows/release-cloud-run.yml"
ci_workflow="${repo_root}/.github/workflows/ci.yml"
operations_workflow="${repo_root}/.github/workflows/cloud-run-operations.yml"
grep -F 'group: candidate-${{ github.sha }}-${{ matrix.target }}' "${ci_workflow}" >/dev/null || fail matrix-concurrency-scope
! grep -F 'verified=false' "${repo_root}/.github/scripts/release/release.sh" >/dev/null || fail attestation-fallback
grep -F 'has no valid attestation for' "${repo_root}/.github/scripts/release/release.sh" >/dev/null || fail attestation-fail-closed
grep -F 'Configure dev Registry auth' "${workflow}" >/dev/null || fail workflow-dev-registry-auth
! grep -F 'RELEASE_SHA: ${{ github.sha }}' "${workflow}" >/dev/null || fail workflow-release-sha-coupling
grep -F -- '--connect-timeout 10 --max-time 60' "${repo_root}/.github/scripts/release/release.sh" >/dev/null || fail health-timeout

ruby -ryaml -e '
  ci = YAML.load_file(ARGV.fetch(0))
  release = YAML.load_file(ARGV.fetch(1))
  job = YAML.load_file(ARGV.fetch(2))
  operations = YAML.load_file(ARGV.fetch(3))
  release_script = File.read(ARGV.fetch(4))
  raise "ci permissions" unless ci.fetch("permissions").fetch("contents") == "read"
  raise "checks timeout" unless ci.fetch("jobs").fetch("checks").fetch("timeout-minutes") == 30
  raise "candidate changes timeout" unless ci.fetch("jobs").fetch("candidate-changes").fetch("timeout-minutes") == 10
  raise "publish timeout" unless ci.fetch("jobs").fetch("publish-candidate").fetch("timeout-minutes") == 30
  publish = ci.fetch("jobs").fetch("publish-candidate")
  changes = ci.fetch("jobs").fetch("candidate-changes").fetch("steps").find { |step| step["id"] == "changes" }
  quote = 39.chr
  empty_target_guard = "targets=$(jq -e -c #{quote}keys | select(length > 0)#{quote}"
  raise "empty target catalog guard" unless changes.fetch("run").include?(empty_target_guard)
  raise "before env" unless changes.fetch("env").fetch("BEFORE_SHA") == "${{ github.event.before }}"
  raise "before inline" if changes.fetch("run").include?("${{ github.event.before }}")
  raise "ci matrix" unless publish.fetch("strategy").fetch("matrix").fetch("target").include?("needs.candidate-changes.outputs.targets")
  raise "candidate concurrency" unless publish.fetch("concurrency").fetch("group").include?("matrix.target")
  stage = publish.fetch("steps").find { |step| step["id"] == "stage" }
  attest = publish.fetch("steps").find { |step| step["name"] == "Attest staged image digest" }
  verify = publish.fetch("steps").find { |step| step["name"] == "Verify candidate attestation" }
  finalize = publish.fetch("steps").find { |step| step["name"] == "Publish verified candidate SHA tag" }
  raise "lowercase attestation subject" unless attest.fetch("with").fetch("subject-name").include?("steps.stage.outputs.owner")
  raise "stage owner output" unless stage.fetch("run").include?("echo \"owner=${owner}\"")
  publish_names = publish.fetch("steps").map { |step| step.fetch("name") }
  raise "attestation finalization order" unless publish_names.index(attest.fetch("name")) < publish_names.index(verify.fetch("name")) && publish_names.index(verify.fetch("name")) < publish_names.index(finalize.fetch("name"))
  raise "existing attestation fail-closed" unless verify.fetch("run").include?("Existing SHA tag")
  raise "staging cleanup command" unless finalize.fetch("run").include?("artifacts docker tags delete")
  raise "staging cleanup warning" unless finalize.fetch("run").include?("immutable-tag repositories may retain it")
  steps = release.fetch("jobs").fetch("release").fetch("steps")
  names = steps.map { |step| step.fetch("name") }
  raise "registry auth order" unless names.index("Configure dev Registry auth") < names.index("Resolve and verify candidate digests")
  raise "mutation guard order" unless names.index("Recheck current main before mutation") < names.index("Promote exact digests to prod")
  gcrane = steps.find { |step| step.fetch("name") == "Install pinned gcrane" }
  raise "gcrane connect timeout" unless gcrane.fetch("run").include?("--connect-timeout 10")
  raise "gcrane transfer timeout" unless gcrane.fetch("run").include?("--max-time 300")
  raise "gcrane retry timeout" unless gcrane.fetch("run").include?("--retry-max-time 120")
  raise "revision retry handling" unless release_script.include?("elif not_found \"${revision_error}\"")
  raise "verify retry count" unless release_script.include?("VERIFY_RETRIES:-10")
  raise "verify retry delay" unless release_script.include?("VERIFY_RETRY_DELAY:-5")
  raise "verify retry cap" unless release_script.include?("max_delay=30") && release_script.include?("delay=$((delay * 2))")
  raise "release timeout" unless release.fetch("jobs").fetch("release").fetch("timeout-minutes") == 90
  raise "project number" unless release.fetch("jobs").fetch("release").fetch("env").fetch("PROJECT_NUMBER") == "${{ vars.GCP_PROJECT_NUMBER }}"
  preflight = steps.find { |step| step.fetch("name") == "Preflight target state" }
  promote = steps.find { |step| step.fetch("name") == "Promote exact digests to prod" }
  raise "preflight unconditional" unless preflight["if"].nil?
  raise "preflight order" unless names.index(preflight.fetch("name")) < names.index(promote.fetch("name"))
  raise "promotion bundle source" unless promote.fetch("run").include?(".images[$target]")
  container = job.fetch("spec").fetch("template").fetch("spec").fetch("template").fetch("spec").fetch("containers").first
  raise "job secret" unless container.fetch("env").any? { |env| env.fetch("name") == "POSTGRES_MASTER_DSN" }
  operation = operations.fetch("jobs").fetch("operate")
  raise "operations timeout" unless operation.fetch("timeout-minutes") == 30
  raise "operation control sha" unless operation.fetch("env").fetch("CONTROL_SHA") == "${{ github.sha }}"
  operation_steps = operation.fetch("steps")
  validate = operation_steps.find { |step| step.fetch("name") == "Validate operation configuration" }
  raise "operation current-main guard" unless validate.fetch("run").include?("git/ref/heads/main")
  cloudflare = operation_steps.find { |step| step.fetch("name") == "Sync Cloudflare origin secret" }.fetch("run")
  raise "cloudflare connect timeout" unless cloudflare.scan("--connect-timeout 10").length == 2
  raise "cloudflare total timeout" unless cloudflare.scan("--max-time 60").length == 2
  raise "job max retries" unless job.fetch("spec").fetch("template").fetch("spec").fetch("template").fetch("spec").fetch("maxRetries") == 1
' "${repo_root}/.github/workflows/ci.yml" "${workflow}" "${repo_root}/deploy/cloud-run/jobs/device-cleanup.yaml" "${operations_workflow}" "${repo_root}/.github/scripts/release/release.sh"

printf 'release checks: PASS\n'
