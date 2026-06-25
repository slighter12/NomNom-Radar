-- +goose Up
-- SQL in this section is executed when the migration is applied.

INSERT INTO discovery_categories (slug, name, display_order, status)
VALUES
    ('food', 'Food', 1, 'active'),
    ('snack', 'Snack', 2, 'active'),
    ('beverage', 'Beverage', 3, 'active'),
    ('goods', 'Goods', 4, 'active'),
    ('service', 'Service', 5, 'active'),
    ('other', 'Other', 6, 'active')
ON CONFLICT (slug) DO UPDATE
SET
    name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    status = EXCLUDED.status;

CREATE TEMP TABLE taxonomy_performance_merchants AS
SELECT mp.user_id
FROM merchant_profiles mp
JOIN discovery_subcategories ds ON ds.id = mp.discovery_subcategory_id
WHERE ds.slug = 'performance';

UPDATE merchant_profiles
SET discovery_subcategory_id = NULL
WHERE user_id IN (SELECT user_id FROM taxonomy_performance_merchants);

UPDATE discovery_categories
SET status = 'inactive'
WHERE slug = 'experience';

WITH subcategory_seed(category_slug, slug, name, display_order) AS (
    VALUES
        ('food', 'rice_noodles', 'Rice & Noodles', 1),
        ('food', 'grill', 'Grill', 2),
        ('food', 'fried', 'Fried Food', 3),
        ('food', 'light_meal', 'Light Meals', 4),
        ('snack', 'taiwanese', 'Taiwanese Snacks', 1),
        ('snack', 'bakery', 'Bakery', 2),
        ('snack', 'sweet', 'Sweets & Ices', 3),
        ('beverage', 'coffee', 'Coffee', 1),
        ('beverage', 'tea', 'Tea', 2),
        ('beverage', 'juice', 'Juice', 3),
        ('beverage', 'alcohol', 'Alcohol', 4),
        ('goods', 'handmade', 'Handmade Accessories', 1),
        ('goods', 'lifestyle', 'Lifestyle Goods', 2),
        ('goods', 'plants_pets', 'Plants & Pets', 3),
        ('service', 'portrait', 'Portraits & Photos', 1),
        ('service', 'workshop', 'Workshops', 2),
        ('service', 'performance', 'Performance', 3),
        ('service', 'repair', 'Repair Services', 4),
        ('other', 'other', 'Other', 1)
)
INSERT INTO discovery_subcategories (category_id, slug, name, display_order, status)
SELECT dc.id, seed.slug, seed.name, seed.display_order, 'active'
FROM subcategory_seed seed
JOIN discovery_categories dc ON dc.slug = seed.category_slug
ON CONFLICT (slug) DO UPDATE
SET
    category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    status = EXCLUDED.status;

UPDATE discovery_subcategories
SET status = 'inactive'
WHERE slug IN ('meal', 'snack', 'beverage', 'goods');

UPDATE merchant_profiles mp
SET
    discovery_category_id = ds.category_id,
    discovery_subcategory_id = ds.id
FROM discovery_subcategories ds
WHERE ds.slug = 'performance'
  AND mp.user_id IN (SELECT user_id FROM taxonomy_performance_merchants);

DROP TABLE taxonomy_performance_merchants;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

INSERT INTO discovery_categories (slug, name, display_order, status)
VALUES
    ('food', 'Food', 1, 'active'),
    ('experience', 'Experience', 2, 'active'),
    ('other', 'Other', 3, 'active')
ON CONFLICT (slug) DO UPDATE
SET
    name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    status = EXCLUDED.status;

CREATE TEMP TABLE taxonomy_performance_merchants AS
SELECT mp.user_id
FROM merchant_profiles mp
JOIN discovery_subcategories ds ON ds.id = mp.discovery_subcategory_id
WHERE ds.slug = 'performance';

UPDATE merchant_profiles
SET discovery_subcategory_id = NULL
WHERE user_id IN (SELECT user_id FROM taxonomy_performance_merchants);

UPDATE discovery_categories
SET status = 'inactive'
WHERE slug IN ('snack', 'beverage', 'goods', 'service');

WITH subcategory_seed(category_slug, slug, name, display_order) AS (
    VALUES
        ('food', 'meal', 'Meal', 1),
        ('food', 'snack', 'Snack', 2),
        ('food', 'beverage', 'Beverage', 3),
        ('experience', 'goods', 'Goods', 1),
        ('experience', 'performance', 'Performance', 2),
        ('other', 'other', 'Other', 1)
)
INSERT INTO discovery_subcategories (category_id, slug, name, display_order, status)
SELECT dc.id, seed.slug, seed.name, seed.display_order, 'active'
FROM subcategory_seed seed
JOIN discovery_categories dc ON dc.slug = seed.category_slug
ON CONFLICT (slug) DO UPDATE
SET
    category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    status = EXCLUDED.status;

UPDATE discovery_subcategories
SET status = 'inactive'
WHERE slug IN (
    'rice_noodles',
    'grill',
    'fried',
    'light_meal',
    'taiwanese',
    'bakery',
    'sweet',
    'coffee',
    'tea',
    'juice',
    'alcohol',
    'handmade',
    'lifestyle',
    'plants_pets',
    'portrait',
    'workshop',
    'repair'
);

UPDATE merchant_profiles mp
SET
    discovery_category_id = ds.category_id,
    discovery_subcategory_id = ds.id
FROM discovery_subcategories ds
WHERE ds.slug = 'performance'
  AND mp.user_id IN (SELECT user_id FROM taxonomy_performance_merchants);

DROP TABLE taxonomy_performance_merchants;
