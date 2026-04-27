-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_devices
ADD COLUMN token_refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX idx_user_devices_push_fanout
ON user_devices (user_id, is_active, token_refreshed_at)
WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_devices_push_fanout;
ALTER TABLE user_devices DROP COLUMN IF EXISTS token_refreshed_at;
-- +goose StatementEnd
