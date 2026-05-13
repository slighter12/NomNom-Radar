# Tier 2 - Mobile Vendor Discovery Plan

This document records the intended Tier 2 work split for Mobile Vendor Discovery. It should guide implementation without over-prescribing internal structure. The implementer should follow existing repository patterns and optimize the concrete schema, APIs, repositories, and usecases around the current codebase.

Tier 2 should be handled as three separate implementation phases:

1. Discovery data foundation.
2. Merchant discovery profile.
3. Consumer discovery search.

## Direction

Tier 2 is the discovery foundation for later product tiers. It should establish stable platform-defined discovery data that Tier 3 and Tier 6 can reuse, rather than adding temporary fields that later need to be replaced.

Core decisions:

- Discovery is organized around platform-defined categories, subcategories, and hubs.
- Merchants may select only platform-defined discovery values; public free-form tags are out of scope.
- Hub identity should be established in Tier 2 because hub filtering is part of discovery search.
- Tier 3 should add hub pages, hub-focused UX, and bot-assisted review on top of the Tier 2 hub model.
- Tier 6 campaign/pass work should be able to attach to existing hub identity or a future participation table without changing Tier 2 discovery identities.
- Platform-defined discovery data is maintained by migration/seeder for v1, not admin CRUD.

## Phase 1 - Discovery Data Foundation

Goal: create the durable taxonomy and hub data model that discovery search and later tiers will use.

Required outcomes:

- Add persistent platform-defined category, subcategory, and hub concepts.
- Add merchant profile references needed for category, subcategory, public visibility, and one active hub.
- Seed initial categories: `meal`, `snack`, `beverage`, `dessert`, `goods`, `experience`, `other`.
- Seed initial subcategories from the current product notes: `coffee`, `tea`, `fried_food`, `grill`, `bakery`, `handmade`, `accessory`, `workshop`.
- Seed production hubs from the formal product/operations hub list when available. Do not invent production hub data.
- Keep IDs and slugs stable enough for public API filters and future campaign/pass references.

Implementation guidance:

- Use the repo's existing domain/entity, repository, GORM model, migration, and generated-query patterns.
- Add indexes and constraints that support lookup by ID/slug and discovery search filters.
- Keep hub as a first-class concept, not a generic tag.
- Avoid admin CRUD and any campaign/pass behavior in this phase.

Acceptance checklist:

- [ ] Category, subcategory, and hub data can be stored and queried.
- [ ] Merchant profiles can reference valid discovery values.
- [ ] Seeded categories and subcategories are present after migration/seeding.
- [ ] Hub seed path is ready for the formal production hub list.
- [ ] Later phases can resolve active discovery values by ID or slug.

## Phase 2 - Merchant Discovery Profile

Goal: allow merchants to configure the discovery data that controls whether and how they appear in consumer discovery.

Required outcomes:

- Add merchant-facing read/update behavior for discovery profile data.
- Merchants can set category, subcategory, optional active hub, and public visibility.
- Merchants can clear the active hub.
- Public visibility is gated so incomplete or untrusted merchants do not appear in consumer discovery.

Product rules:

- `is_public=true` requires a verified merchant, valid active category/subcategory, and an active primary merchant location.
- A selected subcategory must belong to the selected or existing category.
- A selected hub must be active.
- `is_public=false` should remain allowed even when the profile is incomplete.

Implementation guidance:

- Keep HTTP parsing and response mapping in handlers.
- Keep eligibility and validation rules in usecases.
- Keep persistence and lookup logic in repositories.
- Prefer existing domain error and response-envelope patterns unless a new client-branchable discovery error is necessary.

Acceptance checklist:

- [ ] Merchant can read current discovery profile.
- [ ] Merchant can update category, subcategory, hub, and public visibility.
- [ ] Invalid, inactive, or mismatched discovery values are rejected.
- [ ] Unverified merchants or merchants without active primary location cannot enable public discovery.
- [ ] Existing profile behavior remains compatible.

## Phase 3 - Consumer Discovery Search

Goal: expose the consumer read path for browsing, nearby search, and category pages.

Required outcomes:

- Add discovery list/search behavior for active categories, active hubs, and public merchants.
- Merchant search supports keyword, category, subcategory, hub, and coordinate-radius filters.
- Category pages reuse merchant search with category filters; they should not introduce a separate backend data path.
- Future Tier 3 hub pages should be able to reuse merchant search with hub filters.

Product rules:

- Consumer search returns only public, verified, active merchants with valid active discovery values and an active primary merchant location.
- Coordinate search defaults to `3000` meters when radius is omitted.
- Coordinate search radius is capped at `10000` meters.
- Coordinate search sorts by distance and includes distance in results.
- Non-coordinate search uses stable ordering and omits distance.
- Conflicting ID/slug filters for the same concept should be rejected.
- Empty result sets should return success with an empty list.

Implementation guidance:

- Use PostGIS for nearby filtering and distance where appropriate.
- Avoid N+1 lookups when returning merchant summary, discovery values, hub summary, and primary location.
- Reuse the existing API response envelope and pagination conventions.
- Do not implement hub pages, bot review, campaign/pass, redemption, or placeholder redemption APIs in this phase.

Acceptance checklist:

- [ ] Active categories and subcategories can be listed for clients.
- [ ] Active hubs can be listed for clients.
- [ ] Public merchant search supports keyword, category, subcategory, hub, and nearby filters.
- [ ] Category pages can be powered by merchant search alone.
- [ ] Search by hub can later power Tier 3 hub pages.
- [ ] Ineligible merchants are excluded from search.
- [ ] Coordinate and non-coordinate search responses follow the product rules above.

## Recommended Order

Implement in this order:

```txt
Phase 1 -> Phase 2 -> Phase 3
```

Phase 1 creates the stable data model, Phase 2 lets merchants populate it, and Phase 3 exposes it to consumers.

## Out of Scope

- Admin CRUD for platform discovery values.
- Hub pages.
- Bot-assisted hub review.
- Campaign, pass, redemption, or campaign participation APIs.
- Placeholder redemption endpoints.
- Public free-form tags.

## Status

This is the Tier 2 execution guide. Update this section as each phase is implemented.
