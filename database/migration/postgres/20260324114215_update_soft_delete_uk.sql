-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_authentications
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE merchant_profiles
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_user_authentications_deleted_at
    ON user_authentications(deleted_at);

CREATE INDEX IF NOT EXISTS idx_merchant_profiles_deleted_at
    ON merchant_profiles(deleted_at);

UPDATE user_authentications AS ua
SET deleted_at = u.deleted_at
FROM users AS u
WHERE ua.user_id = u.id
  AND u.deleted_at IS NOT NULL
  AND ua.deleted_at IS DISTINCT FROM u.deleted_at;

UPDATE merchant_profiles AS mp
SET deleted_at = u.deleted_at
FROM users AS u
WHERE mp.user_id = u.id
  AND u.deleted_at IS NOT NULL
  AND mp.deleted_at IS DISTINCT FROM u.deleted_at;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_email_key;

DROP INDEX IF EXISTS idx_auth_provider_provider_user_id;

ALTER TABLE merchant_profiles
    DROP CONSTRAINT IF EXISTS merchant_profiles_business_license_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active
    ON users(email)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_provider_provider_user_id_active
    ON user_authentications(provider, provider_user_id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_profiles_business_license_active
    ON merchant_profiles(business_license)
    WHERE deleted_at IS NULL;

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

DROP TRIGGER IF EXISTS trg_users_sync_soft_delete_dependents ON users;

CREATE TRIGGER trg_users_sync_soft_delete_dependents
AFTER UPDATE OF deleted_at ON users
FOR EACH ROW
WHEN (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
EXECUTE FUNCTION sync_user_soft_delete_dependents();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_users_sync_soft_delete_dependents ON users;

DROP FUNCTION IF EXISTS sync_user_soft_delete_dependents();

DROP INDEX IF EXISTS idx_users_email_active;

DROP INDEX IF EXISTS idx_auth_provider_provider_user_id_active;

DROP INDEX IF EXISTS idx_merchant_profiles_business_license_active;

ALTER TABLE users
    ADD CONSTRAINT users_email_key UNIQUE (email);

CREATE UNIQUE INDEX idx_auth_provider_provider_user_id
    ON user_authentications(provider, provider_user_id);

ALTER TABLE merchant_profiles
    ADD CONSTRAINT merchant_profiles_business_license_key UNIQUE (business_license);

DROP INDEX IF EXISTS idx_user_authentications_deleted_at;

DROP INDEX IF EXISTS idx_merchant_profiles_deleted_at;

ALTER TABLE user_authentications
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE merchant_profiles
    DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
