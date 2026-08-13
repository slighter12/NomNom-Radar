# Cloud Run Jobs

This reference owns only Cloud Run Job-specific behavior and settings. Shared
release, attestation, migration, identity, lock, readiness, and break-glass
policy is defined in [`docs/operations.md`](../operations.md).

## Device Cleanup

`cmd/device-cleanup` soft-deletes devices whose `token_refreshed_at` is older
than 270 days, keeping permanently stale FCM tokens out of push fanout.

### Image and Deployment

Build the dedicated image target locally with:

```sh
docker build \
  --target device-cleanup \
  --platform linux/amd64 \
  -t DEVICE_CLEANUP_IMAGE \
  .
```

CI publishes the target as:

```text
${GCP_DEV_REGISTRY}/${IMAGE_NAME}/device-cleanup:${RELEASE_SHA}
```

`IMAGE_NAME` is the lowercase repository owner and `RELEASE_SHA` is the
selected full `main` candidate SHA. Use the unified `Release Cloud Run`
workflow with its `environment` input. The job deploys between `geoworker` and
`radar` with the same `release-sha` label and exact-digest contract.

The release-owned template is
[`deploy/cloud-run/jobs/device-cleanup.yaml`](../../deploy/cloud-run/jobs/device-cleanup.yaml).
The job shares Radar's database and logging configuration, uses a maximum of
five open database connections, and does not require Firebase or HTTP server
settings. `POSTGRES_PRESET` is consumed through environment override; it is not
a YAML config key.

For approved break-glass recovery only:

```sh
# Render deploy/cloud-run/jobs/device-cleanup.yaml with the approved exact
# image, service account, and release-sha, then replace the job:
gcloud run jobs replace device-cleanup.yaml --region REGION --project PROJECT_ID
```

Follow the approval and exact-digest rules in
[`docs/operations.md`](../operations.md); this is not a routine deployment
path.

### Scheduler

Use `Cloud Run Operations` with
`configure-device-cleanup-scheduler`. The resource name is always
`device-cleanup-daily`; schedule and time zone default to `0 3 * * *` and
`Asia/Taipei`. Release never creates, updates, or executes the scheduler.

Job-specific prerequisites:

- `cloudscheduler.googleapis.com` is enabled in the target project.
- The operations identity can edit scheduler jobs and `actAs` the configured
  scheduler caller.
- `GCP_SCHEDULER_SA_EMAIL` differs from the Cloud Run runtime identity and has
  only job-scoped `roles/run.invoker` on `device-cleanup`.

The workflow creates or updates the trigger but does not grant IAM. Manual
fallback, under the break-glass policy, is:

```sh
gcloud scheduler jobs create http device-cleanup-daily \
  --project PROJECT_ID \
  --location REGION \
  --schedule "0 3 * * *" \
  --time-zone "Asia/Taipei" \
  --uri "https://run.googleapis.com/v2/projects/PROJECT_ID/locations/REGION/jobs/device-cleanup:run" \
  --http-method POST \
  --oauth-service-account-email SCHEDULER_SA_EMAIL \
  --oauth-token-scope "https://www.googleapis.com/auth/cloud-platform"
```

### Execution

Prefer `Cloud Run Operations` with `execute-device-cleanup`. Manual execution
is break-glass only:

```sh
gcloud run jobs execute device-cleanup \
  --region REGION \
  --wait
```

Expected log fields are `stale_days` (fixed at `270`) and `rows_affected`.
Monitoring should alert on repeated zero-row runs or unexpected spikes once
the monitoring stack exists.
