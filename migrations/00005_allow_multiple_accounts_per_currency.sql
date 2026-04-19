-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS accounts_user_id_currency_uidx;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE UNIQUE INDEX accounts_user_id_currency_uidx ON accounts (user_id, currency);
-- +goose StatementEnd
