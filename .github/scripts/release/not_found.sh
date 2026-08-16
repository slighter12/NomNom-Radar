#!/usr/bin/env bash

# Only a real gcloud NOT_FOUND status may authorize a create/publish fallback.
not_found_status() {
  grep -Eq '(^|[[:space:]])NOT_FOUND([:[:space:]]|$)' "$1"
}
