-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- PostgreSQL 18 UUID v7 Upgrade
-- UUIDv7 provides time-ordered UUIDs for significantly improved index and query performance

-- Step 1: Create new UUIDv7 function as default
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS UUID AS $$
BEGIN
    -- Use PostgreSQL 18 native UUIDv7 function
    RETURN uuidv7();
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- Step 2: Create alias function for backward compatibility
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_generate_v4()
RETURNS UUID AS $$
BEGIN
    -- Maintain backward compatibility, but recommend uuid_generate_v7() for new tables
    RETURN gen_random_uuid();
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- Step 3: Add performance monitoring function
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_version(input_uuid UUID)
RETURNS INTEGER AS $$
BEGIN
    -- Extract UUID version number for performance analysis
    RETURN (get_byte(input_uuid::bytea, 6) >> 4);
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- Step 4: Add utility function to extract timestamp from UUIDv7
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_v7_timestamp(input_uuid UUID)
RETURNS TIMESTAMPTZ AS $$
DECLARE
    uuid_bytes BYTEA;
    timestamp_ms BIGINT;
BEGIN
    -- Only applicable to UUIDv7, extract timestamp from UUID
    IF uuid_version(input_uuid) != 7 THEN
        RETURN NULL;
    END IF;
    
    uuid_bytes := input_uuid::bytea;
    
    -- Extract first 48 bits as millisecond timestamp
    timestamp_ms := (get_byte(uuid_bytes, 0)::bigint << 40) |
                   (get_byte(uuid_bytes, 1)::bigint << 32) |
                   (get_byte(uuid_bytes, 2)::bigint << 24) |
                   (get_byte(uuid_bytes, 3)::bigint << 16) |
                   (get_byte(uuid_bytes, 4)::bigint << 8) |
                   get_byte(uuid_bytes, 5)::bigint;
    
    -- Convert to PostgreSQL timestamp
    RETURN to_timestamp(timestamp_ms / 1000.0);
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- Step 5: Add comments recommending UUIDv7 for future tables
COMMENT ON FUNCTION uuid_generate_v7() IS 'PostgreSQL 18 UUIDv7 - Time-ordered UUIDs providing better index performance. Recommended for new tables.';
COMMENT ON FUNCTION uuid_generate_v4() IS 'Backward compatible UUID v4 function. New development should consider using uuid_generate_v7().';
COMMENT ON FUNCTION uuid_version(UUID) IS 'Extract UUID version number for performance monitoring and analysis.';
COMMENT ON FUNCTION uuid_v7_timestamp(UUID) IS 'Extract creation timestamp from UUIDv7.';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Remove newly added functions
DROP FUNCTION IF EXISTS uuid_v7_timestamp(UUID);
DROP FUNCTION IF EXISTS uuid_version(UUID);
DROP FUNCTION IF EXISTS uuid_generate_v7();

-- Restore original uuid_generate_v4 function
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION uuid_generate_v4()
RETURNS UUID AS $$
DECLARE
    result UUID;
BEGIN
    SELECT gen_random_uuid() INTO result;
    RETURN result;
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd