#!/usr/bin/env bash
# Check release safety contracts, rather than the workflow's incidental layout.
set -euo pipefail
root=$(cd "$(dirname "$0")/../../.." && pwd)
json=$(mktemp -d)
trap 'rm -rf "$json"' EXIT
for name in ci release-cloud-run version-images cloud-run-operations; do
  yq -o=json '.' "$root/workflows/$name.yml" > "$json/$name.json"
done
jq -e '
  [.jobs[].steps[]?] as $steps
  | any($steps[]; (.uses // "" | startswith("actions/attest@")) and .with["subject-digest"] == "${{ steps.build.outputs.digest }}")
    and any($steps[]; .id == "build" and (.uses // "" | startswith("docker/build-push-action@")))
' "$json/ci.json" >/dev/null || { echo 'Build attestation must consume the build output digest' >&2; exit 1; }
jq -e '
  .jobs.release.steps as $steps
  | [$steps[] | select((.uses // "" | test("^(actions/setup-go|google-github-actions/get-secretmanager-secrets)@")) or (.run // "" | test("go install|secrets versions access|goose.*up"; "s")))] as $migration
  | ($migration | length > 0)
    and all($migration[]; .if == "inputs.run_migrations")
    and any($migration[]; .uses // "" | startswith("google-github-actions/get-secretmanager-secrets@"))
    and ([$steps[] | select(.uses // "" | startswith("google-github-actions/deploy-cloudrun@")) | .id] == ["deploy_geoworker", "deploy_device_cleanup", "deploy_radar"])
    and all($steps[]; (.run // "" | test("gcloud run jobs execute")) | not)
    and all($steps[] | select(.uses // "" | startswith("google-github-actions/deploy-cloudrun@")); (.with.wait != true and .with.wait != "true"))
' "$json/release-cloud-run.json" >/dev/null || { echo 'Migration guards or deployment safety contract changed' >&2; exit 1; }
jq -e --arg prefix "inputs.operation == 'configure-device-cleanup-scheduler' && steps.scheduler.outputs.exists == " '
  .jobs.operate.steps as $steps
  | any($steps[]; (.run // "" | contains("gcloud scheduler jobs create")) and .if == ($prefix + "\u0027false\u0027"))
    and any($steps[]; (.run // "" | contains("gcloud scheduler jobs update")) and .if == ($prefix + "\u0027true\u0027"))
' "$json/cloud-run-operations.json" >/dev/null || { echo 'Scheduler mutation steps must use the existence output' >&2; exit 1; }
for file in "$json/"*.json; do
  jq -e 'all(.jobs[].steps[]? | select(.uses != null); .uses | startswith("./") or test("@[0-9a-f]{40}$"))' "$file" >/dev/null \
    || { echo 'External actions must be pinned to a commit' >&2; exit 1; }
done
printf 'workflow semantic checks passed\n'
