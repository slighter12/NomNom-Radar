-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE SCHEMA IF NOT EXISTS extensions;

CREATE EXTENSION IF NOT EXISTS citext SCHEMA extensions;
CREATE EXTENSION IF NOT EXISTS postgis SCHEMA extensions;

-- +goose StatementBegin
DO $$
BEGIN
    EXECUTE format('ALTER ROLE %I SET search_path = public, extensions', CURRENT_USER);

    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'postgres') THEN
        GRANT USAGE ON SCHEMA extensions TO postgres;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'anon') THEN
        GRANT USAGE ON SCHEMA extensions TO anon;
        ALTER ROLE anon SET search_path = "$user", public, extensions;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'authenticated') THEN
        GRANT USAGE ON SCHEMA extensions TO authenticated;
        ALTER ROLE authenticated SET search_path = "$user", public, extensions;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'service_role') THEN
        GRANT USAGE ON SCHEMA extensions TO service_role;
        ALTER ROLE service_role SET search_path = "$user", public, extensions;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Intentionally no-op. Extension placement and role search_path are environment
-- bootstrap state and should not be destroyed by migration rollback.
