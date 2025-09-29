-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Safe UUIDv7 migration strategy
-- Only affects default values for new records, doesn't touch existing data

-- ⚠️ Important Notes:
-- 1. This migration will not change existing UUID values
-- 2. Only affects future inserted records
-- 3. Existing foreign key relationships remain completely unaffected
-- 4. Safe to execute in production environment

-- Step 1: Update main table default values to UUIDv7
-- Impact: New users will get time-ordered UUIDs
ALTER TABLE users ALTER COLUMN id SET DEFAULT uuid_generate_v7();
COMMENT ON COLUMN users.id IS 'Primary key using UUIDv7 for new records (existing records remain UUIDv4)';

-- Step 2: Update child table default values to UUIDv7
-- Impact: New authentication records, refresh tokens, addresses will use UUIDv7
ALTER TABLE user_authentications ALTER COLUMN id SET DEFAULT uuid_generate_v7();
COMMENT ON COLUMN user_authentications.id IS 'Primary key using UUIDv7 for new records';

ALTER TABLE refresh_tokens ALTER COLUMN id SET DEFAULT uuid_generate_v7();
COMMENT ON COLUMN refresh_tokens.id IS 'Primary key using UUIDv7 for new records';

ALTER TABLE addresses ALTER COLUMN id SET DEFAULT uuid_generate_v7();
COMMENT ON COLUMN addresses.id IS 'Primary key using UUIDv7 for new records';

-- Step 3: Create monitoring view to track UUID version distribution
CREATE OR REPLACE VIEW uuid_version_stats AS
SELECT 
    'users' as table_name,
    uuid_version(id) as uuid_version,
    COUNT(*) as record_count,
    MIN(created_at) as earliest_record,
    MAX(created_at) as latest_record
FROM users 
GROUP BY uuid_version(id)

UNION ALL

SELECT 
    'user_authentications' as table_name,
    uuid_version(id) as uuid_version,
    COUNT(*) as record_count,
    MIN(created_at) as earliest_record,
    MAX(created_at) as latest_record
FROM user_authentications 
GROUP BY uuid_version(id)

UNION ALL

SELECT 
    'refresh_tokens' as table_name,
    uuid_version(id) as uuid_version,
    COUNT(*) as record_count,
    MIN(created_at) as earliest_record,
    MAX(created_at) as latest_record
FROM refresh_tokens 
GROUP BY uuid_version(id)

UNION ALL

SELECT 
    'addresses' as table_name,
    uuid_version(id) as uuid_version,
    COUNT(*) as record_count,
    MIN(created_at) as earliest_record,
    MAX(created_at) as latest_record
FROM addresses 
GROUP BY uuid_version(id)

ORDER BY table_name, uuid_version;

COMMENT ON VIEW uuid_version_stats IS 'Monitor UUID version distribution across tables';

-- Step 4: Create simple monitoring queries (functions prone to errors here, skip for now)
-- You can manually execute the following query to check progress:
-- SELECT 'users' as table_name, uuid_version(id) as version, COUNT(*) FROM users GROUP BY uuid_version(id);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

-- Restore original default values
ALTER TABLE users ALTER COLUMN id SET DEFAULT uuid_generate_v4();
ALTER TABLE user_authentications ALTER COLUMN id SET DEFAULT uuid_generate_v4();
ALTER TABLE refresh_tokens ALTER COLUMN id SET DEFAULT uuid_generate_v4();
ALTER TABLE addresses ALTER COLUMN id SET DEFAULT uuid_generate_v4();

-- Remove monitoring tools
DROP VIEW IF EXISTS uuid_version_stats;

-- Remove comments
COMMENT ON COLUMN users.id IS NULL;
COMMENT ON COLUMN user_authentications.id IS NULL;
COMMENT ON COLUMN refresh_tokens.id IS NULL;
COMMENT ON COLUMN addresses.id IS NULL;