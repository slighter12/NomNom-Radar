# Sourced by release_test.sh: execute actual inline steps with local service fakes.
workflow="$scripts/../../workflows/release-cloud-run.yml"
yq -r '.jobs.release.steps[] | select(.id == "retry") | .run' "$workflow" > "$tmp/retry-step.sh"
yq -r '.runs.steps[] | select(.id == "images") | .run' "$scripts/../../actions/publish-images/action.yml" > "$tmp/images-step.sh"
cat > "$tmp/bin/gh" <<'FAKE'
#!/usr/bin/env bash
set -eu
if [ "$1" = api ]; then cat "$FAKE_DIR/run-metadata.json"; exit; fi
[ "$1 $2" = 'attestation verify' ]
case "$*" in *"--source-digest $EXPECTED_SHA"*) ;; *) exit 1 ;; esac
FAKE
export RETRY_RUN_ID=123 RELEASE_REF=''
jq -n --arg sha "$candidate" --arg repo "$GITHUB_REPOSITORY" '{repository:{full_name:$repo},path:".github/workflows/release-cloud-run.yml",event:"workflow_dispatch",head_branch:"main",status:"completed",conclusion:"failure",head_sha:$sha}' > "$tmp/good-run.json"
cp "$tmp/good-run.json" "$tmp/run-metadata.json"
bash "$tmp/retry-step.sh"
grep -q "control_sha=$candidate" "$GITHUB_OUTPUT" || fail 'retry control output'
for expression in '.repository.full_name="evil/repo"' '.path="other.yml"' '.event="push"' '.head_branch="other"' '.conclusion="success"'; do
  jq "$expression" "$tmp/good-run.json" > "$tmp/run-metadata.json"
  reject bash "$tmp/retry-step.sh"
done
# An existing commit outside main ancestry must also fail.
tree=$(git rev-parse HEAD^{tree})
orphan=$(printf 'unrelated\n' | git commit-tree "$tree")
jq --arg sha "$orphan" '.head_sha=$sha' "$tmp/good-run.json" > "$tmp/run-metadata.json"
reject bash "$tmp/retry-step.sh"
ok 'inline retry step validates API metadata and source commit ancestry'

mkdir -p .github/scripts/release "$RUNNER_TEMP/gcrane"
cp "$scripts/candidate.sh" "$scripts/targets.json" .github/scripts/release/
cat > "$tmp/bin/gcloud" <<'FAKE'
#!/usr/bin/env bash
set -eu
[ "$1 $2 $3" = 'artifacts docker tags' ]
case "$4" in
 list) [ "${QUERY_FAIL:-false}" != true ] || exit 1; cat "$FAKE_DIR/tag-state.json" ;;
 add)
   printf 'tag %s %s\n' "$5" "$6" >> "$FAKE_DIR/mutations"
   base=${6%:*}; tag=${6##*:}; digest=${5##*@}
   jq --arg base "$base" --arg tag "$tag" --arg digest "$digest" \
     'map(select(.image != $base or .tag != $tag)) + [{image:$base,tag:$tag,version:$digest}]' \
     "$FAKE_DIR/tag-state.json" > "$FAKE_DIR/tag-next.json"
   mv "$FAKE_DIR/tag-next.json" "$FAKE_DIR/tag-state.json" ;;
 *) exit 1 ;;
esac
FAKE
cat > "$RUNNER_TEMP/gcrane/gcrane" <<'FAKE'
#!/usr/bin/env bash
set -eu
[ "$1" = copy ]
[ "${2##*@}" = "${3##*@}" ]
printf 'copy %s %s\n' "$2" "$3" >> "$FAKE_DIR/mutations"
FAKE
cat > "$tmp/bin/docker" <<'FAKE'
#!/usr/bin/env bash
printf 'unexpected rebuild\n' >> "$FAKE_DIR/rebuild"
exit 1
FAKE
chmod +x "$RUNNER_TEMP/gcrane/gcrane" "$tmp/bin/docker"
jq '.version="v0.3.0" | .sources |= with_entries(if .key == "device-cleanup" then . else .value |= sub("registry/prod";"registry/dev") end)' "$tmp/retry.json" > "$tmp/promotion-plan.json"
export BUNDLE_FILE="$tmp/promotion-plan.json"
jq -n --arg sha "$candidate" --arg digest "$digest" '[{image:"registry/prod/example/device-cleanup",tag:($sha[0:7]),version:$digest}]' > "$tmp/tag-state.json"
: > "$tmp/mutations"
bash "$tmp/images-step.sh"
jq -e 'length == 6' "$tmp/tag-state.json" >/dev/null || fail 'all short/version tags'
for target in $(target_names); do
  [ "$(candidate_image "$tmp/tag-state.json" "$REGISTRY" "$target" "$candidate" v0.3.0)" = "$REGISTRY/example/$target@$digest" ] || fail 'published digest'
done
cp "$tmp/tag-state.json" "$tmp/complete-tags.json"
bash "$tmp/images-step.sh"
[ "$(jq -Sc 'sort_by(.image,.tag)' "$tmp/tag-state.json")" = "$(jq -Sc 'sort_by(.image,.tag)' "$tmp/complete-tags.json")" ] || fail idempotence
[ ! -f "$tmp/rebuild" ] || fail rebuild
ok 'inline promotion completes partial tags and repeats without changing digests or rebuilding'
for conflict_target in device-cleanup radar; do
for tag in "${candidate:0:7}" v0.3.0; do
  jq --arg image "$REGISTRY/example/$conflict_target" --arg tag "$tag" --arg digest "$other" 'map(if .image == $image and .tag == $tag then .version=$digest else . end)' "$tmp/complete-tags.json" > "$tmp/tag-state.json"
  cp "$tmp/tag-state.json" "$tmp/conflict-state.json"
  : > "$tmp/mutations"
  reject bash "$tmp/images-step.sh"
  cmp "$tmp/tag-state.json" "$tmp/conflict-state.json" || fail 'conflict overwritten'
  [ ! -s "$tmp/mutations" ] || fail 'mutated before rejecting target conflict'
done
done
: > "$tmp/mutations"
reject env QUERY_FAIL=true bash "$tmp/images-step.sh"
[ ! -s "$tmp/mutations" ] || fail 'query failure mutated tags'
ok 'inline promotion fails closed on tag conflicts and list failures'

yq -r '.runs.steps[] | select(.id == "copy") | .run' "$scripts/../../actions/publish-images/action.yml" > "$tmp/copy-step.sh"
: > "$GITHUB_OUTPUT"
env BUNDLE_FILE="$tmp/retry.json" bash "$tmp/copy-step.sh"
grep -qx 'required=false' "$GITHUB_OUTPUT" || fail 'same registry required copy'
: > "$GITHUB_OUTPUT"
bash "$tmp/copy-step.sh"
grep -qx 'required=true' "$GITHUB_OUTPUT" || fail 'cross registry omitted copy'
jq '.images.radar="invalid"' "$BUNDLE_FILE" > "$tmp/invalid-copy-plan.json"
: > "$GITHUB_OUTPUT"
reject env BUNDLE_FILE="$tmp/invalid-copy-plan.json" bash "$tmp/copy-step.sh"
[ ! -s "$GITHUB_OUTPUT" ] || fail 'invalid copy plan emitted decision'
ok 'composite copy decision distinguishes exact reuse and promotion and validates its plan'

yq -r '.jobs.operate.steps[] | select(.id == "scheduler") | .run' "$scripts/../../workflows/cloud-run-operations.yml" > "$tmp/scheduler-step.sh"
cat > "$tmp/bin/gcloud" <<'FAKE'
#!/usr/bin/env bash
set -eu
[ "$1 $2 $3" = 'scheduler jobs list' ]
[ "${QUERY_FAIL:-false}" != true ] || exit 1
cat "$FAKE_DIR/scheduler-jobs.json"
FAKE
printf '[]\n' > "$tmp/scheduler-jobs.json"
: > "$GITHUB_OUTPUT"
bash "$tmp/scheduler-step.sh"
grep -qx 'exists=false' "$GITHUB_OUTPUT" || fail 'empty scheduler list'
jq -n --arg name "projects/$PROJECT_ID/locations/$REGION/jobs/device-cleanup-daily" '[{name:$name},{name:"projects/other/locations/other/jobs/device-cleanup-daily"}]' > "$tmp/scheduler-jobs.json"
: > "$GITHUB_OUTPUT"
bash "$tmp/scheduler-step.sh"
grep -qx 'exists=true' "$GITHUB_OUTPUT" || fail 'exact scheduler name'
: > "$GITHUB_OUTPUT"
reject env QUERY_FAIL=true bash "$tmp/scheduler-step.sh"
[ ! -s "$GITHUB_OUTPUT" ] || fail 'scheduler query failure emitted decision'
printf '{}\n' > "$tmp/scheduler-jobs.json"
reject bash "$tmp/scheduler-step.sh"
[ ! -s "$GITHUB_OUTPUT" ] || fail 'invalid scheduler result emitted decision'
ok 'scheduler query distinguishes absence and exact name from query failures'
