-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- +goose StatementBegin
DO $$
BEGIN
    IF to_regclass('public.users') IS NULL
        OR to_regclass('public.user_authentications') IS NULL
        OR to_regclass('public.merchant_profiles') IS NULL
        OR to_regclass('public.addresses') IS NULL
        OR to_regnamespace('extensions') IS NULL
        OR to_regprocedure('public.update_updated_at_column()') IS NULL
        OR to_regprocedure('public.uuid_generate_v7()') IS NULL
        OR to_regprocedure('public.uuid_generate_v4()') IS NULL
        OR to_regprocedure('public.update_location_from_lat_lng()') IS NULL
        OR to_regprocedure('public.sync_user_soft_delete_dependents()') IS NULL
    ) THEN
        RAISE EXCEPTION 'shared PostgreSQL migrations and Supabase pre-migrations must be applied before Supabase function hardening';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = pg_catalog.now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.uuid_generate_v7()
RETURNS UUID AS $$
BEGIN
    RETURN pg_catalog.uuidv7();
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.uuid_generate_v4()
RETURNS UUID AS $$
BEGIN
    RETURN pg_catalog.gen_random_uuid();
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.update_location_from_lat_lng()
RETURNS TRIGGER AS $$
BEGIN
    NEW.location = extensions.ST_SetSRID(
        extensions.ST_MakePoint(NEW.longitude, NEW.latitude),
        4326
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.sync_user_soft_delete_dependents()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE public.user_authentications
    SET deleted_at = NEW.deleted_at
    WHERE user_id = NEW.id
      AND deleted_at IS DISTINCT FROM NEW.deleted_at;

    UPDATE public.merchant_profiles
    SET deleted_at = NEW.deleted_at
    WHERE user_id = NEW.id
      AND deleted_at IS DISTINCT FROM NEW.deleted_at;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

ALTER FUNCTION public.update_updated_at_column() SET search_path = '';
ALTER FUNCTION public.sync_user_soft_delete_dependents() SET search_path = '';
ALTER FUNCTION public.uuid_generate_v4() SET search_path = '';
ALTER FUNCTION public.uuid_generate_v7() SET search_path = '';
ALTER FUNCTION public.update_location_from_lat_lng() SET search_path = '';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Intentionally no-op. Function hardening is a forward-only Supabase security
-- posture; rolling it back would reintroduce unsafe search_path behavior.
