-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- +goose StatementBegin
DO $$
DECLARE
    missing_object TEXT;
BEGIN
    SELECT object_name
    INTO missing_object
    FROM (
        VALUES
            ('schema extensions', to_regnamespace('extensions') IS NOT NULL),
            ('table public.users', to_regclass('public.users') IS NOT NULL),
            ('table public.user_profiles', to_regclass('public.user_profiles') IS NOT NULL),
            ('table public.merchant_profiles', to_regclass('public.merchant_profiles') IS NOT NULL),
            ('table public.user_authentications', to_regclass('public.user_authentications') IS NOT NULL),
            ('table public.refresh_tokens', to_regclass('public.refresh_tokens') IS NOT NULL),
            ('table public.addresses', to_regclass('public.addresses') IS NOT NULL),
            ('table public.menu_items', to_regclass('public.menu_items') IS NOT NULL),
            ('table public.login_attempts', to_regclass('public.login_attempts') IS NOT NULL),
            ('table public.user_devices', to_regclass('public.user_devices') IS NOT NULL),
            ('table public.user_merchant_subscriptions', to_regclass('public.user_merchant_subscriptions') IS NOT NULL),
            ('table public.merchant_location_notifications', to_regclass('public.merchant_location_notifications') IS NOT NULL),
            ('table public.notification_logs', to_regclass('public.notification_logs') IS NOT NULL),
            ('table public.discovery_categories', to_regclass('public.discovery_categories') IS NOT NULL),
            ('table public.discovery_subcategories', to_regclass('public.discovery_subcategories') IS NOT NULL),
            ('table public.hubs', to_regclass('public.hubs') IS NOT NULL),
            ('function public.update_updated_at_column()', to_regprocedure('public.update_updated_at_column()') IS NOT NULL),
            ('function public.uuid_generate_v7()', to_regprocedure('public.uuid_generate_v7()') IS NOT NULL),
            ('function public.uuid_generate_v4()', to_regprocedure('public.uuid_generate_v4()') IS NOT NULL),
            ('function public.update_location_from_lat_lng()', to_regprocedure('public.update_location_from_lat_lng()') IS NOT NULL),
            ('function public.sync_user_soft_delete_dependents()', to_regprocedure('public.sync_user_soft_delete_dependents()') IS NOT NULL)
    ) AS required_objects(object_name, object_exists)
    WHERE NOT object_exists
    LIMIT 1;

    IF missing_object IS NOT NULL THEN
        RAISE EXCEPTION 'shared PostgreSQL migrations and Supabase pre-migrations must be applied before Supabase function hardening; missing %', missing_object;
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
