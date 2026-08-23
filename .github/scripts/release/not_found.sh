#!/usr/bin/env bash

# Recognising "this resource does not exist" is the one place where a wrong
# answer is silently destructive: a false positive lets a create/publish
# fallback run against a resource that is actually there but unreadable.
#
# Every matcher here therefore accepts only two things: a real gcloud
# NOT_FOUND status token, or one complete, exactly observed error line. Whole
# line fixed-string matching is what removes the need for a status denylist --
# a PERMISSION_DENIED or UNAVAILABLE line simply is not equal to the expected
# line, so it fails closed on its own.
#
# Every literal below was captured from gcloud on 2026-08-23 against project
# radar-dev-491902 in asia-east1. The release workflow re-probes them on every
# run (release.sh check-error-contract) so wording drift is caught before any
# mutation instead of at the moment a resource is misjudged.

# Surfaces that return a structured status: Artifact Registry when the
# repository itself is absent, and Cloud Scheduler.
#   ERROR: (gcloud.artifacts.docker.images.describe) NOT_FOUND: Requested entity was not found. ...
#   ERROR: (gcloud.scheduler.jobs.describe) NOT_FOUND: Job not found. ...
not_found_status() {
  grep -Eq '(^|[[:space:]])NOT_FOUND([:[:space:]]|$)' "$1"
}

# Artifact Registry reports a missing tag or image inside an existing
# repository with no status token at all:
#   ERROR: (gcloud.artifacts.docker.images.describe) Image not found.
# Further explanatory lines follow it; only this line is trusted.
image_not_found() {
  not_found_status "$1" && return 0
  grep -Fxq 'ERROR: (gcloud.artifacts.docker.images.describe) Image not found.' "$1"
}

# Cloud Run also reports missing resources without a status token. The three
# surfaces are not consistently punctuated -- jobs end in a period, services
# and revisions do not -- so both renderings are accepted:
#   ERROR: (gcloud.run.services.describe) Cannot find service [NAME]
#   ERROR: (gcloud.run.jobs.describe) Cannot find job [NAME].
#   ERROR: (gcloud.run.revisions.describe) Cannot find revision [NAME]
cloud_run_not_found() {
  local type=$1 name=$2 file=$3 line
  not_found_status "${file}" && return 0
  line="ERROR: (gcloud.run.${type}s.describe) Cannot find ${type} [${name}]"
  grep -Fxq -e "${line}" -e "${line}." "${file}"
}
