-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE SCHEMA IF NOT EXISTS extensions;

CREATE EXTENSION IF NOT EXISTS citext SCHEMA extensions;
CREATE EXTENSION IF NOT EXISTS postgis SCHEMA extensions;

-- +goose StatementBegin
DO $$
BEGIN
    IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
        EXECUTE $compat$
            CREATE OR REPLACE FUNCTION public.uuidv7()
            RETURNS UUID
            LANGUAGE sql
            VOLATILE
            SET search_path = ''
            AS $function$
            WITH uuid_parts AS (
                SELECT
                    pg_catalog.lpad(
                        pg_catalog.to_hex(
                            pg_catalog.floor(EXTRACT(EPOCH FROM pg_catalog.clock_timestamp()) * 1000)::bigint
                        ),
                        12,
                        '0'
                    ) AS unix_ts_ms,
                    pg_catalog.replace(pg_catalog.gen_random_uuid()::text, '-', '') AS random_hex
            )
            SELECT (
                pg_catalog.substr(unix_ts_ms, 1, 8) || '-' ||
                pg_catalog.substr(unix_ts_ms, 9, 4) || '-' ||
                '7' || pg_catalog.substr(random_hex, 1, 3) || '-' ||
                pg_catalog.substr(
                    '89ab',
                    (pg_catalog.get_byte(pg_catalog.decode(pg_catalog.substr(random_hex, 4, 2), 'hex'), 0) % 4) + 1,
                    1
                ) ||
                pg_catalog.substr(random_hex, 6, 3) || '-' ||
                pg_catalog.substr(random_hex, 9, 12)
            )::uuid
            FROM uuid_parts;
            $function$
        $compat$;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    EXECUTE format('ALTER ROLE %I SET search_path = "$user", public, extensions', CURRENT_USER);

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
