#!/bin/sh
# Runs both in the pinned yq action and in focused local tests.
set -eu
: "${BUNDLE_FILE:?}" "${MANIFEST_DIR:?}" "${PROJECT_ID:?}" "${PROJECT_NUMBER:?}" \
  "${RUNTIME_SA_EMAIL:?}" "${ALLOWED_HOST:?}" "${GOOGLE_OAUTH_CLIENT_ID:?}" "${TARGET_ENVIRONMENT:?}"
fail() { echo 'release: invalid or incompatible deployment template or configuration' >&2; exit 1; }
case "$PROJECT_NUMBER" in *[!0-9]*|'') fail ;; esac
if [ "$TARGET_ENVIRONMENT" = prod ]; then : "${CLOUDFLARE_ORIGIN_SECRET:?}"; fi
export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-}"
export RATE_LIMIT_ENABLED="${RATE_LIMIT_ENABLED:-true}" RATE_LIMIT_RATE="${RATE_LIMIT_RATE:-10}"
export RATE_LIMIT_BURST="${RATE_LIMIT_BURST:-30}" RATE_LIMIT_EXPIRES_IN="${RATE_LIMIT_EXPIRES_IN:-3m}"
export CLOUDFLARE_ORIGIN_SECRET="${CLOUDFLARE_ORIGIN_SECRET:-}"
RELEASE_SHA=$(yq -r '.release_sha' "$BUNDLE_FILE")
export RELEASE_SHA
rules="$(dirname "$0")/render.yq"
umask 077
for TARGET in radar geoworker device-cleanup; do
  export TARGET
  IMAGE=$(yq -r '.images[strenv(TARGET)]' "$BUNDLE_FILE")
  export IMAGE
  file="$MANIFEST_DIR/$TARGET.yaml"
  # Parser diagnostics can include input values. Keep rendered data out of logs.
  yq --from-file "$rules" "$file" > "$file.tmp" 2>/dev/null || fail
  yq -e '
    .metadata.name == strenv(TARGET) and
    .metadata.labels.release-sha == strenv(RELEASE_SHA) and
    ([.. | select(tag == "!!str") | select(test("(_PLACEHOLDER|PLACEHOLDER_)"))] | length == 0) and
    ([.. | select(tag == "!!map" and has("containers")) | .containers] | length == 1) and
    ([.. | select(tag == "!!map" and has("containers")) | .containers |
      (length == 1 and .[0].image == strenv(IMAGE) and
       ([.[0].env[]? | select(has("value")) | .value | select(tag != "!!str")] | length == 0))] | .[0]) and
    ((strenv(TARGET) == "device-cleanup" and .kind == "Job") or
     (strenv(TARGET) != "device-cleanup" and .kind == "Service" and
      .spec.template.metadata.labels.release-sha == strenv(RELEASE_SHA)))
  ' "$file.tmp" >/dev/null 2>&1 || fail
  # The action runs as root; preserve the runner-owned file and its 0600 mode.
  cat "$file.tmp" > "$file"
  rm -f "$file.tmp"
done
