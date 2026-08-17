#!/usr/bin/env bash

# Repository-specific Cloud Run release operations. Configuration is supplied
# by the trusted workflow through environment variables; there is intentionally
# no generic option layer.
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
default_targets_file="${repo_root}/.github/scripts/release/targets.json"
default_impact_paths_file="${repo_root}/.github/scripts/release/impact-paths.txt"
# shellcheck source=.github/scripts/release/not_found.sh
source "${repo_root}/.github/scripts/release/not_found.sh"

die() { printf 'release: %s\n' "$*" >&2; exit 1; }
integrity_die() { printf 'release: %s\n' "$*" >&2; exit 3; }
required() { for name in "$@"; do test -n "${!name:-}" || die "missing ${name}"; done; }
is_sha() { printf '%s' "$1" | grep -Eq '^[0-9a-f]{40}$'; }
is_digest() { printf '%s' "$1" | grep -Eq '^[^@[:space:]]+@sha256:[0-9a-f]{64}$'; }
safe_line() { case "$2" in *$'\r'*|*$'\n'*) die "$1 must not contain CR or LF" ;; esac; }
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
impact_paths_file() { repo_path "${IMPACT_PATHS_FILE:-}" "${default_impact_paths_file}"; }
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
      case "${overlay}" in
        ''|/*|*..*) die "target ${target} has an unsafe service overlay path" ;;
      esac
      for environment in dev prod; do
        test -f "${repo_root}/deploy/cloud-run/overlays/${environment}/${overlay}/kustomization.yaml" \
          || die "target ${target} is missing its ${environment} overlay"
      done
    else
      manifest=$(target_manifest "${target}")
      case "${manifest}" in
        ''|/*|*..*) die "target ${target} has an unsafe job manifest path" ;;
      esac
      test -f "${repo_root}/deploy/cloud-run/${manifest}" \
        || die "target ${target} is missing its job manifest"
    fi
  done < <(jq -r 'keys[]' "$(targets_file)")
}
target_names() {
  validate_catalog
  jq -r 'to_entries | sort_by(.value.order) | .[].key' "$(targets_file)"
}
target_kind() { jq -er --arg target "$1" '.[$target].kind' "$(targets_file)"; }
target_overlay() { jq -er --arg target "$1" '.[$target].overlay' "$(targets_file)"; }
target_manifest() { jq -er --arg target "$1" '.[$target].manifest' "$(targets_file)"; }

impact_path_args() {
  test -f "$(impact_paths_file)" || die "missing impact path manifest"
  for required_path in \
    'Dockerfile' \
    '.dockerignore' \
    'Makefile' \
    'go.mod' \
    'go.sum' \
    'cmd/**' \
    'config/**' \
    'internal/**' \
    '.github/scripts/release/**' \
    '.github/workflows/ci.yml' \
    '.github/workflows/release-cloud-run.yml' \
    'deploy/cloud-run/**' \
    'database/migration/**'; do
    grep -Fx "${required_path}" "$(impact_paths_file)" >/dev/null \
      || die "impact path manifest is missing ${required_path}"
  done
  sed -e '/^[[:space:]]*#/d' -e '/^[[:space:]]*$/d' "$(impact_paths_file)"
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

resolve_candidate_image() {
  target=$1 sha=$2 owner=$3
  tag="${DEV_REGISTRY}/${owner}/${target}:${sha}"
  error_file=$(mktemp "${RUNNER_TEMP:-/tmp}/candidate-describe.XXXXXX")
  if image=$(gcloud artifacts docker images describe "${tag}" \
    --project="${DEV_PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)' 2>"${error_file}"); then
    rm -f "${error_file}"
  else
    if image_not_found "${error_file}"; then
      rm -f "${error_file}"
      return 1
    fi
    cat "${error_file}" >&2
    rm -f "${error_file}"
    integrity_die "unable to inspect candidate image ${target}"
  fi
  image=$(printf '%s' "${image}" | tr -d '\r\n')
  expected_base="${DEV_REGISTRY}/${owner}/${target}"
  if ! { [ "${image%@*}" = "${expected_base}" ] && is_digest "${image}"; }; then
    integrity_die "candidate ${target} at ${sha} did not resolve to an exact dev digest"
  fi
  printf '%s\n' "${image}"
}

resolve_candidate() {
  required CONTROL_SHA DEV_PROJECT_ID DEV_REGISTRY OWNER GITHUB_REPOSITORY
  is_sha "${CONTROL_SHA}" || die 'invalid CONTROL_SHA'
  validate_catalog
  test -f "$(impact_paths_file)" || die 'missing impact path manifest'
  git cat-file -e "${CONTROL_SHA}^{commit}" || die 'CONTROL_SHA is not available locally'
  owner=$(printf '%s' "${OWNER}" | tr '[:upper:]' '[:lower:]')
  path_args=()
  while IFS= read -r path; do
    path_args+=("${path}")
  done < <(impact_path_args)
  test "${#path_args[@]}" -gt 0 || die 'impact path manifest is empty'

  if [ -n "${REQUESTED_SHA:-}" ]; then
    is_sha "${REQUESTED_SHA}" || die 'invalid REQUESTED_SHA'
    git cat-file -e "${REQUESTED_SHA}^{commit}" || die 'REQUESTED_SHA is not available locally'
    git merge-base --is-ancestor "${REQUESTED_SHA}" "${CONTROL_SHA}" \
      || die 'REQUESTED_SHA is not an ancestor of the current main control commit'
    candidates="${REQUESTED_SHA}"
  else
    candidates=$(git rev-list --first-parent --max-count=50 "${CONTROL_SHA}") \
      || die "git rev-list failed while scanning candidates"
    test -n "${candidates}" || die 'git rev-list returned no candidates'
  fi
  while IFS= read -r candidate <&3; do
    # A candidate can only be promoted when everything after it is outside the
    # release contract. This is what permits docs-only commits without rebuilds.
    if [ -z "${REQUESTED_SHA:-}" ]; then
      if git diff --quiet "${candidate}..${CONTROL_SHA}" -- "${path_args[@]}"; then
        :
      else
        diff_status=$?
        [ "${diff_status}" -eq 1 ] || die "git diff failed while scanning candidate ${candidate}"
        continue
      fi
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

not_found() {
  not_found_status "$3" && return 0
  # Cloud Run renders not-found without a status token. Explicitly reject
  # other gcloud status codes before accepting its resource-specific wording.
  other_gcloud_status "$3" && return 1
  grep -Fqi "Cannot find $1 [$2]" "$3"
}

describe_resource() {
  type=$1 name=$2
  error_file=$(mktemp "${RUNNER_TEMP:-/tmp}/release-error.XXXXXX")
  if [ "${type}" = service ]; then
    if ! json=$(gcloud run services describe "${name}" --project="${PROJECT_ID}" --region="${REGION}" --format=json 2>"${error_file}"); then
      if not_found "${type}" "${name}" "${error_file}"; then
        rm -f "${error_file}"
        jq -cn '{exists:false,label:"",desired_label:"",ready:false,image:""}'
        return
      fi
      cat "${error_file}" >&2; rm -f "${error_file}"; die "cannot describe ${name}"
    fi
    rm -f "${error_file}"
    desired=$(jq -r '.metadata.labels["release-sha"] // ""' <<<"${json}")
    revision=$(jq -r '.status.latestReadyRevisionName // ""' <<<"${json}")
    ready=$(jq -r '([.status.conditions[]? | select(.type == "Ready") | .status] | first) // "False"' <<<"${json}")
    label='' image=''
    if [ -n "${revision}" ]; then
      revision_error=$(mktemp "${RUNNER_TEMP:-/tmp}/release-revision.XXXXXX")
      if revision_json=$(gcloud run revisions describe "${revision}" \
        --project="${PROJECT_ID}" --region="${REGION}" --format=json 2>"${revision_error}"); then
        rm -f "${revision_error}"
        label=$(jq -r '.metadata.labels["release-sha"] // .spec.template.metadata.labels["release-sha"] // ""' <<<"${revision_json}")
        image=$(jq -r '.spec.containers[0].image // .spec.template.spec.containers[0].image // ""' <<<"${revision_json}")
      elif not_found revision "${revision}" "${revision_error}"; then
        rm -f "${revision_error}"
        ready=False
      else
        cat "${revision_error}" >&2
        rm -f "${revision_error}"
        die "cannot describe revision ${revision}"
      fi
    fi
    [ "${ready}" = True ] && ready=true || ready=false
    jq -cn --arg label "${label}" --arg desired "${desired}" --arg image "${image}" --argjson ready "${ready}" \
      '{exists:true,label:$label,desired_label:$desired,ready:$ready,image:$image}'
  else
    if ! json=$(gcloud run jobs describe "${name}" --project="${PROJECT_ID}" --region="${REGION}" --format=json 2>"${error_file}"); then
      if not_found "${type}" "${name}" "${error_file}"; then
        rm -f "${error_file}"
        jq -cn '{exists:false,label:"",desired_label:"",ready:true,image:""}'
        return
      fi
      cat "${error_file}" >&2; rm -f "${error_file}"; die "cannot describe ${name}"
    fi
    rm -f "${error_file}"
    jq -c '{exists:true,label:(.metadata.labels["release-sha"] // ""),desired_label:"",ready:true,image:(.spec.template.spec.template.spec.containers[0].image // .spec.template.template.spec.containers[0].image // .spec.template.spec.containers[0].image // "")}' <<<"${json}"
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
  owner=$(printf '%s' "${OWNER}" | tr '[:upper:]' '[:lower:]')
  gcloud artifacts docker images describe "${REGISTRY}/${owner}/${target}:${label}" \
    --project="${PROJECT_ID}" --format='value(image_summary.fully_qualified_digest)'
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
    is_sha "${CONTROL_SHA}" || die 'invalid CONTROL_SHA'
    [ "${REQUESTED_SHA}" = "${RELEASE_SHA}" ] || die 'pinned release SHA does not match REQUESTED_SHA'
    git cat-file -e "${CONTROL_SHA}^{commit}" || die 'CONTROL_SHA is not available locally'
    git merge-base --is-ancestor "${RELEASE_SHA}" "${CONTROL_SHA}" \
      || die 'pinned RELEASE_SHA is not an ancestor of the current main control commit'
  fi
  validate_bundle
  state=$(snapshot) || die 'cannot snapshot Cloud Run state'
  expected_targets=$(jq -c 'keys | sort' "$(targets_file)")
  jq -e --argjson expected_targets "${expected_targets}" \
    '(keys | sort) == $expected_targets and all(.[]; has("exists") and has("label"))' \
    <<<"${state}" >/dev/null || die 'invalid Cloud Run state'

  # Desired service labels must never disagree with their ready revisions.
  jq -e --arg target "${RELEASE_SHA}" \
    'all(.[]; (.desired_label // "") == "" or .desired_label == .label or .desired_label == $target)' \
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
      owner=$(printf '%s' "${OWNER}" | tr '[:upper:]' '[:lower:]')
      [ "${image%@*}" = "${REGISTRY}/${owner}/${target}" ] \
        || die "target-labelled ${target} image is outside the current environment registry"
      [ "${image##*@}" = "${expected##*@}" ] \
        || die "target-labelled ${target} does not run the candidate digest"
    elif [ -n "${label}" ]; then
      is_digest "${image}" || die "baseline ${target} image is not immutable"
      image_base=${image%@*}
      [ "${image_base##*/}" = "${target}" ] || die "baseline ${target} image repository does not match its resource"
      tagged=$(baseline_digest "${target}" "${label}")
      [ "${tagged}" = "${image}" ] || die "baseline ${target} label does not resolve to its running digest"
    fi
  done < <(target_names)

  missing=$(jq '[.[] | select(.exists == false)] | length' <<<"${state}")
  blank=$(jq '[.[] | select(.exists == true and (.label // "") == "")] | length' <<<"${state}")
  labels=$(jq -c '[.[] | select(.exists == true and (.label // "") != "") | .label] | unique' <<<"${state}")
  count=$(jq length <<<"${labels}")
  target_count=$(jq length <<<"${expected_targets}")
  baseline=

  if [ "${missing}" -eq "${target_count}" ]; then
    :
  elif [ "${missing}" -gt 0 ] || [ "${blank}" -gt 0 ]; then
    [ "${count}" -eq 1 ] && [ "$(jq -r '.[0]' <<<"${labels}")" = "${RELEASE_SHA}" ] \
      || die 'missing or unlabeled resources are accepted only for a target-SHA retry'
  elif [ "${count}" -eq 1 ]; then
    baseline=$(jq -r '.[0]' <<<"${labels}")
    if [ "${baseline}" != "${RELEASE_SHA}" ]; then
      baseline_is_compatible "${baseline}" \
        || die "release is not compatible with the current target baseline ${baseline}"
    fi
  elif [ "${count}" -eq 2 ] && jq -e --arg target "${RELEASE_SHA}" 'index($target) != null' <<<"${labels}" >/dev/null; then
    baseline=$(jq -r --arg target "${RELEASE_SHA}" '.[] | select(. != $target)' <<<"${labels}")
    baseline_is_compatible "${baseline}" \
      || die "release is not compatible with the partial target baseline ${baseline}"
  else
    die 'Cloud Run resources have divergent release state'
  fi

  result=$(jq -cn --arg baseline_sha "${baseline}" '{baseline_sha:$baseline_sha}')
  printf '%s\n' "${result}"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf 'baseline_sha=%s\n' "${baseline}" >> "${GITHUB_OUTPUT}"
  fi
}

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
  if [ "${TARGET_ENVIRONMENT}" = prod ]; then
    required CLOUDFLARE_ORIGIN_SECRET
    safe_line CLOUDFLARE_ORIGIN_SECRET "${CLOUDFLARE_ORIGIN_SECRET}"
  fi
  validate_catalog
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
    if [ "${kind}" = service ]; then
      manifest_file="${temp_dir}/${target}.yaml"
      render_service "${target}" "${image}" "${manifest_file}"
      gcloud run services replace "${manifest_file}" --project="${PROJECT_ID}" --region="${REGION}" --quiet
    else
      manifest_file="${temp_dir}/${target}.yaml"
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

case "${1:-}" in
  resolve-candidate) resolve_candidate ;;
  preflight) preflight ;;
  deploy) deploy ;;
  verify) verify ;;
  *) die 'usage: release.sh resolve-candidate|preflight|deploy|verify' ;;
esac
