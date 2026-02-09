-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Step 1: Remove the old single-address columns from existing tables.
ALTER TABLE user_profiles DROP COLUMN IF EXISTS default_shipping_address;
ALTER TABLE merchant_profiles DROP COLUMN IF EXISTS store_address;

-- Step 2: Create a new, unified table for all addresses.
-- This table uses nullable foreign keys to associate with different owner types (user_profiles, merchant_profiles).
CREATE TABLE addresses (
    -- The unique ID for this address record.
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),

    -- Nullable foreign keys - exactly one must be set
    user_profile_id UUID REFERENCES user_profiles(user_id) ON DELETE CASCADE,
    merchant_profile_id UUID REFERENCES merchant_profiles(user_id) ON DELETE CASCADE,

    -- A user-defined label for the address, e.g., "Home", "Office", "Main Store".
    label TEXT NOT NULL,
    -- The full, human-readable street address.
    full_address TEXT NOT NULL,

    -- Geographic coordinates for distance calculations.
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,

    -- A flag to mark one address as the primary one for an owner.
    is_primary BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure valid owner and coordinate constraints
    CHECK (
        (user_profile_id IS NOT NULL)::int +
        (merchant_profile_id IS NOT NULL)::int = 1
    ),
    CHECK (latitude BETWEEN -90 AND 90),
    CHECK (longitude BETWEEN -180 AND 180)
);

-- Step 3: Create indexes for performance.
-- Indexes for fast lookups of addresses by owner
CREATE INDEX idx_addresses_user_profile ON addresses(user_profile_id) WHERE user_profile_id IS NOT NULL;
CREATE INDEX idx_addresses_merchant_profile ON addresses(merchant_profile_id) WHERE merchant_profile_id IS NOT NULL;

-- Partial unique indexes to enforce that each owner can have AT MOST ONE primary address.
CREATE UNIQUE INDEX idx_addresses_user_primary
    ON addresses(user_profile_id)
    WHERE is_primary = TRUE AND user_profile_id IS NOT NULL;

CREATE UNIQUE INDEX idx_addresses_merchant_primary
    ON addresses(merchant_profile_id)
    WHERE is_primary = TRUE AND merchant_profile_id IS NOT NULL;

-- Step 4: Bind the updated_at trigger to the new table.
CREATE TRIGGER update_addresses_updated_at
    BEFORE UPDATE ON addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Step 1: Drop the new addresses table and its trigger.
DROP TRIGGER IF EXISTS update_addresses_updated_at ON addresses;
DROP TABLE IF EXISTS addresses;

-- Step 2: Re-add the old single-address columns to the original tables.
ALTER TABLE user_profiles ADD COLUMN default_shipping_address TEXT;
ALTER TABLE merchant_profiles ADD COLUMN store_address TEXT;
