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
  .on.push.branches == ["main"] and .on.push.tags == null
  and .on.pull_request.branches == ["main"] and (.on | has("workflow_call"))
  and .jobs["version-signal"] == null
  and .jobs.checks.permissions == {contents:"read"}
  and .jobs["candidate-changes"].needs == "checks"
  and .jobs["publish-candidate"].needs == "candidate-changes"
  and .jobs["publish-candidate"].concurrency.queue == "max"
' "$json/ci.json" >/dev/null || { echo 'CI must share checks/build with version requests and serialize candidates' >&2; exit 1; }
jq -e '
  .concurrency == null
  and .jobs.request.concurrency == null
  and .jobs.release.concurrency == {group:"cloud-run-release",queue:"max","cancel-in-progress":false}
  and .on.workflow_run == null and .on.workflow_dispatch.inputs.version.required == true
  and .jobs.request.if == "github.ref == '\''refs/heads/main'\''"
  and .jobs.request.environment == "prod"
  and .jobs.request.permissions == {contents:"read",attestations:"read"}
  and .jobs.candidate.needs == "request"
  and .jobs.candidate.uses == "./.github/workflows/ci.yml"
  and .jobs.release.needs == ["request","candidate"]
  and .jobs.release.environment == "prod"
  and .jobs.release.permissions == {contents:"write",attestations:"read"}
  and ([.jobs.release.steps[].name] as $names
    | ($names | index("Prepare all prod images and SHA tags")) < ($names | index("Establish Git tag"))
    and ($names | index("Establish Git tag")) < ($names | index("Publish version image tags")))
  and all(.jobs[].steps[]?; ((.uses // "" | test("deploy-cloudrun|get-secretmanager-secrets")) | not)
    and ((.run // "" | test("goose|gcloud run|migration.*up")) | not))
  and all(.jobs[] | select(.steps != null); all(.steps[] | select(.uses // "" | startswith("actions/checkout@"));
    .with.ref == "${{ github.sha }}" and .with["persist-credentials"] == false))
' "$json/version-images.json" >/dev/null || { echo 'Version request/check/preparation/tagging order or trust boundary changed' >&2; exit 1; }
jq -e '
  .jobs.release.environment == "${{ inputs.environment }}"
  and all(.jobs.release.steps[] | select(.uses == "./.github/actions/publish-images");
    .if == "inputs.environment == '\''prod'\''" and .with.tags == "${{ steps.candidate.outputs.sha_tags }}")
' "$json/release-cloud-run.json" >/dev/null || { echo 'Only prod deployment may promote, without version publication' >&2; exit 1; }
for name in release-cloud-run cloud-run-operations; do
  jq -e '. as $workflow | all(.jobs[]; (.concurrency // $workflow.concurrency) == {group:"cloud-run-release",queue:"max","cancel-in-progress":false})' "$json/$name.json" >/dev/null \
    || { echo 'Shared serialization must queue requests without replacing pending work' >&2; exit 1; }
done
# actionlint currently lacks GitHub queue support. Validate precisely the key
# excluded from its legacy concurrency schema, including future workflow files.
for file in "$root/workflows/"*.yml; do
  yq -o=json '.' "$file" | jq -e '
    all((.concurrency, .jobs[].concurrency) | select(type == "object" and has("queue"));
      (.queue == "max" or .queue == "single") and (."cancel-in-progress" // false) == false)
  ' >/dev/null || { echo 'Invalid concurrency queue' >&2; exit 1; }
done
jq -e '
  [.jobs[].steps[]?] as $steps
  | any($steps[]; (.uses // "" | startswith("actions/attest@")) and .with["subject-digest"] == "${{ steps.build.outputs.digest }}")
    and any($steps[]; .id == "build" and (.uses // "" | startswith("docker/build-push-action@"))
      and (.with.outputs | type == "string" and contains("push-by-digest=true") and contains("name-canonical=true") and contains("push=true"))
    and all($steps[]; (.with.tags // "") | contains("staging-") | not))
' "$json/ci.json" >/dev/null || { echo 'Build must attest a digest-only push without staging tags' >&2; exit 1; }
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
