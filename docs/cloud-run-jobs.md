# Cloud Run Jobs

This project uses Cloud Run Jobs for periodic maintenance work that should not be exposed as public HTTP API endpoints.

## Device Cleanup

`cmd/device-cleanup` soft-deletes user devices whose `token_refreshed_at` is older than 270 days. This aligns with the FCM Android token expiration threshold and keeps permanently stale tokens out of push fanout.

### Build Image

Build the dedicated Docker target:

```sh
docker build \
  --target device-cleanup \
  --platform linux/amd64 \
  -t DEVICE_CLEANUP_IMAGE \
  .
```

CI builds and publishes this target as:

```text
${REGISTRY}/${IMAGE_NAME}/device-cleanup:${TAG}
${REGISTRY}/${IMAGE_NAME}/device-cleanup:latest
```

### Deploy Job

Deploy the image as a Cloud Run Job. Use the same database and logging environment variables as the `radar` service; Firebase and HTTP server settings are not required.

`POSTGRES_PRESET` is consumed by the shared Postgres configuration helper through environment override and preset handling; it is not a YAML key in this repository's config files.

```sh
gcloud run jobs deploy device-cleanup \
  --image DEVICE_CLEANUP_IMAGE \
  --region REGION \
  --service-account SERVICE_ACCOUNT \
  --set-env-vars ENV_LOG_PRETTY=false,ENV_LOG_LEVEL=info,POSTGRES_PRESET=supabase_transaction,POSTGRES_SSLMODE=require,POSTGRES_MAXOPENCONNS=5 \
  --set-secrets POSTGRES_MASTER_DSN=postgres-master-dsn:latest
```

### Schedule

Create a Cloud Scheduler trigger that executes the job once per day:

```sh
gcloud scheduler jobs create http device-cleanup-daily \
  --location REGION \
  --schedule "0 3 * * *" \
  --uri "https://REGION-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/PROJECT_ID/jobs/device-cleanup:run" \
  --http-method POST \
  --oauth-service-account-email SERVICE_ACCOUNT
```

The service account used by Cloud Scheduler must have permission to run the Cloud Run Job.

### Manual Run

```sh
gcloud run jobs execute device-cleanup \
  --region REGION \
  --wait
```

Expected log fields:

- `stale_days`: fixed at `270`
- `rows_affected`: number of devices soft-deleted by the run

Operational follow-up:

- Add metrics and alerting for repeated zero-row runs or unexpected spikes in `rows_affected` once the monitoring stack is in place.
