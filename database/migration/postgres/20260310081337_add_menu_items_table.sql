-- +goose Up
-- +goose StatementBegin
CREATE TABLE menu_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    merchant_id UUID NOT NULL REFERENCES merchant_profiles(user_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    category TEXT NOT NULL,
    price INT NOT NULL CHECK (price >= 0),
    currency TEXT NOT NULL DEFAULT 'TWD',
    prep_minutes INT NOT NULL CHECK (prep_minutes > 0),
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    is_popular BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INT NOT NULL CHECK (display_order > 0),
    image_url TEXT,
    external_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_menu_items_merchant_deleted
    ON menu_items (merchant_id, deleted_at);

CREATE UNIQUE INDEX idx_menu_items_merchant_display_order_active
    ON menu_items (merchant_id, display_order)
    WHERE deleted_at IS NULL;

CREATE TRIGGER update_menu_items_updated_at
    BEFORE UPDATE ON menu_items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN menu_items.price IS
'Base price stored in minor currency units before any future promotion, discount, or campaign pricing rules. For TWD this currently matches whole dollars used by the UI. Store derived prices in separate pricing logic, not in this column.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_menu_items_updated_at ON menu_items;
DROP TABLE IF EXISTS menu_items;
-- +goose StatementEnd
