#!/usr/bin/env bash
# Resolve a selected commit (or a verified retry plan) to exact image digests.
set -euo pipefail
source "$(dirname "$0")/candidate.sh"

: "${CONTROL_SHA:?}" "${TARGET_ENVIRONMENT:?}" "${REGISTRY:?}" "${PROJECT_ID:?}" "${DEV_REGISTRY:?}" "${DEV_PROJECT_ID:?}" "${RUNNER_TEMP:?}"
case "$TARGET_ENVIRONMENT" in dev|prod) ;; *) release_error 'invalid environment'; exit 1 ;; esac
case "${RUN_MIGRATIONS:-false}" in true|false) ;; *) release_error 'invalid migration choice'; exit 1 ;; esac
is_sha "$CONTROL_SHA" || { release_error 'invalid control commit'; exit 1; }
plan_dir="${RUNNER_TEMP}/release-plan"
mkdir -p "$plan_dir"
tags_dir=$(mktemp -d "${RUNNER_TEMP}/candidate-tags.XXXXXX")
trap 'rm -rf "$tags_dir"' EXIT
list_tags "$REGISTRY" "$PROJECT_ID" > "$tags_dir/destination.json"

# Only consult dev when the destination cannot provide a target. A retained
# prod version remains usable after the dev registry has cleaned its images.
load_dev_tags() {
  if [ ! -f "$tags_dir/dev.json" ]; then
    if [ "$REGISTRY" = "$DEV_REGISTRY" ] && [ "$PROJECT_ID" = "$DEV_PROJECT_ID" ]; then
      cp "$tags_dir/destination.json" "$tags_dir/dev.json"
    else
      list_tags "$DEV_REGISTRY" "$DEV_PROJECT_ID" > "$tags_dir/dev.json"
    fi
  fi
}

version=''
if [ -n "${RETRY_BUNDLE_FILE:-}" ]; then
  [ -z "${RELEASE_REF:-}" ] || { release_error 'retry and release_ref are mutually exclusive'; exit 1; }
  validate_plan "$RETRY_BUNDLE_FILE"
  [ "$(jq -r .control_sha "$RETRY_BUNDLE_FILE")" = "${RETRY_CONTROL_SHA:?}" ] \
    || { release_error 'retry plan does not belong to the verified source run'; exit 1; }
  release_sha=$(jq -r .release_sha "$RETRY_BUNDLE_FILE")
  tag_commit=$(jq -r .tag_commit "$RETRY_BUNDLE_FILE")
  version=$(jq -r .version "$RETRY_BUNDLE_FILE")
  git merge-base --is-ancestor "$tag_commit" "$CONTROL_SHA"
  git merge-base --is-ancestor "$release_sha" "$tag_commit"
  if impact_changed "$release_sha" "$tag_commit"; then
    release_error 'retry tag commit is not compatible with its candidate'; exit 1
  fi
  images=$(jq -c .images "$RETRY_BUNDLE_FILE")
  sources='{}'
  for target in $(target_names); do
    expected=$(jq -r --arg target "$target" '.images[$target]' "$RETRY_BUNDLE_FILE")
    destination=$(candidate_image "$tags_dir/destination.json" "$REGISTRY" "$target" "$release_sha" "$version")
    [ -z "$destination" ] || [ "$destination" = "$expected" ] \
      || { release_error 'retry destination tags changed'; exit 1; }
    if [ -n "$destination" ]; then
      source=$destination
    else
      source=$(jq -r --arg target "$target" '.sources[$target]' "$RETRY_BUNDLE_FILE")
    fi
    # Verification pulls the exact saved digest, never a mutable tag.
    verify_image "$source" "$release_sha"
    sources=$(jq -c --arg target "$target" --arg source "$source" '. + {($target):$source}' <<<"$sources")
  done
else
  if [ -z "${RELEASE_REF:-}" ]; then
    [ "$TARGET_ENVIRONMENT" = dev ] || { release_error 'prod requires release_ref'; exit 1; }
    tag_commit=$CONTROL_SHA
    candidates=$(git rev-list --first-parent --max-count=50 "$CONTROL_SHA")
    exact=false
  elif is_version "$RELEASE_REF"; then
    version=$RELEASE_REF
    tag_commit=$(git rev-parse --verify "refs/tags/${version}^{commit}")
    version_pattern=${version//./[.]}
    git show "${tag_commit}:CHANGELOG.md" > "$tags_dir/changelog"
    grep -Eq "^## \\[${version_pattern#v}\\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$" "$tags_dir/changelog" \
      || { release_error 'version requires a matching dated changelog heading'; exit 1; }
    candidates=$(git rev-list --first-parent --max-count=50 "$tag_commit")
    exact=false
  else
    [[ "$RELEASE_REF" =~ ^[0-9a-f]{7,40}$ ]] \
      || { release_error 'release_ref must be a SHA or vX.Y.Z'; exit 1; }
    tag_commit=$(git rev-parse --verify "${RELEASE_REF}^{commit}")
    candidates=$tag_commit
    exact=true
  fi
  if [ "${REQUIRE_VERSION:-false}" = true ] && [ -z "$version" ]; then
    release_error 'version tagging requires vX.Y.Z'; exit 1
  fi
  git merge-base --is-ancestor "$tag_commit" "$CONTROL_SHA" \
    || { release_error 'selected commit is not a main ancestor'; exit 1; }
  release_sha=''
  # Search retained destination images first, including docs-only ancestors.
  # Only if there is no complete set do we need the dev repository at all.
  for location in destination combined; do
    while IFS= read -r candidate; do
      if [ "$exact" = false ] && impact_changed "$candidate" "$tag_commit"; then continue; fi
      images='{}' sources='{}' complete=true
      for target in $(target_names); do
        destination=$(candidate_image "$tags_dir/destination.json" "$REGISTRY" "$target" "$candidate")
        source=$destination
        if [ -z "$source" ] && [ "$location" = combined ]; then
          load_dev_tags
          source=$(candidate_image "$tags_dir/dev.json" "$DEV_REGISTRY" "$target" "$candidate")
        fi
        if [ -z "$source" ]; then complete=false; continue; fi
        # Invalid evidence must not turn into a fallback to an older candidate.
        verify_image "$source" "$candidate"
        if [ -n "$version" ]; then
          version_image=$(tag_digest "$tags_dir/destination.json" "$REGISTRY" "$target" "$version")
          if [ -n "$version_image" ] && [ "${version_image##*@}" != "${source##*@}" ]; then
            release_error 'version tag conflicts with candidate'; exit 1
          fi
        fi
        sources=$(jq -c --arg target "$target" --arg source "$source" '. + {($target):$source}' <<<"$sources")
        images=$(jq -c --arg target "$target" --arg image "$(image_base "$REGISTRY" "$target")@${source##*@}" '. + {($target):$image}' <<<"$images")
      done
      [ "$complete" = true ] || continue
      release_sha=$candidate
      break 2
    done <<<"$candidates"
  done
  [ -n "$release_sha" ] || { release_error 'no complete compatible candidate; publish or select another candidate'; exit 1; }
fi

# Nonsecret run artifact: enough to retry exact contents without a release DB.
jq -n --arg environment "$TARGET_ENVIRONMENT" --arg sha "$release_sha" \
  --arg control "$CONTROL_SHA" --arg tag_commit "$tag_commit" --arg version "$version" \
  --argjson migrations "${RUN_MIGRATIONS:-false}" \
  --argjson images "$images" --argjson sources "$sources" \
  '{format_version:1,environment:$environment,release_sha:$sha,tag_commit:$tag_commit,
    version:$version,control_sha:$control,run_migrations:$migrations,
    images:$images,sources:$sources}' > "$plan_dir/plan.json"
validate_plan "$plan_dir/plan.json"
{
  printf 'release_sha=%s\nbundle_file=%s\n' "$release_sha" "$plan_dir/plan.json"
  printf 'version=%s\n' "$version"
} >> "${GITHUB_OUTPUT:-/dev/stdout}"
