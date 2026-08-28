#!/usr/bin/env bash

# Repository-specific Cloud Run release operations. Configuration is supplied
# by the trusted workflow through environment variables; there is intentionally
# no generic option layer.
#
# Every step a release workflow performs lives here rather than inline in YAML,
# so that it is reachable from release_test.sh. A workflow step should be a
# call to one of the subcommands at the bottom of this file plus the GitHub
# Actions that cannot be expressed in bash (authentication, buildx, attest).
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
default_targets_file="${repo_root}/.github/scripts/release/targets.json"
# shellcheck source=.github/scripts/release/not_found.sh
source "${repo_root}/.github/scripts/release/not_found.sh"

die() { printf 'release: %s\n' "$*" >&2; exit 1; }
integrity_die() { printf 'release: %s\n' "$*" >&2; exit 3; }

# gcloud writes the text the not-found matchers need to stderr, so every lookup
# captures it to a scratch file. These three own that file's whole lifetime:
# take one from new_error_file, and end every unclassified failure in one of
# the two reporters so the diagnostics are surfaced and the file is dropped.
new_error_file() { mktemp "${RUNNER_TEMP:-/tmp}/release-gcloud-error.XXXXXX"; }
fail_with_error_file() {
  local file=$1
  shift
  cat "${file}" >&2
  rm -f "${file}"
  die "$*"
}
integrity_fail_with_error_file() {
  local file=$1
  shift
  cat "${file}" >&2
  rm -f "${file}"
  integrity_die "$*"
}
required() { for name in "$@"; do test -n "${!name:-}" || die "missing ${name}"; done; }
is_sha() { printf '%s' "$1" | grep -Eq '^[0-9a-f]{40}$'; }

# A pinned rollback is typed by hand, usually while something is broken, so an
# abbreviated SHA is accepted and expanded here. This can only widen what may
# be typed, never what may be deployed: git rejects both unknown and ambiguous
# prefixes, and the expanded SHA still has to pass the ancestry, completeness,
# and attestation checks. Image tags are always the full 40 characters, so
# every consumer downstream sees the expanded form.
resolve_requested_sha() {
  local resolved
  printf '%s' "${REQUESTED_SHA}" | grep -Eq '^[0-9a-f]{7,40}$' \
    || die 'REQUESTED_SHA must be 7 to 40 hexadecimal characters'
  resolved=$(git rev-parse --verify --quiet "${REQUESTED_SHA}^{commit}") \
    || die "REQUESTED_SHA ${REQUESTED_SHA} is not a unique commit in this repository"
  printf '%s\n' "${resolved}"
}
is_digest() { printf '%s' "$1" | grep -Eq '^[^@[:space:]]+@sha256:[0-9a-f]{64}$'; }
safe_line() { case "$2" in *$'\r'*|*$'\n'*) die "$1 must not contain CR or LF" ;; esac; }
owner_slug() { printf '%s' "${OWNER}" | tr '[:upper:]' '[:lower:]'; }
repo_path() {
  local configured=$1 default=$2
  if [ -z "${configured}" ]; then
    printf '%s\n' "${default}"
  else
    case "${configured}" in
      /*) printf '%s\n' "${configured}" ;;
      *) printf '%s/%s\n' "${repo_root}" "${configured#./}" ;;
    esac
  fi
}
targets_file() { repo_path "${TARGETS_FILE:-}" "${default_targets_file}"; }

# Validated once by the dispatcher, not on every catalog read.
validate_catalog() {
  test -f "$(targets_file)" || die 'missing target catalog'
  jq -e '
    type == "object" and length > 0
    and ([to_entries[].value.order] | length == (unique | length))
    and all(to_entries[];
      (.key | test("^[a-z0-9-]+$"))
      and (.value | type == "object")
      and (.value.order | type == "number" and floor == .)
      and (
        (.value.kind == "service" and (.value.overlay | type == "string" and length > 0))
        or
        (.value.kind == "job" and (.value.manifest | type == "string" and length > 0))
      )
    )
  ' "$(targets_file)" >/dev/null \
    || die 'target catalog has an invalid schema'
  while IFS= read -r target; do
    kind=$(target_kind "${target}")
    if [ "${kind}" = service ]; then
      overlay=$(target_overlay "${target}")
      for environment in dev prod; do
        test -f "${repo_root}/deploy/cloud-run/overlays/${environment}/${overlay}/kustomization.yaml" \
          || die "target ${target} is missing its ${environment} overlay"
      done
    else
      manifest=$(target_manifest "${target}")
      test -f "${repo_root}/deploy/cloud-run/${manifest}" \
        || die "target ${target} is missing its job manifest"
    fi
  done < <(jq -r 'keys[]' "$(targets_file)")
}
target_names() { jq -r 'to_entries | sort_by(.value.order) | .[].key' "$(targets_file)"; }
target_kind() { jq -er --arg target "$1" '.[$target].kind' "$(targets_file)"; }
target_overlay() { jq -er --arg target "$1" '.[$target].overlay' "$(targets_file)"; }
target_manifest() { jq -er --arg target "$1" '.[$target].manifest' "$(targets_file)"; }

# A change in any of these paths requires a new candidate image. This list is
# the single source consumed by both the resolver and CI change detection.
impact_path_args() {
  printf '%s\n' \
    'Dockerfile' \
    '.dockerignore' \
    'Makefile' \
    'go.mod' \
    'go.sum' \
    'cmd/**' \
    'config/**' \
    'internal/**' \
    'database/migration/**' \
    'deploy/cloud-run/**' \
    '.github/scripts/release/**' \
    '.github/workflows/ci.yml' \
    '.github/workflows/release-cloud-run.yml'
}

# 0 = at least one of the given paths differs, 1 = none do. Any other git
# failure is fatal: "git broke" must never be read as "nothing changed".
paths_changed() {
  local from=$1 to=$2 status
  shift 2
  test "$#" -gt 0 || die 'paths_changed requires at least one path'
  git diff --quiet "${from}..${to}" -- "$@" && return 1
  status=$?
  [ "${status}" -eq 1 ] || die "git diff failed while comparing ${from}..${to}"
  return 0
}

impact_changed() {
  local path_args=()
  while IFS= read -r path; do
    path_args+=("${path}")
  done < <(impact_path_args)
  test "${#path_args[@]}" -gt 0 || die 'impact path manifest is empty'
  paths_changed "$1" "$2" "${path_args[@]}"
}

# Whether a push to main has to publish new candidate images. A force push, or
# a before-SHA that is no longer reachable, cannot be diffed against at all, so
# publish a complete candidate instead of guessing that nothing changed.
candidate_changes() {
  required GITHUB_SHA
  local candidate=true
  if printf '%s' "${BEFORE_SHA:-}" | grep -Eq '^[0]{40}$' \
    || ! git cat-file -e "${BEFORE_SHA:-}^{commit}" 2>/dev/null; then
    printf '::notice::Force-push or missing before SHA: publishing a complete candidate.\n'
  elif ! impact_changed "${BEFORE_SHA}" "${GITHUB_SHA}"; then
    candidate=false
  fi
  {
    printf 'candidate=%s\n' "${candidate}"
    printf 'targets=%s\n' "$(jq -e -c 'keys | select(length > 0)' "$(targets_file)")"
  } >> "${GITHUB_OUTPUT:-/dev/stdout}"
}

# gcloud must answer with a fully qualified digest under exactly the expected
# repository. Anything else is an integrity failure, not a usable reference.
assert_digest_under() {
  local image expected_base=$2
  image=$(printf '%s' "$1" | tr -d '\r\n')
  { [ "${image%@*}" = "${expected_base}" ] && is_digest "${image}"; } \
    || integrity_die "${image} is not an exact digest under ${expected_base}"
  printf '%s\n' "${image}"
}

# Describe and validate in one call. Callers that already hold a describe
# result must use assert_digest_under instead of describing a second time: the
# resolver walks up to 50 commits across every target and a duplicate describe
# doubles that traffic.
resolved_digest() {
  local ref=$1 project=$2 expected_base=$3 image
  image=$(gcloud artifacts docker images describe "${ref}" \
    --project="${project}" --format='value(image_summary.fully_qualified_digest)')
  assert_digest_under "${image}" "${expected_base}"
}

verify_attestation() {
  image=$1 sha=$2
  attempt=1
  while ! gh attestation verify "oci://${image}" \
    --repo "${GITHUB_REPOSITORY}" \
    --signer-workflow "${GITHUB_REPOSITORY}/.github/workflows/ci.yml" \
    --source-ref refs/heads/main \
    --source-digest "${sha}" \
    --deny-self-hosted-runners \
    --predicate-type https://slsa.dev/provenance/v1; do
    if [ "${attempt}" -ge "${ATTESTATION_RETRIES:-4}" ]; then
      return 1
    fi
    sleep "${ATTESTATION_RETRY_DELAY:-5}"
    attempt=$((attempt + 1))
  done
}

# ---------------------------------------------------------------------------
# Error contract
# ---------------------------------------------------------------------------

error_contract_probe='error-contract-probe-do-not-create'

# not_found.sh recognises missing resources by exact gcloud error strings.
# Re-probe those strings against the live API before a release mutates
# anything, so wording drift fails here rather than being misread as a missing
# resource at a point where that authorizes a create or a publish.
check_error_contract() {
  required PROJECT_ID REGION REGISTRY OWNER
  local owner type ref
  owner=$(owner_slug)
  # Not local: the EXIT trap below runs after this function has returned.
  probe_error=$(new_error_file)
  trap 'rm -f "${probe_error:-}"' EXIT HUP INT TERM

  for type in service job revision; do
    if gcloud run "${type}s" describe "${error_contract_probe}" \
      --project="${PROJECT_ID}" --region="${REGION}" --format=json >/dev/null 2>"${probe_error}"; then
      die "error contract probe ${type} unexpectedly exists"
    fi
    cloud_run_not_found "${type}" "${error_contract_probe}" "${probe_error}" \
      || fail_with_error_file "${probe_error}" \
        "gcloud run ${type}s not-found wording changed; update not_found.sh before releasing"
  done

  ref="${REGISTRY}/${owner}/${error_contract_probe}:probe"
  if gcloud artifacts docker images describe "${ref}" \
    --project="${PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)' \
    >/dev/null 2>"${probe_error}"; then
    die 'error contract probe image unexpectedly exists'
  fi
  image_not_found "${probe_error}" \
    || fail_with_error_file "${probe_error}" \
      'Artifact Registry not-found wording changed; update not_found.sh before releasing'
  printf 'release: not-found error contract verified against live gcloud\n' >&2
}

# ---------------------------------------------------------------------------
# Candidate publication
# ---------------------------------------------------------------------------

# Build to a run-scoped staging tag first. The SHA tag is added only after the
# digest has been attested and that attestation verified, because CI cannot
# repair a SHA tag it published early: the release path refuses to re-attest an
# existing digest, and removing the tag needs a permission CI does not hold.
# See docs/adr/0001-attested-candidate-images.md.
stage_candidate() {
  required OWNER TARGET RELEASE_SHA REGISTRY PROJECT_ID
  is_sha "${RELEASE_SHA}" || die 'invalid RELEASE_SHA'
  jq -e --arg target "${TARGET}" '.[$target] != null' "$(targets_file)" >/dev/null \
    || die "unknown target ${TARGET}"
  local owner built final_ref expected_base describe_error existing nonce staging_ref staged
  owner=$(owner_slug)
  expected_base="${REGISTRY}/${owner}/${TARGET}"
  final_ref="${expected_base}:${RELEASE_SHA}"
  built=$(git show -s --format=%cI "${RELEASE_SHA}")

  describe_error=$(new_error_file)
  if existing=$(gcloud artifacts docker images describe "${final_ref}" \
    --project="${PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)' \
    2>"${describe_error}"); then
    rm -f "${describe_error}"
    existing=$(assert_digest_under "${existing}" "${expected_base}")
    {
      printf 'owner=%s\n' "${owner}"
      printf 'digest=%s\n' "${existing##*@}"
      printf 'staging_ref=\n'
      printf 'needs_finalize=false\n'
    } >> "${GITHUB_OUTPUT:-/dev/stdout}"
    return 0
  fi
  image_not_found "${describe_error}" \
    || fail_with_error_file "${describe_error}" "unable to inspect candidate image for ${TARGET}"
  rm -f "${describe_error}"

  nonce=$(openssl rand -hex 16)
  staging_ref="${expected_base}:candidate-stage-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}-${nonce}-${TARGET}"
  docker buildx build \
    --file "${repo_root}/Dockerfile" \
    --target "${TARGET}" \
    --platform linux/amd64 \
    --pull \
    --provenance=false \
    --sbom=false \
    --build-arg "VERSION=${RELEASE_SHA}" \
    --build-arg "BUILT=${built}" \
    --build-arg "GIT_COMMIT=${RELEASE_SHA}" \
    --build-arg "IMAGE_NAME=${owner}" \
    --cache-from "type=gha,scope=${TARGET}" \
    --cache-to "type=gha,mode=max,scope=${TARGET}" \
    --tag "${staging_ref}" \
    --push \
    "${repo_root}"
  staged=$(resolved_digest "${staging_ref}" "${PROJECT_ID}" "${expected_base}")
  {
    printf 'owner=%s\n' "${owner}"
    printf 'digest=%s\n' "${staged##*@}"
    printf 'staging_ref=%s\n' "${staging_ref}"
    printf 'needs_finalize=true\n'
  } >> "${GITHUB_OUTPUT:-/dev/stdout}"
}

# Add the SHA tag to the already attested digest and prove the tag resolves to
# exactly that digest.
finalize_candidate() {
  required OWNER TARGET RELEASE_SHA REGISTRY PROJECT_ID STAGED_DIGEST NEEDS_FINALIZE
  local owner expected_base final_ref final_digest
  owner=$(owner_slug)
  expected_base="${REGISTRY}/${owner}/${TARGET}"
  final_ref="${expected_base}:${RELEASE_SHA}"
  if [ "${NEEDS_FINALIZE}" = true ]; then
    required STAGING_REF
    gcloud artifacts docker tags add "${STAGING_REF%:*}@${STAGED_DIGEST}" "${final_ref}" \
      --project="${PROJECT_ID}"
  fi
  final_digest=$(resolved_digest "${final_ref}" "${PROJECT_ID}" "${expected_base}")
  [ "${final_digest##*@}" = "${STAGED_DIGEST}" ] \
    || integrity_die "final candidate tag for ${TARGET} does not equal the attested digest"
  # The staging tag is deliberately left in place. It aliases a digest that now
  # carries its SHA tag, so it occupies no extra storage, is never a release
  # input, and expires with its image version under the repository cleanup
  # policy. Deleting it would need artifactregistry.tags.delete, which the
  # candidate identity does not hold and should not be given: that widens a
  # public repository's long-lived key from "can publish" to "can unpublish".
  {
    echo '## Candidate image'
    echo
    echo "Release SHA: \`${RELEASE_SHA}\`"
    echo
    echo '| Target | Digest |'
    echo '| --- | --- |'
    echo "| ${TARGET} | \`${final_digest##*@}\` |"
  } >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
}

verify_candidate_attestation() {
  required IMAGE RELEASE_SHA GITHUB_REPOSITORY
  verify_attestation "${IMAGE}" "${RELEASE_SHA}" \
    || die "attestation did not verify for ${IMAGE}"
}

# ---------------------------------------------------------------------------
# Candidate resolution
# ---------------------------------------------------------------------------

resolve_candidate_image() {
  target=$1 sha=$2 owner=$3
  expected_base="${DEV_REGISTRY}/${owner}/${target}"
  error_file=$(new_error_file)
  if image=$(gcloud artifacts docker images describe "${expected_base}:${sha}" \
    --project="${DEV_PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)' \
    2>"${error_file}"); then
    rm -f "${error_file}"
  else
    if image_not_found "${error_file}"; then
      rm -f "${error_file}"
      return 1
    fi
    integrity_fail_with_error_file "${error_file}" "unable to inspect candidate image ${target}"
  fi
  assert_digest_under "${image}" "${expected_base}"
}

resolve_candidate() {
  required CONTROL_SHA DEV_PROJECT_ID DEV_REGISTRY OWNER GITHUB_REPOSITORY
  is_sha "${CONTROL_SHA}" || die 'invalid CONTROL_SHA'
  git cat-file -e "${CONTROL_SHA}^{commit}" || die 'CONTROL_SHA is not available locally'
  owner=$(owner_slug)

  if [ -n "${REQUESTED_SHA:-}" ]; then
    requested=$(resolve_requested_sha)
    git merge-base --is-ancestor "${requested}" "${CONTROL_SHA}" \
      || die 'REQUESTED_SHA is not an ancestor of the current main control commit'
    candidates="${requested}"
  else
    candidates=$(git rev-list --first-parent --max-count=50 "${CONTROL_SHA}") \
      || die "git rev-list failed while scanning candidates"
    test -n "${candidates}" || die 'git rev-list returned no candidates'
  fi
  while IFS= read -r candidate <&3; do
    # A candidate can only be promoted when everything after it is outside the
    # release contract. This is what permits docs-only commits without rebuilds.
    if [ -z "${REQUESTED_SHA:-}" ] && impact_changed "${candidate}" "${CONTROL_SHA}"; then
      continue
    fi
    images='{}'
    complete=true
    while IFS= read -r target; do
      image=$(resolve_candidate_image "${target}" "${candidate}" "${owner}") || {
        image_status=$?
        [ "${image_status}" -eq 3 ] && exit 3
        complete=false
        break
      }
      images=$(jq -c --arg target "${target}" --arg image "${image}" '. + {($target):$image}' <<<"${images}")
    done < <(target_names)
    [ "${complete}" = true ] || continue

    while IFS= read -r target; do
      image=$(jq -r --arg target "${target}" '.[$target]' <<<"${images}")
      if ! verify_attestation "${image}" "${candidate}"; then
        die "candidate ${candidate} has no valid attestation for ${target} (${image})"
      fi
    done < <(target_names)

    bundle="${RUNNER_TEMP:-/tmp}/bundle.json"
    jq -cn --arg sha "${candidate}" --argjson images "${images}" \
      '{release_sha:$sha,images:$images}' > "${bundle}"
    printf 'release_sha=%s\npath=%s\n' "${candidate}" "${bundle}" >> "${GITHUB_OUTPUT:-/dev/stdout}"
    return 0
  done 3<<<"${candidates}"
  if [ -n "${REQUESTED_SHA:-}" ]; then
    die "pinned candidate ${REQUESTED_SHA} has no complete attested image set"
  fi
  die 'no complete candidate ancestor is compatible with the current main control commit'
}

validate_bundle() {
  required BUNDLE_FILE RELEASE_SHA
  expected_targets=$(jq -c 'keys | sort' "$(targets_file)")
  jq -e --arg sha "${RELEASE_SHA}" --argjson expected_targets "${expected_targets}" '
    type == "object" and .release_sha == $sha
    and (.images | keys | sort) == $expected_targets
    and all(.images[];
      type == "string"
      and test("^[^@[:space:]]+@sha256:[0-9a-f]{64}$")
      and ((split("@")[0] | split("/")[-1] | contains(":")) | not)
    )
  ' "${BUNDLE_FILE}" >/dev/null || die 'invalid release bundle'
}

# ---------------------------------------------------------------------------
# Cloud Run state
# ---------------------------------------------------------------------------

# Each field below is read from the one JSON path gcloud actually populates,
# verified against live dev resources on 2026-08-23. A missing image means the
# response shape changed and must be treated as an error, never as an empty
# string that would compare unequal and silently look like drift.
describe_resource() {
  type=$1 name=$2
  error_file=$(new_error_file)
  if [ "${type}" = service ]; then
    if ! json=$(gcloud run services describe "${name}" --project="${PROJECT_ID}" --region="${REGION}" --format=json 2>"${error_file}"); then
      if cloud_run_not_found "${type}" "${name}" "${error_file}"; then
        rm -f "${error_file}"
        jq -cn '{exists:false,label:"",desired_label:"",ready:false,image:""}'
        return
      fi
      fail_with_error_file "${error_file}" "cannot describe ${name}"
    fi
    rm -f "${error_file}"
    desired=$(jq -r '.metadata.labels["release-sha"] // ""' <<<"${json}")
    revision=$(jq -r '.status.latestReadyRevisionName // ""' <<<"${json}")
    ready=$(jq -r '([.status.conditions[]? | select(.type == "Ready") | .status] | first) // "False"' <<<"${json}")
    label='' image=''
    if [ -n "${revision}" ]; then
      revision_error=$(new_error_file)
      if revision_json=$(gcloud run revisions describe "${revision}" \
        --project="${PROJECT_ID}" --region="${REGION}" --format=json 2>"${revision_error}"); then
        rm -f "${revision_error}"
        label=$(jq -r '.metadata.labels["release-sha"] // ""' <<<"${revision_json}")
        image=$(jq -er '.spec.containers[0].image' <<<"${revision_json}") \
          || die "revision ${revision} response has no container image"
      elif cloud_run_not_found revision "${revision}" "${revision_error}"; then
        rm -f "${revision_error}"
        ready=False
      else
        fail_with_error_file "${revision_error}" "cannot describe revision ${revision}"
      fi
    fi
    [ "${ready}" = True ] && ready=true || ready=false
    jq -cn --arg label "${label}" --arg desired "${desired}" --arg image "${image}" --argjson ready "${ready}" \
      '{exists:true,label:$label,desired_label:$desired,ready:$ready,image:$image}'
  else
    if ! json=$(gcloud run jobs describe "${name}" --project="${PROJECT_ID}" --region="${REGION}" --format=json 2>"${error_file}"); then
      if cloud_run_not_found "${type}" "${name}" "${error_file}"; then
        rm -f "${error_file}"
        jq -cn '{exists:false,label:"",desired_label:"",ready:true,image:""}'
        return
      fi
      fail_with_error_file "${error_file}" "cannot describe ${name}"
    fi
    rm -f "${error_file}"
    # jq -e only inspects the last output value, so wrapping this in an object
    # would report success with a null image. Extract it on its own.
    label=$(jq -r '.metadata.labels["release-sha"] // ""' <<<"${json}")
    image=$(jq -er '.spec.template.spec.template.spec.containers[0].image' <<<"${json}") \
      || die "job ${name} response has no container image"
    jq -cn --arg label "${label}" --arg image "${image}" \
      '{exists:true,label:$label,desired_label:"",ready:true,image:$image}'
  fi
}

snapshot() {
  if [ -n "${RELEASE_STATE_FILE:-}" ]; then
    test "${ALLOW_RELEASE_FIXTURES:-false}" = true \
      || die 'RELEASE_STATE_FILE is a test-only hook'
    jq -c . "${RELEASE_STATE_FILE}"
    return
  fi
  required PROJECT_ID REGION
  state='{}'
  while IFS= read -r target; do
    kind=$(target_kind "${target}") \
      || die "cannot resolve kind for ${target}"
    resource=$(describe_resource "${kind}" "${target}") \
      || die "cannot snapshot ${target}"
    state=$(jq -c --arg target "${target}" --argjson resource "${resource}" '. + {($target):$resource}' <<<"${state}")
  done < <(target_names)
  printf '%s\n' "${state}"
}

baseline_digest() {
  target=$1 label=$2
  if [ -n "${RELEASE_BASELINE_DIGESTS_FILE:-}" ]; then
    test "${ALLOW_RELEASE_FIXTURES:-false}" = true \
      || die 'RELEASE_BASELINE_DIGESTS_FILE is a test-only hook'
    jq -er --arg label "${label}" --arg target "${target}" '.[$label][$target]' \
      "${RELEASE_BASELINE_DIGESTS_FILE}" || die "missing baseline digest fixture for ${target}"
    return
  fi
  required REGISTRY OWNER PROJECT_ID
  owner=$(owner_slug)
  local error_file digest
  error_file=$(new_error_file)
  if digest=$(gcloud artifacts docker images describe "${REGISTRY}/${owner}/${target}:${label}" \
    --project="${PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)' \
    2>"${error_file}"); then
    rm -f "${error_file}"
    printf '%s\n' "${digest}"
    return 0
  fi
  if image_not_found "${error_file}"; then
    rm -f "${error_file}"
    return 0
  fi
  fail_with_error_file "${error_file}" "cannot resolve baseline tag ${target}:${label}"
}

# A pinned release may move backwards, so its baseline only has to be
# comparable; an unpinned release must move strictly forward.
baseline_is_compatible() {
  candidate_baseline=$1
  if [ -n "${REQUESTED_SHA:-}" ]; then
    git merge-base --is-ancestor "${candidate_baseline}" "${CONTROL_SHA}" || return 1
    git merge-base --is-ancestor "${candidate_baseline}" "${RELEASE_SHA}" \
      || git merge-base --is-ancestor "${RELEASE_SHA}" "${candidate_baseline}"
  else
    git merge-base --is-ancestor "${candidate_baseline}" "${RELEASE_SHA}"
  fi
}

preflight() {
  required RELEASE_SHA
  is_sha "${RELEASE_SHA}" || die 'invalid RELEASE_SHA'
  if [ -n "${REQUESTED_SHA:-}" ]; then
    required CONTROL_SHA
    # Compare against the expanded form: the resolver already turned an
    # abbreviated pin into the full SHA that RELEASE_SHA carries.
    [ "$(resolve_requested_sha)" = "${RELEASE_SHA}" ] \
      || die 'pinned release SHA does not match REQUESTED_SHA'
  fi
  validate_bundle
  state=$(snapshot) || die 'cannot snapshot Cloud Run state'
  expected_targets=$(jq -c 'keys | sort' "$(targets_file)")
  jq -e --argjson expected_targets "${expected_targets}" \
    '(keys | sort) == $expected_targets and all(.[]; has("exists") and has("label"))' \
    <<<"${state}" >/dev/null || die 'invalid Cloud Run state'

  # Desired service labels must never disagree with their ready revisions.
  jq -e --arg release_sha "${RELEASE_SHA}" \
    'all(.[]; (.desired_label // "") == "" or .desired_label == .label or .desired_label == $release_sha)' \
    <<<"${state}" >/dev/null \
    || die 'Cloud Run desired/running label drift'
  while IFS= read -r label; do
    [ -z "${label}" ] || is_sha "${label}" || die 'malformed release-sha label'
  done < <(jq -r '.[].label // ""' <<<"${state}")
  while IFS= read -r target; do
    exists=$(jq -r --arg target "${target}" '.[$target].exists' <<<"${state}")
    label=$(jq -r --arg target "${target}" '.[$target].label // ""' <<<"${state}")
    image=$(jq -r --arg target "${target}" '.[$target].image // ""' <<<"${state}")
    [ "${exists}" = true ] || continue
    if [ "${label}" = "${RELEASE_SHA}" ]; then
      required REGISTRY OWNER
      expected=$(jq -r --arg target "${target}" '.images[$target]' "${BUNDLE_FILE}")
      is_digest "${image}" || die "target-labelled ${target} image is not immutable"
      owner=$(owner_slug)
      [ "${image%@*}" = "${REGISTRY}/${owner}/${target}" ] \
        || die "target-labelled ${target} image is outside the current environment registry"
      [ "${image##*@}" = "${expected##*@}" ] \
        || die "target-labelled ${target} does not run the candidate digest"
    elif [ -n "${label}" ]; then
      is_digest "${image}" || die "baseline ${target} image is not immutable"
      image_base=${image%@*}
      [ "${image_base##*/}" = "${target}" ] || die "baseline ${target} image repository does not match its resource"
      tagged=$(baseline_digest "${target}" "${label}")
      if [ -z "${tagged}" ]; then
        printf '::warning::Baseline tag for target %s with label %s is no longer present in the registry (most likely reclaimed by the repository cleanup policy); skipping the tag-to-running-digest check.\n' \
          "${target}" "${label}"
      else
        [ "${tagged}" = "${image}" ] || die "baseline ${target} label does not resolve to its running digest"
      fi
    fi
  done < <(target_names)

  missing=$(jq '[.[] | select(.exists == false)] | length' <<<"${state}")
  blank=$(jq '[.[] | select(.exists == true and (.label // "") == "")] | length' <<<"${state}")
  labels=$(jq -c '[.[] | select(.exists == true and (.label // "") != "") | .label] | unique' <<<"${state}")
  count=$(jq length <<<"${labels}")
  baseline=

  # An absent target, or one deployed before release-sha labelling existed,
  # leaves no fleet-wide baseline to diff against. Keep the baseline empty so
  # every migration phase runs; goose skips versions it already applied.
  if [ "${missing}" -gt 0 ] || [ "${blank}" -gt 0 ]; then
    :
  elif [ "${count}" -eq 1 ]; then
    baseline=$(jq -r '.[0]' <<<"${labels}")
    if [ "${baseline}" != "${RELEASE_SHA}" ]; then
      baseline_is_compatible "${baseline}" \
        || die "release is not compatible with the current target baseline ${baseline}"
    fi
  elif [ "${count}" -eq 2 ] && jq -e --arg release_sha "${RELEASE_SHA}" 'index($release_sha) != null' <<<"${labels}" >/dev/null; then
    baseline=$(jq -r --arg release_sha "${RELEASE_SHA}" '.[] | select(. != $release_sha)' <<<"${labels}")
    baseline_is_compatible "${baseline}" \
      || die "release is not compatible with the partial target baseline ${baseline}"
  else
    die 'Cloud Run resources have divergent release state'
  fi

  # Keep the machine-readable result on the last stdout line; workflow warnings
  # may precede it.
  result=$(jq -cn --arg baseline_sha "${baseline}" '{baseline_sha:$baseline_sha}')
  printf '%s\n' "${result}"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf 'baseline_sha=%s\n' "${baseline}" >> "${GITHUB_OUTPUT}"
  fi
}

# ---------------------------------------------------------------------------
# Migrations
# ---------------------------------------------------------------------------

# Which goose phases the release must run, derived from what changed between
# the fleet baseline and the selected candidate.
migration_phases() {
  required RELEASE_SHA
  local pre=false shared=false post=false any=false

  if [ -n "${REQUESTED_SHA:-}" ]; then
    # A pinned release moves code without moving schema, and the schema is
    # forward-only. If the pin does not sit on the same side of every
    # migration change as the running baseline, deploying it would run old
    # code against a schema it has never seen. Stop and make that explicit
    # rather than reporting it in a notice nobody reads.
    if [ -n "${BASELINE_SHA:-}" ] && [ "${BASELINE_SHA}" != "${RELEASE_SHA}" ] \
      && paths_changed "${RELEASE_SHA}" "${BASELINE_SHA}" 'database/migration/**'; then
      test "${ACKNOWLEDGE_SCHEMA_DRIFT:-false}" = true || die \
        "pinned release ${RELEASE_SHA} crosses a migration change against baseline ${BASELINE_SHA}; handle the schema first, then re-dispatch with acknowledge_schema_drift"
      printf '::warning::Pinned release crosses a migration change; proceeding on explicit acknowledgement.\n'
    fi
    printf 'pre=false\nshared=false\npost=false\nany=false\n' >> "${GITHUB_OUTPUT:-/dev/stdout}"
    printf '::notice::Pinned release: migrations skipped. Handle schema separately.\n'
    return 0
  fi

  if [ -z "${BASELINE_SHA:-}" ]; then
    pre=true shared=true post=true
  elif [ "${BASELINE_SHA}" != "${RELEASE_SHA}" ]; then
    paths_changed "${BASELINE_SHA}" "${RELEASE_SHA}" 'database/migration/supabase/pre' && pre=true
    paths_changed "${BASELINE_SHA}" "${RELEASE_SHA}" 'database/migration/postgres' && shared=true
    paths_changed "${BASELINE_SHA}" "${RELEASE_SHA}" 'database/migration/supabase/post' && post=true
  fi
  { [ "${pre}" = false ] && [ "${shared}" = false ] && [ "${post}" = false ]; } || any=true
  printf 'pre=%s\nshared=%s\npost=%s\nany=%s\n' \
    "${pre}" "${shared}" "${post}" "${any}" >> "${GITHUB_OUTPUT:-/dev/stdout}"
}

# ---------------------------------------------------------------------------
# Promotion
# ---------------------------------------------------------------------------

# Copy the exact dev digests into the prod registry without rebuilding. An
# existing prod tag is accepted only when it already points at the same digest.
promote() {
  required SOURCE_BUNDLE RELEASE_SHA OWNER REGISTRY PROJECT_ID GCRANE
  test -x "${GCRANE}" || die 'pinned gcrane is not executable'
  local owner promoted target source destination expected_base describe_error existing image bundle
  owner=$(owner_slug)
  promoted='{}'
  while IFS= read -r target; do
    # Preflight has already validated the complete target set and pinned digests.
    source=$(jq -er --arg target "${target}" '.images[$target]' "${SOURCE_BUNDLE}") \
      || die "source bundle has no image for ${target}"
    expected_base="${REGISTRY}/${owner}/${target}"
    destination="${expected_base}:${RELEASE_SHA}"
    describe_error=$(new_error_file)
    if existing=$(gcloud artifacts docker images describe "${destination}" \
      --project="${PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)' 2>"${describe_error}"); then
      rm -f "${describe_error}"
      image=$(assert_digest_under "${existing}" "${expected_base}")
      [ "${image##*@}" = "${source##*@}" ] \
        || integrity_die "immutable prod tag already points to a different digest: ${target}"
    else
      # Only a genuine missing image may authorize a copy. Any other failure,
      # a permission error in particular, must not be read as "not there yet".
      image_not_found "${describe_error}" \
        || fail_with_error_file "${describe_error}" "unable to inspect prod image: ${target}"
      rm -f "${describe_error}"
      "${GCRANE}" copy "${source}" "${destination}"
      image=$(resolved_digest "${destination}" "${PROJECT_ID}" "${expected_base}")
      [ "${image##*@}" = "${source##*@}" ] \
        || integrity_die "prod digest verification failed: ${target}"
    fi
    promoted=$(jq -c --arg target "${target}" --arg image "${image}" '. + {($target):$image}' <<<"${promoted}")
  done < <(target_names)
  bundle="${RUNNER_TEMP:-/tmp}/promoted-bundle.json"
  jq -cn --arg sha "${RELEASE_SHA}" --argjson images "${promoted}" \
    '{release_sha:$sha,images:$images}' > "${bundle}"
  printf 'path=%s\n' "${bundle}" >> "${GITHUB_OUTPUT:-/dev/stdout}"
}

# ---------------------------------------------------------------------------
# Deployment
# ---------------------------------------------------------------------------

replace_placeholder() {
  local placeholder=$1 value=$2
  manifest=${manifest//"${placeholder}"/"${value}"}
}

render_service() {
  local target=$1 image=$2 output=$3 overlay manifest secret_json
  overlay=$(target_overlay "${target}")
  manifest=$(kubectl kustomize "${repo_root}/deploy/cloud-run/overlays/${TARGET_ENVIRONMENT}/${overlay}")
  secret_json=
  if [ "${TARGET_ENVIRONMENT}" = prod ] && [ "${target}" = radar ]; then
    required CLOUDFLARE_ORIGIN_SECRET
    safe_line CLOUDFLARE_ORIGIN_SECRET "${CLOUDFLARE_ORIGIN_SECRET}"
    secret_json=$(CLOUDFLARE_ORIGIN_SECRET="${CLOUDFLARE_ORIGIN_SECRET}" jq -Rn '$ENV.CLOUDFLARE_ORIGIN_SECRET')
  fi
  replace_placeholder IMAGE_PLACEHOLDER "${image}"
  replace_placeholder SERVICE_ACCOUNT_PLACEHOLDER "${RUNTIME_SA_EMAIL}"
  replace_placeholder PROJECT_ID_PLACEHOLDER "${PROJECT_ID}"
  replace_placeholder HTTP_ALLOWEDHOST_PLACEHOLDER "${ALLOWED_HOST}"
  replace_placeholder HTTP_CORSALLOWEDORIGINS_PLACEHOLDER "${CORS_ALLOWED_ORIGINS:-}"
  replace_placeholder HTTP_RATELIMIT_ENABLED_PLACEHOLDER "${RATE_LIMIT_ENABLED:-true}"
  replace_placeholder HTTP_RATELIMIT_RATE_PLACEHOLDER "${RATE_LIMIT_RATE:-10}"
  replace_placeholder HTTP_RATELIMIT_BURST_PLACEHOLDER "${RATE_LIMIT_BURST:-30}"
  replace_placeholder HTTP_RATELIMIT_EXPIRESIN_PLACEHOLDER "${RATE_LIMIT_EXPIRES_IN:-3m}"
  replace_placeholder GOOGLEOAUTH_CLIENTID_PLACEHOLDER "${GOOGLE_OAUTH_CLIENT_ID}"
  replace_placeholder RELEASE_SHA_PLACEHOLDER "${RELEASE_SHA}"
  replace_placeholder HTTP_CLOUDFLARESECRET_PLACEHOLDER "${secret_json}"
  grep -Eq '(_PLACEHOLDER|PLACEHOLDER_)' <<<"${manifest}" && die "unresolved ${target} manifest placeholder"
  printf '%s\n' "${manifest}" > "${output}"
}

render_job() {
  local target=$1 image=$2 output=$3 manifest_path manifest project_number
  manifest_path=$(target_manifest "${target}")
  manifest=$(cat "${repo_root}/deploy/cloud-run/${manifest_path}")
  required PROJECT_ID PROJECT_NUMBER RUNTIME_SA_EMAIL
  project_number="${PROJECT_NUMBER}"
  safe_line PROJECT_NUMBER "${project_number}"
  replace_placeholder IMAGE_PLACEHOLDER "${image}"
  replace_placeholder SERVICE_ACCOUNT_PLACEHOLDER "${RUNTIME_SA_EMAIL}"
  replace_placeholder RELEASE_SHA_PLACEHOLDER "${RELEASE_SHA}"
  replace_placeholder PROJECT_NUMBER_PLACEHOLDER "${project_number}"
  grep -Eq '(_PLACEHOLDER|PLACEHOLDER_)' <<<"${manifest}" && die "unresolved ${target} manifest placeholder"
  printf '%s\n' "${manifest}" > "${output}"
}

deploy() {
  required RELEASE_SHA TARGET_ENVIRONMENT PROJECT_ID REGION RUNTIME_SA_EMAIL ALLOWED_HOST GOOGLE_OAUTH_CLIENT_ID
  case "${TARGET_ENVIRONMENT}" in dev|prod) ;; *) die 'TARGET_ENVIRONMENT must be dev or prod' ;; esac
  for pair in RUNTIME_SA_EMAIL:"${RUNTIME_SA_EMAIL}" ALLOWED_HOST:"${ALLOWED_HOST}" GOOGLE_OAUTH_CLIENT_ID:"${GOOGLE_OAUTH_CLIENT_ID}"; do
    safe_line "${pair%%:*}" "${pair#*:}"
  done
  safe_line CORS_ALLOWED_ORIGINS "${CORS_ALLOWED_ORIGINS:-}"
  safe_line RATE_LIMIT_ENABLED "${RATE_LIMIT_ENABLED:-true}"
  safe_line RATE_LIMIT_RATE "${RATE_LIMIT_RATE:-10}"
  safe_line RATE_LIMIT_BURST "${RATE_LIMIT_BURST:-30}"
  safe_line RATE_LIMIT_EXPIRES_IN "${RATE_LIMIT_EXPIRES_IN:-3m}"
  if [ "${TARGET_ENVIRONMENT}" = prod ]; then
    required CLOUDFLARE_ORIGIN_SECRET
    safe_line CLOUDFLARE_ORIGIN_SECRET "${CLOUDFLARE_ORIGIN_SECRET}"
  fi
  validate_bundle
  if jq -e 'any(.[]; .kind == "job")' "$(targets_file)" >/dev/null; then
    required PROJECT_NUMBER
    safe_line PROJECT_NUMBER "${PROJECT_NUMBER}"
  fi
  umask 077
  temp_dir=$(mktemp -d "${RUNNER_TEMP:-/tmp}/release-manifests.XXXXXX")
  trap 'rm -rf "${temp_dir}"' EXIT HUP INT TERM
  while IFS= read -r target; do
    image=$(jq -r --arg target "${target}" '.images[$target]' "${BUNDLE_FILE}")
    is_digest "${image}" || die "bundle contains a mutable image for ${target}"
    kind=$(target_kind "${target}")
    manifest_file="${temp_dir}/${target}.yaml"
    if [ "${kind}" = service ]; then
      render_service "${target}" "${image}" "${manifest_file}"
      gcloud run services replace "${manifest_file}" --project="${PROJECT_ID}" --region="${REGION}" --quiet
    else
      render_job "${target}" "${image}" "${manifest_file}"
      gcloud run jobs replace "${manifest_file}" --project="${PROJECT_ID}" --region="${REGION}" --quiet
    fi
  done < <(target_names)
}

verify_once() {
  state=$(snapshot)
  while IFS= read -r target; do
    label=$(jq -r --arg target "${target}" '.[$target].label // ""' <<<"${state}")
    actual=$(jq -r --arg target "${target}" '.[$target].image // ""' <<<"${state}")
    expected=$(jq -r --arg target "${target}" '.images[$target]' "${BUNDLE_FILE}")
    [ "${label}" = "${RELEASE_SHA}" ] && [ "${actual}" = "${expected}" ] || return 1
  done < <(target_names)
  jq -e 'all(.[]; .ready == true)' <<<"${state}" >/dev/null
}

verify() {
  required RELEASE_SHA PROJECT_ID REGION
  validate_bundle
  attempt=0
  delay="${VERIFY_RETRY_DELAY:-5}"
  max_delay=30
  until verify_once; do
    attempt=$((attempt + 1))
    [ "${attempt}" -ge "${VERIFY_RETRIES:-10}" ] && die 'released labels, digests, or readiness did not converge'
    sleep "${delay}"
    delay=$((delay * 2))
    if [ "${delay}" -gt "${max_delay}" ]; then
      delay="${max_delay}"
    fi
  done
  if [ "${SKIP_HEALTH:-false}" != true ]; then
    radar_url=$(gcloud run services describe radar --project="${PROJECT_ID}" --region="${REGION}" --format='value(status.url)')
    test -n "${radar_url}" || die 'radar has no service URL'
    curl --silent --show-error --fail \
      --connect-timeout 10 --max-time 60 \
      --retry 5 --retry-all-errors "${radar_url}/health" >/dev/null
  fi
}

# === dispatch ===
# release_test.sh sources everything above this marker to exercise individual
# functions. Keep the marker line exactly as written.
command=${1:-}
# The catalog is the input to almost every subcommand, so validate it once here
# rather than on each of the dozen catalog reads a single release performs.
case "${command}" in
  impact-paths|check-error-contract) ;;
  *) validate_catalog ;;
esac
case "${command}" in
  impact-paths) impact_path_args ;;
  check-error-contract) check_error_contract ;;
  candidate-changes) candidate_changes ;;
  stage-candidate) stage_candidate ;;
  verify-attestation) verify_candidate_attestation ;;
  finalize-candidate) finalize_candidate ;;
  resolve-candidate) resolve_candidate ;;
  preflight) preflight ;;
  migration-phases) migration_phases ;;
  promote) promote ;;
  deploy) deploy ;;
  verify) verify ;;
  *) die 'usage: release.sh impact-paths|check-error-contract|candidate-changes|stage-candidate|verify-attestation|finalize-candidate|resolve-candidate|preflight|migration-phases|promote|deploy|verify' ;;
esac
