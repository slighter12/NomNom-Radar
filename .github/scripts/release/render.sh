#!/usr/bin/env bash
# Prepare selected checkout manifests; yq performs structured rendering next.
set -euo pipefail
source "$(dirname "$0")/candidate.sh"
: "${BUNDLE_FILE:?}" "${SOURCE_DIR:?}" "${MANIFEST_DIR:?}"
validate_plan "$BUNDLE_FILE"
[ -d "$SOURCE_DIR/deploy/cloud-run" ] && [ -d "$SOURCE_DIR/database/migration" ]
[ -z "$(find "$SOURCE_DIR/deploy/cloud-run" "$SOURCE_DIR/database/migration" -type l -print -quit)" ] \
  || { release_error 'candidate data must not contain symlinks'; exit 1; }
umask 077
mkdir -p "$MANIFEST_DIR"
for target in $(target_names); do
  if [ "$target" = device-cleanup ]; then
    cp "$SOURCE_DIR/deploy/cloud-run/jobs/device-cleanup.yaml" "$MANIFEST_DIR/$target.yaml"
  else
    kubectl kustomize "$SOURCE_DIR/deploy/cloud-run/overlays/$TARGET_ENVIRONMENT/$target" > "$MANIFEST_DIR/$target.yaml" 2>/dev/null \
      || { release_error 'invalid or incompatible deployment template'; exit 1; }
  fi
done
