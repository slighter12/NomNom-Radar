-- +goose Up
-- SQL in this section is executed when the migration is applied.

ALTER TABLE menu_items
    ADD COLUMN category_id UUID;

CREATE INDEX idx_menu_items_category_id
    ON menu_items(category_id);

COMMENT ON COLUMN menu_items.category_id IS
'Discovery subcategory selected by the merchant for this menu item. Intentionally not constrained by foreign key so merchants can repair stale taxonomy references.';

ALTER TABLE menu_items
    DROP COLUMN category;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP INDEX IF EXISTS idx_menu_items_category_id;

ALTER TABLE menu_items
    DROP COLUMN IF EXISTS category_id;

-- Original menu category values cannot be recovered after category is dropped.
-- Use a valid legacy value only to restore the old NOT NULL schema shape.
ALTER TABLE menu_items
    ADD COLUMN category TEXT NOT NULL DEFAULT 'main';

ALTER TABLE menu_items
    ALTER COLUMN category DROP DEFAULT;
