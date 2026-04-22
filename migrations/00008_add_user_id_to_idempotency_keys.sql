-- +goose Up
-- +goose StatementBegin
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
ALTER TABLE idempotency_keys ADD COLUMN user_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE idempotency_keys ALTER COLUMN user_id DROP DEFAULT;
ALTER TABLE idempotency_keys ADD PRIMARY KEY (user_id, key, operation_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
ALTER TABLE idempotency_keys DROP COLUMN user_id;
ALTER TABLE idempotency_keys ADD PRIMARY KEY (key, operation_type);
-- +goose StatementEnd
