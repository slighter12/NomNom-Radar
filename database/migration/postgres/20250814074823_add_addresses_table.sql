-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Step 1: Remove the old single-address columns from existing tables.
ALTER TABLE user_profiles DROP COLUMN IF EXISTS default_shipping_address;
ALTER TABLE merchant_profiles DROP COLUMN IF EXISTS store_address;

-- Step 2: Create a new, unified table for all addresses.
-- This table uses a polymorphic design to associate with different owner types (users, merchants).
CREATE TABLE addresses (
    -- The unique ID for this address record.
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- The ID of the owner (can be a user's ID or a merchant's ID).
    owner_id UUID NOT NULL,
    -- The type of the owner, e.g., 'user_profile' or 'merchant_profile'.
    -- This, combined with owner_id, forms the polymorphic relationship.
    owner_type VARCHAR(255) NOT NULL,

    -- A user-defined label for the address, e.g., "Home", "Office", "Main Store".
    label VARCHAR(100) NOT NULL,
    -- The full, human-readable street address.
    full_address TEXT NOT NULL,
    
    -- Geographic coordinates for distance calculations.
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,

    -- A flag to mark one address as the primary one for an owner.
    is_primary BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Step 3: Create indexes for performance.
-- Index for fast lookups of all addresses belonging to a specific owner.
CREATE INDEX idx_addresses_on_owner ON addresses(owner_id, owner_type);
-- A partial unique index to enforce the business rule that an owner can have AT MOST ONE primary address.
-- This is a powerful PostgreSQL feature that guarantees data integrity at the database level.
CREATE UNIQUE INDEX idx_addresses_one_primary_per_owner ON addresses(owner_id, owner_type) WHERE is_primary = TRUE;

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