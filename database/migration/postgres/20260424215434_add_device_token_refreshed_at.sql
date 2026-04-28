-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_devices
ADD COLUMN token_refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DROP INDEX IF EXISTS idx_user_devices_user_active;

CREATE INDEX idx_user_devices_push_fanout
ON user_devices (user_id, is_active, token_refreshed_at)
WHERE deleted_at IS NULL;

CREATE INDEX idx_user_devices_stale_cleanup
ON user_devices (token_refreshed_at)
WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_devices_stale_cleanup;
DROP INDEX IF EXISTS idx_user_devices_push_fanout;
CREATE INDEX idx_user_devices_user_active
ON user_devices (user_id, is_active)
WHERE deleted_at IS NULL;
ALTER TABLE user_devices DROP COLUMN IF EXISTS token_refreshed_at;
-- +goose StatementEnd
