-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS postgis;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS UUID AS $$
BEGIN
    RETURN uuidv7();
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_generate_v4()
RETURNS UUID AS $$
BEGIN
    RETURN gen_random_uuid();
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_location_from_lat_lng()
RETURNS TRIGGER AS $$
BEGIN
    NEW.location = ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    email CITEXT NOT NULL CHECK (length(email) <= 320),
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_users_email_active
    ON users(email)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_users_deleted_at
    ON users(deleted_at);

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN users.deleted_at IS
'Soft delete timestamp for user account. When set, all related data (addresses, devices, subscriptions) should be treated as deleted in queries.';

CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    loyalty_points INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_user_profiles_updated_at
    BEFORE UPDATE ON user_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE merchant_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    store_name TEXT NOT NULL,
    store_description TEXT,
    business_license TEXT,
    verification_status TEXT NOT NULL DEFAULT 'unverified',
    business_license_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT merchant_profiles_verification_status_check
        CHECK (verification_status IN ('unverified', 'verified'))
);

CREATE UNIQUE INDEX idx_merchant_profiles_business_license_active
    ON merchant_profiles(business_license)
    WHERE deleted_at IS NULL AND business_license IS NOT NULL;

CREATE INDEX idx_merchant_profiles_deleted_at
    ON merchant_profiles(deleted_at);

CREATE TRIGGER update_merchant_profiles_updated_at
    BEFORE UPDATE ON merchant_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE user_authentications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    password_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_auth_provider_provider_user_id_active
    ON user_authentications(provider, provider_user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_auth_user_id_provider_active
    ON user_authentications(user_id, provider)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_user_authentications_deleted_at
    ON user_authentications(deleted_at);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    family_id UUID NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT false,
    replaced_by UUID REFERENCES refresh_tokens(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_expires_at
    ON refresh_tokens(expires_at);

CREATE INDEX idx_refresh_tokens_user_id_expires
    ON refresh_tokens(user_id, expires_at);

CREATE INDEX idx_refresh_tokens_user_active_sessions
    ON refresh_tokens(user_id, is_revoked, expires_at);

CREATE INDEX idx_refresh_tokens_family_id
    ON refresh_tokens(family_id);

CREATE INDEX idx_refresh_tokens_revoked_created_at
    ON refresh_tokens(is_revoked, created_at);

CREATE TABLE addresses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_profile_id UUID REFERENCES user_profiles(user_id) ON DELETE CASCADE,
    merchant_profile_id UUID REFERENCES merchant_profiles(user_id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    full_address TEXT NOT NULL,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    location GEOMETRY(POINT, 4326),
    CHECK (
        (user_profile_id IS NOT NULL)::int +
        (merchant_profile_id IS NOT NULL)::int = 1
    ),
    CHECK (latitude BETWEEN -90 AND 90),
    CHECK (longitude BETWEEN -180 AND 180)
);

CREATE INDEX idx_addresses_user_profile_active
    ON addresses(user_profile_id, is_primary DESC, created_at)
    WHERE deleted_at IS NULL AND user_profile_id IS NOT NULL;

CREATE INDEX idx_addresses_merchant_profile_active
    ON addresses(merchant_profile_id, is_primary DESC, created_at)
    WHERE deleted_at IS NULL AND merchant_profile_id IS NOT NULL;

CREATE UNIQUE INDEX idx_addresses_user_primary
    ON addresses(user_profile_id)
    WHERE is_primary = TRUE AND user_profile_id IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX idx_addresses_merchant_primary
    ON addresses(merchant_profile_id)
    WHERE is_primary = TRUE AND merchant_profile_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_addresses_location
    ON addresses USING GIST(location);

CREATE INDEX idx_addresses_location_geography_active
    ON addresses USING GIST((location::geography))
    WHERE deleted_at IS NULL AND is_active = TRUE;

CREATE INDEX idx_addresses_deleted_at
    ON addresses(deleted_at);

CREATE TRIGGER update_addresses_updated_at
    BEFORE UPDATE ON addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_update_address_location
    BEFORE INSERT OR UPDATE ON addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_location_from_lat_lng();

COMMENT ON COLUMN addresses.deleted_at IS
'Soft delete timestamp for this address. Independent from user deletion - users can delete/restore addresses separately. Always check owner user deleted_at as well to ensure the owner account is active.';

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
    ON menu_items(merchant_id, deleted_at);

CREATE UNIQUE INDEX idx_menu_items_merchant_display_order_active
    ON menu_items(merchant_id, display_order)
    WHERE deleted_at IS NULL;

CREATE TRIGGER update_menu_items_updated_at
    BEFORE UPDATE ON menu_items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN menu_items.price IS
'Base price stored in minor currency units before any future promotion, discount, or campaign pricing rules. For TWD this currently matches whole dollars used by the UI. Store derived prices in separate pricing logic, not in this column.';

CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    attempt_key TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    failed_count INT NOT NULL DEFAULT 0,
    lockout_count INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_failed_at TIMESTAMPTZ,
    last_lockout_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_login_attempts_attempt_key
    ON login_attempts(attempt_key);

CREATE INDEX idx_login_attempts_user_id
    ON login_attempts(user_id);

CREATE INDEX idx_login_attempts_last_lockout_at
    ON login_attempts(last_lockout_at)
    WHERE last_lockout_at IS NOT NULL;

CREATE TRIGGER update_login_attempts_updated_at
    BEFORE UPDATE ON login_attempts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE user_devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fcm_token TEXT NOT NULL,
    device_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    token_refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_user_devices_fcm_token_unique
    ON user_devices(fcm_token)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_user_devices_user_device_unique
    ON user_devices(user_id, device_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_user_devices_user_device_lookup
    ON user_devices(user_id, device_id);

CREATE INDEX idx_user_devices_push_fanout
    ON user_devices(user_id, is_active, token_refreshed_at)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_user_devices_stale_cleanup
    ON user_devices(token_refreshed_at)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_user_devices_user_created
    ON user_devices(user_id, created_at DESC);

CREATE TRIGGER update_user_devices_updated_at
    BEFORE UPDATE ON user_devices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN user_devices.deleted_at IS
'Soft delete timestamp for this device. Independent from user deletion - users can remove/re-add devices. Always check users.deleted_at as well to ensure the user account is active.';

CREATE TABLE user_merchant_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    notification_radius DECIMAL(10, 2) NOT NULL DEFAULT 1000.0 CHECK (notification_radius >= 0 AND notification_radius <= 10000),
    subscribed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_subscriptions_user_id
    ON user_merchant_subscriptions(user_id, subscribed_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_subscriptions_merchant_id
    ON user_merchant_subscriptions(merchant_id, subscribed_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_subscriptions_merchant_active
    ON user_merchant_subscriptions(merchant_id, is_active, user_id)
    INCLUDE (notification_radius)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_subscriptions_unique_active
    ON user_merchant_subscriptions(user_id, merchant_id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER update_user_merchant_subscriptions_updated_at
    BEFORE UPDATE ON user_merchant_subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN user_merchant_subscriptions.deleted_at IS
'Soft delete timestamp for this subscription. Independent from user deletion - users can unsubscribe/resubscribe. Always check both user_id and merchant_id users.deleted_at to ensure both accounts are active.';

COMMENT ON COLUMN user_merchant_subscriptions.notification_radius IS
'Notification radius in meters. Defines the distance within which the user wants to receive notifications from the merchant.';

CREATE TABLE merchant_location_notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address_id UUID REFERENCES addresses(id) ON DELETE SET NULL,
    location_name TEXT NOT NULL,
    full_address TEXT NOT NULL,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    location GEOMETRY(POINT, 4326),
    hint_message TEXT,
    total_sent INTEGER NOT NULL DEFAULT 0,
    total_failed INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (latitude BETWEEN -90 AND 90),
    CHECK (longitude BETWEEN -180 AND 180)
);

CREATE INDEX idx_merchant_notifications_merchant_published
    ON merchant_location_notifications(merchant_id, published_at DESC);

CREATE INDEX idx_merchant_notifications_location
    ON merchant_location_notifications USING GIST(location);

CREATE INDEX idx_merchant_notifications_address_id
    ON merchant_location_notifications(address_id);

CREATE TRIGGER trigger_update_notification_location
    BEFORE INSERT OR UPDATE ON merchant_location_notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_location_from_lat_lng();

CREATE TRIGGER update_merchant_location_notifications_updated_at
    BEFORE UPDATE ON merchant_location_notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE notification_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    notification_id UUID NOT NULL REFERENCES merchant_location_notifications(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES user_devices(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'sent',
    fcm_message_id TEXT,
    error_message TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_logs_notification_id
    ON notification_logs(notification_id);

CREATE INDEX idx_notification_logs_user_id
    ON notification_logs(user_id);

CREATE INDEX idx_notification_logs_device_id
    ON notification_logs(device_id);

CREATE INDEX idx_notification_logs_sent_at
    ON notification_logs(sent_at DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_user_soft_delete_dependents()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE user_authentications
    SET deleted_at = NEW.deleted_at
    WHERE user_id = NEW.id
      AND deleted_at IS DISTINCT FROM NEW.deleted_at;

    UPDATE merchant_profiles
    SET deleted_at = NEW.deleted_at
    WHERE user_id = NEW.id
      AND deleted_at IS DISTINCT FROM NEW.deleted_at;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_users_sync_soft_delete_dependents
    AFTER UPDATE OF deleted_at ON users
    FOR EACH ROW
    WHEN (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
    EXECUTE FUNCTION sync_user_soft_delete_dependents();

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS merchant_location_notifications;
DROP TABLE IF EXISTS user_merchant_subscriptions;
DROP TABLE IF EXISTS user_devices;
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS addresses;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_authentications;
DROP TABLE IF EXISTS merchant_profiles;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS sync_user_soft_delete_dependents();
DROP FUNCTION IF EXISTS update_location_from_lat_lng();
DROP FUNCTION IF EXISTS uuid_generate_v4();
DROP FUNCTION IF EXISTS uuid_generate_v7();
DROP FUNCTION IF EXISTS update_updated_at_column();
