#!/usr/bin/env bash

# Only a real gcloud NOT_FOUND status may authorize a create/publish fallback.
not_found_status() {
  grep -Eq '(^|[[:space:]])NOT_FOUND([:[:space:]]|$)' "$1"
}

# Any gcloud status code other than NOT_FOUND is a real error, never a missing
# resource. Keep this list explicit: an unlisted code degrades to the
# surface-specific wording check, not to a blanket accept.
other_gcloud_status() {
  grep -Eq '(^|[[:space:]])(PERMISSION_DENIED|UNAUTHENTICATED|INVALID_ARGUMENT|FAILED_PRECONDITION|RESOURCE_EXHAUSTED|UNAVAILABLE|INTERNAL|ABORTED|UNKNOWN)([:[:space:]]|$)' "$1"
}

# Artifact Registry reports a missing image with no status token. Observed
# 2026-08-17 (ci run 32002042274):
#   ERROR: (gcloud.artifacts.docker.images.describe) Image not found.
image_not_found() {
  not_found_status "$1" && return 0
  other_gcloud_status "$1" && return 1
  grep -Fqi 'Image not found.' "$1"
}
