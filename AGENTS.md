# AGENTS.md - NomNom-Radar Agent Guide

These instructions apply to the whole repository.

## Working Rules

- Chat with the user in Traditional Chinese.
- Keep repository artifacts in English.
- Keep changes minimal. Do not refactor, rename public APIs, add dependencies, or introduce speculative abstractions unless the task explicitly requires it.
- Preserve unrelated worktree changes and do not stage, commit, deploy, or run migrations unless explicitly requested.
- Treat current docs and code as the source of truth. Call out drift instead of copying stale statements.
- Do not turn future directions, deferred work, or historical plans into requirements without an explicit product decision.

## Task Context

- Read only the context needed for the task.
- Use `docs/product.md` and `docs/roadmap.md` for product scope, non-goals, status, and future direction.
- Use `docs/architecture.md` for runtime services, data flow, integrations, and package boundaries.
- Use `docs/operations.md` and the relevant file under `docs/reference/` for deployment, database, or API-specific work.
- Treat `docs/history/` as background only. It must not override current docs or code.

## Runtime and Ownership

- Runtime entrypoints are `cmd/radar`, `cmd/geoworker`, and `cmd/device-cleanup`.
- `cmd/routing`, `internal/infra/routing/ch`, and `internal/infra/routing/loader` are legacy or offline tooling, not the notification runtime path.
- Follow the existing dependency direction: delivery -> usecase -> domain <- infra.
- Keep HTTP and worker parsing, transport validation, and response mapping in delivery packages.
- Keep product validation and authorization-sensitive orchestration in usecases.
- Keep entities, policies, canonical errors, and repository/service contracts in domain packages.
- Keep persistence and external integration details in infra packages.
- Keep `internal/platform` limited to cross-layer runtime primitives such as correlation context and request-scoped logging.

## Error and Observability Boundaries

- Preserve canonical `AppError` identities, response envelopes, HTTP semantics, request IDs, auth boundaries, and config names.
- Capture source stacks where errors originate or where their semantic identity is replaced. Preserve existing stack providers instead of wrapping repeatedly.
- Do not emit duplicate lower-layer error logs. The API request logger owns the final request lifecycle log.
- Never return stack traces, SQL details, internal paths, or raw internal errors to clients.
- Never log secrets, tokens, passwords, PII, or full request/response bodies. Keep logged request and error details sanitized.

## Database Migrations

- Put shared PostgreSQL schema changes in `database/migration/postgres/`.
- Put Supabase extension/bootstrap changes in `database/migration/supabase/pre/` and function hardening in `database/migration/supabase/post/`.
- Add a new migration instead of rewriting one already applied to shared environments.
- Use the existing Supabase pre/shared/post deployment flow. Follow `docs/operations.md` and the active deployment reference for connection and ordering constraints.

## Verification

- Do not run broad test suites unless requested or needed for non-trivial code changes.
- For docs-only changes, verify referenced paths and current-status claims manually; do not run programs.
- For code changes, run the narrowest relevant check and report any skipped verification.
- For migration changes, validate migration structure without applying it unless execution was explicitly requested.
