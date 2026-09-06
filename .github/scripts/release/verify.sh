#!/usr/bin/env bash
# Check the deployed data, not just whether the deployment command succeeded.
set -euo pipefail
source "$(dirname "$0")/candidate.sh"
: "${BUNDLE_FILE:?}" "${PROJECT_ID:?}" "${REGION:?}" "${RADAR_URL:?}"
[[ "$RADAR_URL" == https://* ]] || exit 1
release_sha=$(jq -er .release_sha "$BUNDLE_FILE")
is_sha "$release_sha"

verify_once() {
  local target expected json revision revision_json traffic
  for target in $(target_names); do
    expected=$(jq -er --arg target "$target" '.images[$target]' "$BUNDLE_FILE") || return 1
    if [ "$target" = device-cleanup ]; then
      json=$(gcloud run jobs describe "$target" --project="$PROJECT_ID" --region="$REGION" --format=json) || return 1
      jq -e --arg sha "$release_sha" --arg image "$expected" '
        .metadata.labels["release-sha"] == $sha
        and .spec.template.spec.template.spec.containers[0].image == $image
        and all(.status.conditions[]?; .status != "False" and .status != "Unknown")
      ' <<<"$json" >/dev/null || return 1
    else
      json=$(gcloud run services describe "$target" --project="$PROJECT_ID" --region="$REGION" --format=json) || return 1
      jq -e --arg sha "$release_sha" '
        .metadata.labels["release-sha"] == $sha
        and any(.status.conditions[]?; .type == "Ready" and .status == "True")
        and ([.status.traffic[]? | select((.percent // 0) > 0) | .percent] | add) == 100
      ' <<<"$json" >/dev/null || return 1
      traffic=$(jq -er '[.status.traffic[] | select((.percent // 0) > 0) | .revisionName]
        | select(length > 0 and all(.[]; type == "string" and length > 0)) | unique[]' <<<"$json") || return 1
      while IFS= read -r revision; do
        revision_json=$(gcloud run revisions describe "$revision" --project="$PROJECT_ID" --region="$REGION" --format=json) || return 1
        jq -e --arg sha "$release_sha" --arg image "$expected" '
          .metadata.labels["release-sha"] == $sha and .spec.containers[0].image == $image
          and any(.status.conditions[]?; .type == "Ready" and .status == "True")
        ' <<<"$revision_json" >/dev/null || return 1
      done <<<"$traffic"
    fi
  done
}

attempt=1
until verify_once; do
  [ "$attempt" -lt "${VERIFY_RETRIES:-5}" ] \
    || { release_error 'deployed digests, labels, readiness, or traffic did not converge'; exit 1; }
  sleep "${VERIFY_RETRY_DELAY:-5}"
  attempt=$((attempt + 1))
done
curl --silent --show-error --fail --connect-timeout 10 --max-time 60 \
  --retry 5 --retry-all-errors "${RADAR_URL%/}/health" >/dev/null
