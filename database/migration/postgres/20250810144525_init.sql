-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Ensure citext is available for case-insensitive email uniqueness
CREATE EXTENSION IF NOT EXISTS citext;

-- Step 1: Create the reusable trigger function for updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- +goose StatementBegin
-- PostgreSQL 18+ native UUIDv7 function
-- UUIDv7 provides time-ordered UUIDs for better index and query performance
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS UUID AS $$
BEGIN
    RETURN uuidv7();
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- +goose StatementBegin
-- Backward compatibility alias (for any code that might reference v4)
CREATE OR REPLACE FUNCTION uuid_generate_v4()
RETURNS UUID AS $$
BEGIN
    RETURN gen_random_uuid();
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- Step 2: Create all tables and their indexes
-- Table: users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    email CITEXT NOT NULL UNIQUE CHECK (length(email) <= 320),
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN users.deleted_at IS 
'Soft delete timestamp for user account. When set, all related data (addresses, devices, subscriptions) should be treated as deleted in queries.';

-- Table: user_authentications
CREATE TABLE user_authentications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    password_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_auth_provider_provider_user_id ON user_authentications(provider, provider_user_id);
CREATE INDEX idx_auth_user_id ON user_authentications(user_id);

-- Table: refresh_tokens
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- Table: user_profiles
CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    default_shipping_address TEXT,
    loyalty_points INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TRIGGER update_user_profiles_updated_at BEFORE UPDATE ON user_profiles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Table: merchant_profiles
CREATE TABLE merchant_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    store_name TEXT NOT NULL,
    store_description TEXT,
    business_license TEXT NOT NULL UNIQUE,
    store_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TRIGGER update_merchant_profiles_updated_at BEFORE UPDATE ON merchant_profiles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
-- We must drop objects in the reverse order of creation to respect dependencies.

-- Drop triggers first
DROP TRIGGER IF EXISTS update_merchant_profiles_updated_at ON merchant_profiles;
DROP TRIGGER IF EXISTS update_user_profiles_updated_at ON user_profiles;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop tables that have foreign keys pointing to 'users'
DROP TABLE IF EXISTS merchant_profiles;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_authentications;

-- Drop the 'users' table last
DROP TABLE IF EXISTS users;

-- Drop the trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS uuid_generate_v7();
DROP FUNCTION IF EXISTS uuid_generate_v4();
