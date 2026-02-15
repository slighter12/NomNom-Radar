-- +goose NO TRANSACTION
-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Optimize slow queries on addresses table for both user and merchant lookups
-- Using partial indexes to only index active (non-deleted) records

-- Create new partial indexes for efficient lookups with deleted_at IS NULL
CREATE INDEX CONCURRENTLY idx_addresses_merchant_profile_active
    ON addresses (merchant_profile_id)
    WHERE deleted_at IS NULL AND merchant_profile_id IS NOT NULL;

CREATE INDEX CONCURRENTLY idx_addresses_user_profile_active
    ON addresses (user_profile_id)
    WHERE deleted_at IS NULL AND user_profile_id IS NOT NULL;

-- Drop old single-column indexes (replaced by partial indexes)
DROP INDEX CONCURRENTLY IF EXISTS idx_addresses_merchant_profile;
DROP INDEX CONCURRENTLY IF EXISTS idx_addresses_user_profile;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Recreate original partial indexes
CREATE INDEX idx_addresses_merchant_profile
    ON addresses (merchant_profile_id)
    WHERE merchant_profile_id IS NOT NULL;

CREATE INDEX idx_addresses_user_profile
    ON addresses (user_profile_id)
    WHERE user_profile_id IS NOT NULL;

-- Drop new partial indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_addresses_merchant_profile_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_addresses_user_profile_active;
