-- +goose Up
-- +goose StatementBegin
CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    attempt_key TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    failed_count INT NOT NULL DEFAULT 0,
    lockout_count INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_failed_at TIMESTAMPTZ,
    last_lockout_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_login_attempts_attempt_key ON login_attempts(attempt_key);
CREATE INDEX idx_login_attempts_last_lockout_at
    ON login_attempts(last_lockout_at)
    WHERE last_lockout_at IS NOT NULL;

CREATE TRIGGER update_login_attempts_updated_at
    BEFORE UPDATE ON login_attempts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_login_attempts_updated_at ON login_attempts;
DROP TABLE IF EXISTS login_attempts;
-- +goose StatementEnd
