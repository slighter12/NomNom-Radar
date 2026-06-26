# Tier 2 Discovery Playbook

This is compact historical background. It preserves the discovery decisions that still guide future work.

## Purpose

Tier 2 established mobile-vendor discovery:

- Platform-defined categories and subcategories.
- Platform-defined hubs.
- Merchant discovery profile fields.
- Authenticated consumer search over publicly visible merchants.

## Final Status

The backend discovery foundation is implemented:

- Active categories, subcategories, and hubs can be listed.
- Merchants can read and update discovery profile data.
- Public visibility is gated by merchant eligibility.
- Search supports keyword, category, subcategory, hub, and nearby filters.
- Category pages and future hub pages should reuse merchant search.

## Current Entry Points

- Discovery routes: `internal/delivery/api/router/router.go`
- Discovery handler: `internal/delivery/api/router/handler/discovery_handler.go`
- Discovery usecase: `internal/usecase/impl/discovery_service.go`
- Discovery repository: `internal/infra/persistence/postgres/discovery_repository.go`
- Product direction: `docs/product.md`

## Key Decisions

- Discovery values are platform-defined, not free-form merchant public tags.
- Hubs are first-class discovery concepts, not generic labels.
- A merchant has zero or one active hub in the current model.
- Consumer-facing discovery data can be publicly visible while the API still requires authentication.
- Campaign/pass/redemption work remains future scope and should not be added as placeholder APIs.

## Minimal Flow

1. Platform seed data defines categories, subcategories, and hubs.
2. Merchant sets discovery profile fields and public visibility.
3. Eligibility rules prevent incomplete or untrusted merchants from appearing.
4. Authenticated consumers list discovery values or search visible merchants.
5. Future category or hub pages reuse the same search path.

## Read Next

- `docs/product.md`
- `docs/roadmap.md`
- `docs/architecture.md`
