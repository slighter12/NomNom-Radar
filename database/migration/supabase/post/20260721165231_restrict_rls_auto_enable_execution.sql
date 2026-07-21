-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Restrict the optional Supabase rls_auto_enable() helper reported by Security
-- Advisor lints 0028 and 0029. Environments without the helper are skipped.
-- +goose StatementBegin
DO $$
BEGIN
    IF pg_catalog.to_regprocedure('public.rls_auto_enable()') IS NULL THEN
        RETURN;
    END IF;

    REVOKE EXECUTE ON FUNCTION public.rls_auto_enable() FROM PUBLIC, anon, authenticated;

    IF pg_catalog.has_function_privilege('anon', 'public.rls_auto_enable()', 'EXECUTE')
        OR pg_catalog.has_function_privilege('authenticated', 'public.rls_auto_enable()', 'EXECUTE') THEN
        RAISE EXCEPTION
            'failed to revoke EXECUTE on public.rls_auto_enable(); migration role % must own the function or hold the required grant option',
            CURRENT_USER;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Intentionally no-op. Restoring public execution on a SECURITY DEFINER
-- function would reintroduce the exposure this migration removes.
