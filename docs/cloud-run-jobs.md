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

### Deploy with GitHub Actions

Use the dedicated workflow for the target environment:

- `Deploy Cloud Run Job Dev`
- `Deploy Cloud Run Job Prod`

Inputs:

| Input | Description |
|-------|-------------|
| `target` | Cloud Run Job target. Currently only `device-cleanup`. |
| `image_ref` | Required image tag or commit SHA to deploy. |
| `run_migration` | Runs the shared database migrations before deploy when this release includes schema changes. Defaults to `false`. |
| `configure_scheduler` | Creates or updates the Cloud Scheduler trigger configured by `schedule`, `schedule_time_zone`, and `scheduler_name`. Leave disabled for image-only job deploys. Defaults to `false`. |
| `execute_now` | Executes the job immediately after deploy. Defaults to `false`. |
| `schedule` | Cloud Scheduler cron expression. Defaults to `0 3 * * *`. |
| `schedule_time_zone` | Cloud Scheduler time zone. Defaults to `Asia/Taipei`. |
| `scheduler_name` | Cloud Scheduler job name. Defaults to `device-cleanup-daily` for this job; change it if `schedule` is no longer daily or if adding another job target. |

The workflow reuses the shared Cloud Run deployment setup used by services: Google auth, Artifact Registry image resolution, optional migration, and image existence verification. If `run_migration` is enabled, it runs the same goose migration path used by service deploys before deploying the job. The deployment branch for jobs runs `gcloud run jobs deploy` instead of `gcloud run services replace`.

The job deploy sets the same database and logging runtime configuration documented below. Firebase and HTTP server settings are not required.

Prerequisites for Scheduler management:

- `cloudscheduler.googleapis.com` must be enabled in the target GCP project.
- The workflow deployer service account needs permission to create or update Cloud Scheduler jobs, for example `roles/cloudscheduler.jobEditor`, and `roles/iam.serviceAccountUser` on the scheduler service account.
- The service account used by Cloud Scheduler must have permission to run the Cloud Run Job, for example `roles/run.invoker` on the target job or project.

### Manual Deploy Fallback

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

Cloud Scheduler is configured separately from the job image deploy. The GitHub Actions workflow creates or updates this scheduler only when `configure_scheduler` is enabled. Leave it disabled for normal image-only job deploys. Enable it when first creating the scheduler or intentionally changing `scheduler_name`, `schedule`, `schedule_time_zone`, URI, HTTP method, or OAuth scope.

For manual fallback, create a Cloud Scheduler trigger that executes the job once per day:

```sh
gcloud scheduler jobs create http device-cleanup-daily \
  --location REGION \
  --schedule "0 3 * * *" \
  --time-zone "Asia/Taipei" \
  --uri "https://run.googleapis.com/v2/projects/PROJECT_ID/locations/REGION/jobs/device-cleanup:run" \
  --http-method POST \
  --oauth-service-account-email SERVICE_ACCOUNT \
  --oauth-token-scope "https://www.googleapis.com/auth/cloud-platform"
```

The workflow creates or updates the scheduler trigger, but it does not grant IAM. The service account used by Cloud Scheduler must already have permission to run the Cloud Run Job, for example `roles/run.invoker` on the target job or project.

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
