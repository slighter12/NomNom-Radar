-- +goose Up
-- +goose StatementBegin
ALTER TABLE refresh_tokens ADD COLUMN family_id UUID;
UPDATE refresh_tokens SET family_id = uuid_generate_v7() WHERE family_id IS NULL;
ALTER TABLE refresh_tokens ALTER COLUMN family_id SET NOT NULL;
ALTER TABLE refresh_tokens ADD COLUMN is_revoked BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE refresh_tokens ADD COLUMN replaced_by UUID REFERENCES refresh_tokens(id);
CREATE INDEX idx_refresh_tokens_family_id ON refresh_tokens(family_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_refresh_tokens_family_id;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS replaced_by;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS is_revoked;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS family_id;
-- +goose StatementEnd
