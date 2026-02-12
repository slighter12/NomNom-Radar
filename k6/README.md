# K6 Test Scenarios

This folder provides two k6 test scenarios:

- `smoke.js`: high-frequency login smoke for member + merchant
- `full.js`: end-to-end write/read flow with authenticated API paths

## Prerequisites

- k6 installed locally
- API server reachable from the machine running k6

## 1) Smoke Test

Runs high-frequency login checks for both member and merchant accounts.

Before traffic starts, `setup()` will:

- Auto-generate a member account pool
- Auto-generate a merchant account pool
- Register all generated accounts (`/auth/register/user`, `/auth/register/merchant`)
- Accept `201` (created) or `409` (already exists)

```bash
k6 run k6/smoke.js \
  -e BASE_URL=http://localhost:4433 \
  -e SMOKE_RUN_ID=$(date +%s) \
  -e SMOKE_PASSWORD='K6SmokePass1!' \
  -e SMOKE_POOL_SIZE=300 \
  -e SMOKE_SETUP_TIMEOUT=10m \
  -e SMOKE_VUS=80 \
  -e SMOKE_DURATION=2m
```

Optional tuning:

- `SMOKE_VUS` (default: `20`)
- `SMOKE_DURATION` (default: `1m`)
- `SMOKE_SLEEP_SECONDS` (default: `0`)
- `SMOKE_POOL_SIZE` (default: `200`)
- `SMOKE_SETUP_TIMEOUT` (default: `10m`)
- `SMOKE_RUN_ID` (default: current timestamp)
- `SMOKE_PASSWORD` (default: `K6SmokePass1!`)
- `SMOKE_DO_LOGOUT` (default: `true`)

When backend session limit is enabled (reject policy), keep `SMOKE_DO_LOGOUT=true` to avoid hitting the limit during smoke traffic.

## 2) Full Test

Runs complete flow:

1. Setup merchant account (register or reuse + login).
2. Register/login user.
3. `GET /user/profile`.
4. User location create/update/delete.
5. Device register/get/update/deactivate.
6. Subscription subscribe/list/unsubscribe.
7. Merchant location create/delete.
8. Merchant notification publish/history.
9. Logout user + merchant session.

Example:

```bash
k6 run k6/full.js \
  -e BASE_URL=http://localhost:4433 \
  -e RUN_ID=$(date +%s) \
  -e K6_TEST_PASSWORD='K6pass!1234'
```

Optional tuning:

- `FULL_START_VUS` (default: `1`)
- `FULL_TARGET_VUS` (default: `8`)
- `FULL_RAMP_UP` (default: `1m`)
- `FULL_STEADY` (default: `3m`)
- `FULL_RAMP_DOWN` (default: `1m`)
- `FULL_SLEEP_SECONDS` (default: `0.2`)
- `FULL_MERCHANT_EMAIL` (optional fixed merchant account)
- `FULL_MERCHANT_NAME`, `FULL_STORE_NAME`, `FULL_BUSINESS_LICENSE` (optional setup identity fields)

## Recommended Environment/Data Workflow

Because `full.js` writes test data, run it on staging/perf environments.

Recommended order:

1. Take a database snapshot/backup before full tests.
2. Run `smoke.js` first.
3. Run `full.js`.
4. Validate metrics and logs.
5. Cleanup by restoring from pre-test snapshot, or by running targeted cleanup SQL for test records (by `RUN_ID` naming convention).

Avoid running `full.js` directly on production data.
