-- +goose Up
-- +goose StatementBegin
CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    reserved_amount BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0),
    CONSTRAINT accounts_reserved_amount_non_negative CHECK (reserved_amount >= 0),
    CONSTRAINT accounts_reserved_amount_lte_balance CHECK (reserved_amount <= balance),
    CONSTRAINT accounts_currency_length CHECK (char_length(currency) = 3)
);

CREATE INDEX accounts_user_id_idx ON accounts (user_id);

CREATE UNIQUE INDEX accounts_user_id_currency_uidx ON accounts (user_id, currency);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS accounts;
-- +goose StatementEnd
