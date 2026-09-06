#!/usr/bin/env bash
# Shared candidate data operations. Workflow steps own build, copy, and deploy.

release_error() { printf 'release: %s\n' "$*" >&2; return 1; }
target_names() { jq -r '.[]' "$(dirname "${BASH_SOURCE[0]}")/targets.json"; }
image_base() { printf '%s/%s/%s' "$1" "$(printf '%s' "${OWNER:?}" | tr '[:upper:]' '[:lower:]')" "$2"; }
is_sha() { [[ "$1" =~ ^[0-9a-f]{40}$ ]]; }
is_version() { [[ "$1" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; }

# Both push detection and docs-only candidate selection use this one list.
impact_changed() {
  local status=0
  git diff --quiet "$1" "$2" -- Dockerfile .dockerignore Makefile go.mod go.sum \
    cmd config internal database/migration deploy/cloud-run .github/scripts/release \
    .github/actions/publish-images \
    .github/workflows/ci.yml .github/workflows/release-cloud-run.yml || status=$?
  case "$status" in
    0) return 1 ;;
    1) return 0 ;;
    *) printf 'release: git diff failed\n' >&2; exit "$status" ;;
  esac
}

list_tags() {
  local json
  # Query the existing repository, not a possibly absent image package.
  json=$(gcloud artifacts docker tags list "$1" --project="$2" --format=json) || return 1
  jq -e 'type == "array" and all(.[];
    (.tag | type == "string") and (.image | type == "string")
    and (.version | type == "string"))' <<<"$json" >/dev/null \
    || { release_error 'unexpected registry tag list'; return 1; }
  printf '%s\n' "$json"
}

tag_digest() {
  local file=$1 base matches digest
  base=$(image_base "$2" "$3")
  matches=$(jq -c --arg base "$base" --arg tag "$4" \
    '[.[] | select(.image == $base and (.tag | split("/")[-1]) == $tag)]' "$file") || return 1
  case "$(jq length <<<"$matches")" in
    0) return 0 ;;
    1) ;;
    *) release_error 'duplicate registry tag result'; return 1 ;;
  esac
  digest=$(jq -r '.[0].version | split("/")[-1]' <<<"$matches")
  [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] \
    || { release_error 'invalid registry digest'; return 1; }
  printf '%s@%s\n' "$base" "$digest"
}

candidate_image() {
  local file=$1 registry=$2 target=$3 sha=$4 version=${5:-} tag found='' image
  is_sha "$sha" || { release_error 'invalid candidate SHA'; return 1; }
  for tag in "${sha:0:7}" "$sha" "$version"; do
    [ -n "$tag" ] || continue
    image=$(tag_digest "$file" "$registry" "$target" "$tag") || return 1
    [ -n "$image" ] || continue
    if [ -n "$found" ] && [ "$found" != "$image" ]; then
      release_error "conflicting tags for ${target}"; return 1
    fi
    found=$image
  done
  printf '%s' "$found"
}

verify_image() {
  local image=$1 sha=$2 attempt=1
  is_sha "$sha" || { release_error 'invalid provenance SHA'; return 1; }
  while ! gh attestation verify "oci://${image}" \
    --repo "${GITHUB_REPOSITORY:?}" \
    --signer-workflow "${GITHUB_REPOSITORY}/.github/workflows/ci.yml" \
    --source-ref refs/heads/main --source-digest "$sha" \
    --deny-self-hosted-runners --predicate-type https://slsa.dev/provenance/v1 >/dev/null; do
    [ "$attempt" -lt "${ATTESTATION_RETRIES:-4}" ] \
      || { release_error 'candidate attestation did not verify'; return 1; }
    sleep "${ATTESTATION_RETRY_DELAY:-5}"
    attempt=$((attempt + 1))
  done
}

validate_plan() {
  local file=$1 target expected source destination
  jq -e --arg environment "${TARGET_ENVIRONMENT:?}" '
    .format_version == 1 and .environment == $environment
    and (.release_sha | type == "string" and test("^[0-9a-f]{40}$"))
    and (.tag_commit | type == "string" and test("^[0-9a-f]{40}$"))
    and (.control_sha | type == "string" and test("^[0-9a-f]{40}$"))
    and (.version | type == "string" and (. == "" or test("^v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$")))
    and (.run_migrations | type == "boolean")
    and (.images | keys) == ["device-cleanup","geoworker","radar"]
    and (.sources | keys) == ["device-cleanup","geoworker","radar"]
    and all((.images[], .sources[]); type == "string"
      and test("^[^@[:space:]]+@sha256:[0-9a-f]{64}$"))
  ' "$file" >/dev/null || { release_error 'invalid release plan'; return 1; }
  for target in $(target_names); do
    expected=$(image_base "${REGISTRY:?}" "$target")
    destination=$(jq -r --arg target "$target" '.images[$target]' "$file")
    source=$(jq -r --arg target "$target" '.sources[$target]' "$file")
    [ "${destination%@*}" = "$expected" ] \
      && [ "${destination##*@}" = "${source##*@}" ] \
      || { release_error 'release plan destination mismatch'; return 1; }
    if [ "${source%@*}" != "$expected" ] \
      && [ "${source%@*}" != "$(image_base "${DEV_REGISTRY:?}" "$target")" ]; then
      release_error 'release plan source is outside configured registries'; return 1
    fi
  done
}
