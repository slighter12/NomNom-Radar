-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE INDEX idx_addresses_primary_merchant_location_geography_active
    ON addresses USING GIST((location::geography))
    WHERE deleted_at IS NULL
      AND merchant_profile_id IS NOT NULL
      AND is_primary = TRUE
      AND is_active = TRUE;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP INDEX IF EXISTS idx_addresses_primary_merchant_location_geography_active;
