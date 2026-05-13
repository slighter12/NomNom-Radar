-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE discovery_categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    display_order INT NOT NULL CHECK (display_order > 0),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT discovery_categories_slug_unique UNIQUE (slug),
    CONSTRAINT discovery_categories_status_check
        CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_discovery_categories_status_order
    ON discovery_categories(status, display_order, name);

CREATE TRIGGER update_discovery_categories_updated_at
    BEFORE UPDATE ON discovery_categories
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE discovery_subcategories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    category_id UUID NOT NULL REFERENCES discovery_categories(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    display_order INT NOT NULL CHECK (display_order > 0),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT discovery_subcategories_slug_unique UNIQUE (slug),
    CONSTRAINT discovery_subcategories_id_category_unique UNIQUE (id, category_id),
    CONSTRAINT discovery_subcategories_status_check
        CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_discovery_subcategories_category_status_order
    ON discovery_subcategories(category_id, status, display_order, name);

CREATE TRIGGER update_discovery_subcategories_updated_at
    BEFORE UPDATE ON discovery_subcategories
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE hubs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    city TEXT NOT NULL,
    area_name TEXT NOT NULL,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT hubs_slug_unique UNIQUE (slug),
    CONSTRAINT hubs_type_check
        CHECK (type IN ('market', 'event', 'tourism_area', 'transit_area', 'other')),
    CONSTRAINT hubs_status_check
        CHECK (status IN ('active', 'inactive')),
    CONSTRAINT hubs_time_range_check
        CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at >= starts_at)
);

CREATE INDEX idx_hubs_status_city_area_name
    ON hubs(status, city, area_name, name);

CREATE INDEX idx_hubs_type_status
    ON hubs(type, status);

CREATE TRIGGER update_hubs_updated_at
    BEFORE UPDATE ON hubs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

ALTER TABLE merchant_profiles
    ADD COLUMN discovery_category_id UUID REFERENCES discovery_categories(id) ON DELETE RESTRICT,
    ADD COLUMN discovery_subcategory_id UUID REFERENCES discovery_subcategories(id) ON DELETE RESTRICT,
    ADD COLUMN active_hub_id UUID REFERENCES hubs(id) ON DELETE RESTRICT,
    ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT false,
    ADD CONSTRAINT merchant_profiles_subcategory_requires_category_check
        CHECK (discovery_subcategory_id IS NULL OR discovery_category_id IS NOT NULL),
    ADD CONSTRAINT merchant_profiles_subcategory_category_fk
        FOREIGN KEY (discovery_subcategory_id, discovery_category_id)
        REFERENCES discovery_subcategories(id, category_id)
        ON DELETE RESTRICT;

CREATE INDEX idx_merchant_profiles_discovery_public
    ON merchant_profiles(is_public, verification_status, discovery_category_id, discovery_subcategory_id, active_hub_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_merchant_profiles_active_hub
    ON merchant_profiles(active_hub_id)
    WHERE deleted_at IS NULL AND active_hub_id IS NOT NULL;

INSERT INTO discovery_categories (slug, name, display_order)
VALUES
    ('meal', 'Meal', 1),
    ('snack', 'Snack', 2),
    ('beverage', 'Beverage', 3),
    ('dessert', 'Dessert', 4),
    ('goods', 'Goods', 5),
    ('experience', 'Experience', 6),
    ('other', 'Other', 7);

INSERT INTO discovery_subcategories (category_id, slug, name, display_order)
SELECT id, 'grill', 'Grill', 1
FROM discovery_categories
WHERE slug = 'meal';

INSERT INTO discovery_subcategories (category_id, slug, name, display_order)
SELECT id, seed.slug, seed.name, seed.display_order
FROM discovery_categories
CROSS JOIN (
    VALUES
        ('fried_food', 'Fried Food', 1),
        ('bakery', 'Bakery', 2)
) AS seed(slug, name, display_order)
WHERE discovery_categories.slug = 'snack';

INSERT INTO discovery_subcategories (category_id, slug, name, display_order)
SELECT id, seed.slug, seed.name, seed.display_order
FROM discovery_categories
CROSS JOIN (
    VALUES
        ('coffee', 'Coffee', 1),
        ('tea', 'Tea', 2)
) AS seed(slug, name, display_order)
WHERE discovery_categories.slug = 'beverage';

INSERT INTO discovery_subcategories (category_id, slug, name, display_order)
SELECT id, seed.slug, seed.name, seed.display_order
FROM discovery_categories
CROSS JOIN (
    VALUES
        ('handmade', 'Handmade', 1),
        ('accessory', 'Accessory', 2)
) AS seed(slug, name, display_order)
WHERE discovery_categories.slug = 'goods';

INSERT INTO discovery_subcategories (category_id, slug, name, display_order)
SELECT id, 'workshop', 'Workshop', 1
FROM discovery_categories
WHERE slug = 'experience';

COMMENT ON TABLE discovery_categories IS
'Platform-defined main categories for public mobile vendor discovery.';

COMMENT ON TABLE discovery_subcategories IS
'Platform-defined discovery subcategories. Each subcategory belongs to exactly one main category.';

COMMENT ON TABLE hubs IS
'Platform-defined vendor aggregation points used by discovery search, hub pages, and future campaign/pass features.';

COMMENT ON COLUMN merchant_profiles.is_public IS
'Controls whether a merchant is eligible for public discovery after usecase-level eligibility checks.';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

ALTER TABLE merchant_profiles
    DROP CONSTRAINT IF EXISTS merchant_profiles_subcategory_category_fk,
    DROP CONSTRAINT IF EXISTS merchant_profiles_subcategory_requires_category_check,
    DROP COLUMN IF EXISTS is_public,
    DROP COLUMN IF EXISTS active_hub_id,
    DROP COLUMN IF EXISTS discovery_subcategory_id,
    DROP COLUMN IF EXISTS discovery_category_id;

DROP TABLE IF EXISTS hubs;
DROP TABLE IF EXISTS discovery_subcategories;
DROP TABLE IF EXISTS discovery_categories;
