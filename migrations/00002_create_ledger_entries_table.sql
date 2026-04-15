-- +goose Up
-- +goose StatementBegin
CREATE TABLE ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id),
    type TEXT NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    balance_after BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ledger_entries_type_allowed CHECK (type IN ('topup')),
    CONSTRAINT ledger_entries_amount_positive CHECK (amount > 0),
    CONSTRAINT ledger_entries_balance_after_non_negative CHECK (balance_after >= 0),
    CONSTRAINT ledger_entries_currency_length CHECK (char_length(currency) = 3)
);

CREATE INDEX ledger_entries_account_id_idx ON ledger_entries (account_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ledger_entries;
-- +goose StatementEnd
