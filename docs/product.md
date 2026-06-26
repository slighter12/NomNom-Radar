# Product Brief

NomNom-Radar is a backend for mobile-vendor and market discovery. It helps consumers find mobile vendors and market-style vendor clusters, and it lets vendors notify subscribed users when they are nearby.

The v1 product direction is reliable notifications plus discovery. Future market partnership features, including electronic passes and redemption, should remain possible, but they are not part of the v1 backend scope.

## Product Positioning

NomNom-Radar combines two related surfaces:

- Mobile-vendor discovery: authenticated consumers browse and search publicly visible mobile vendors by category, hub, keyword, and nearby location.
- Location-triggered notification: vendors publish their current location, and subscribed consumers receive route-aware push notifications when they are in range.

Markets and hubs are discovery concepts today. A hub represents a market, gathering, tourism area, transit area, event area, or other vendor aggregation point. Hubs should stay platform-defined until there is a real organizer workflow.

## Current Actors

- Consumer: subscribes to merchants, manages device/location records, receives push notifications, and searches publicly visible merchants through authenticated APIs.
- Merchant: maintains profile, menu, locations, discovery fields, QR subscription surface, verification status, and notification history.
- Platform operator: defines categories, subcategories, and hubs; manages deployment and runtime infrastructure.
- Market or organizer partner: future actor. Do not model as an account type until the product needs organizer-owned management.

## V1 Scope

V1 should focus on:

- Auth and session reliability.
- Device registration, health, stale-token cleanup, and rebind support.
- Merchant verification and lightweight self-service profile maintenance.
- Menu and merchant location management.
- QR-based merchant subscription.
- Authenticated consumer discovery over publicly visible merchants with category, subcategory, hub, keyword, and nearby filters.
- Location notification publishing with route-aware delivery and safe fallback.

The existing merchant operations surface is intentionally lightweight. It is not a full POS, CRM, analytics, or campaign-management product.

## Non-Goals

Do not add these as v1 requirements without a new product decision:

- Electronic pass, coupon, ticket, or redemption flows.
- Market organizer accounts or organizer-owned hub management.
- Full merchant operations dashboard.
- Scheduled notification workflow.
- Notification analytics and subscriber operations dashboard.
- Public free-form tags.
- Placeholder APIs for future redemption or campaign features.

## Future Direction

Future market partnership work may support a flow similar to replacing paper tourist-area or market passes with electronic passes. Keep that direction as a product principle only:

- Discovery and hub identity should make future campaigns attachable.
- A future pass/redemption model should be explicit and auditable.
- Organizer accounts should be introduced only when a partner needs to manage markets, vendors, campaigns, or passes directly.
- Merchant advanced operations should be driven by user feedback, not prebuilt for completeness.

## Product Rules

- Discovery values are platform-defined, not merchant-created free-form public tags.
- Use stable IDs and slugs for public filters.
- A merchant may have zero or one active hub in the current model.
- Hub pages should reuse merchant search rather than create a parallel backend path.
- Campaign/pass features should use dedicated resources when implemented; do not add no-op redemption endpoints early.
