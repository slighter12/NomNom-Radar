-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Step 1: Enable PostGIS extension for geospatial queries
CREATE EXTENSION IF NOT EXISTS postgis;

-- Step 2: Extend existing addresses table with location notification features
ALTER TABLE addresses 
ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN deleted_at TIMESTAMPTZ,
ADD COLUMN location GEOMETRY(POINT, 4326);

-- Populate location column using existing latitude and longitude
UPDATE addresses 
SET location = ST_SetSRID(ST_MakePoint(longitude, latitude), 4326);

-- Create spatial index for geolocation queries
CREATE INDEX idx_addresses_location ON addresses USING GIST(location);
CREATE INDEX idx_addresses_deleted_at ON addresses(deleted_at);

COMMENT ON COLUMN addresses.deleted_at IS 
'Soft delete timestamp for this address. Independent from user deletion - users can delete/restore addresses separately. Always check owner user deleted_at as well to ensure the owner account is active.';

-- Update unique indexes to respect soft deletes
DROP INDEX IF EXISTS idx_addresses_user_primary;
DROP INDEX IF EXISTS idx_addresses_merchant_primary;

CREATE UNIQUE INDEX idx_addresses_user_primary 
    ON addresses(user_profile_id) 
    WHERE is_primary = TRUE AND user_profile_id IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX idx_addresses_merchant_primary 
    ON addresses(merchant_profile_id) 
    WHERE is_primary = TRUE AND merchant_profile_id IS NOT NULL AND deleted_at IS NULL;

-- Step 3: Create shared trigger function to automatically update location column from lat/lng
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_location_from_lat_lng()
RETURNS TRIGGER AS $$
BEGIN
    NEW.location = ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326);
    RETURN NEW;
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER trigger_update_address_location
    BEFORE INSERT OR UPDATE ON addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_location_from_lat_lng();

-- Step 4: Create user devices table for FCM token management
CREATE TABLE user_devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fcm_token VARCHAR(255) NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Unique constraints that respect soft deletes (only apply to non-deleted records)
CREATE UNIQUE INDEX idx_user_devices_fcm_token_unique 
    ON user_devices(fcm_token) 
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_user_devices_user_device_unique 
    ON user_devices(user_id, device_id) 
    WHERE deleted_at IS NULL;

-- Regular indexes for queries
CREATE INDEX idx_user_devices_user_id ON user_devices(user_id);
CREATE INDEX idx_user_devices_active ON user_devices(is_active) WHERE is_active = true;
CREATE INDEX idx_user_devices_deleted_at ON user_devices(deleted_at);

-- Create updated_at trigger
CREATE TRIGGER update_user_devices_updated_at
    BEFORE UPDATE ON user_devices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN user_devices.deleted_at IS 
'Soft delete timestamp for this device. Independent from user deletion - users can remove/re-add devices. Always check users.deleted_at as well to ensure the user account is active.';

-- Step 5: Create user merchant subscriptions table
CREATE TABLE user_merchant_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    notification_radius DECIMAL(10, 2) NOT NULL DEFAULT 1000.0,
    subscribed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_subscriptions_user_id ON user_merchant_subscriptions(user_id);
CREATE INDEX idx_subscriptions_merchant_id ON user_merchant_subscriptions(merchant_id);
CREATE INDEX idx_subscriptions_active ON user_merchant_subscriptions(is_active);
CREATE INDEX idx_subscriptions_deleted_at ON user_merchant_subscriptions(deleted_at);

-- Adjust unique constraint for soft delete: only applies to non-deleted records
CREATE UNIQUE INDEX idx_subscriptions_unique_active 
ON user_merchant_subscriptions(user_id, merchant_id) 
WHERE deleted_at IS NULL;

-- Create updated_at trigger
CREATE TRIGGER update_user_merchant_subscriptions_updated_at
    BEFORE UPDATE ON user_merchant_subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN user_merchant_subscriptions.deleted_at IS 
'Soft delete timestamp for this subscription. Independent from user deletion - users can unsubscribe/resubscribe. Always check both user_id and merchant_id users.deleted_at to ensure both accounts are active.';

COMMENT ON COLUMN user_merchant_subscriptions.notification_radius IS 
'Notification radius in meters. Defines the distance within which the user wants to receive notifications from the merchant.';

-- Step 6: Create merchant location notifications table
-- This table stores location-based notifications sent by merchants.
-- Merchants can either select from saved addresses or input a temporary location.
CREATE TABLE merchant_location_notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Optional reference to a saved address in the addresses table.
    -- NULL if merchant used a temporary/custom location instead of quick selection.
    -- ON DELETE SET NULL preserves notification history even if the address is deleted.
    address_id UUID REFERENCES addresses(id) ON DELETE SET NULL,
    
    -- Snapshot of location data at the time of notification.
    -- These fields are always required regardless of whether address_id is set.
    -- This ensures historical accuracy even if the referenced address is modified or deleted.
    location_name VARCHAR(255) NOT NULL,
    full_address TEXT NOT NULL,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    location GEOMETRY(POINT, 4326),
    
    hint_message TEXT,
    total_sent INTEGER NOT NULL DEFAULT 0,
    total_failed INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_merchant_notifications_merchant_id ON merchant_location_notifications(merchant_id);
CREATE INDEX idx_merchant_notifications_location ON merchant_location_notifications USING GIST(location);
CREATE INDEX idx_merchant_notifications_published_at ON merchant_location_notifications(published_at);

-- Create trigger using shared function
CREATE TRIGGER trigger_update_notification_location
    BEFORE INSERT OR UPDATE ON merchant_location_notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_location_from_lat_lng();

-- Create updated_at trigger
CREATE TRIGGER update_merchant_location_notifications_updated_at
    BEFORE UPDATE ON merchant_location_notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Step 7: Create notification logs table
CREATE TABLE notification_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    notification_id UUID NOT NULL REFERENCES merchant_location_notifications(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES user_devices(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'sent',
    fcm_message_id VARCHAR(255),
    error_message TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_logs_notification_id ON notification_logs(notification_id);
CREATE INDEX idx_notification_logs_user_id ON notification_logs(user_id);
CREATE INDEX idx_notification_logs_device_id ON notification_logs(device_id);
CREATE INDEX idx_notification_logs_status ON notification_logs(status);
CREATE INDEX idx_notification_logs_sent_at ON notification_logs(sent_at);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Step 1: Remove tables (in dependency order)
DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS merchant_location_notifications;
DROP TABLE IF EXISTS user_merchant_subscriptions;
DROP TABLE IF EXISTS user_devices;

-- Step 2: Remove triggers and functions
DROP TRIGGER IF EXISTS trigger_update_notification_location ON merchant_location_notifications;
DROP TRIGGER IF EXISTS trigger_update_address_location ON addresses;
DROP FUNCTION IF EXISTS update_location_from_lat_lng();

-- Step 3: Remove added columns from addresses table
ALTER TABLE addresses 
DROP COLUMN IF EXISTS location,
DROP COLUMN IF EXISTS deleted_at,
DROP COLUMN IF EXISTS is_active;
