-- +goose Up
-- +goose StatementBegin
CREATE TABLE holds (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id),
    amount BIGINT NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT holds_amount_positive CHECK (amount > 0),
    CONSTRAINT holds_status_allowed CHECK (status IN ('pending', 'confirmed', 'cancelled', 'expired'))
);

ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (
        type IN ('topup', 'withdrawal', 'transfer_in', 'transfer_out', 'hold_confirmed')
    );

CREATE INDEX holds_account_id_idx ON holds (account_id);
CREATE INDEX holds_status_expires_at_idx ON holds (status, expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS holds_status_expires_at_idx;
DROP INDEX IF EXISTS holds_account_id_idx;

ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (
        type IN ('topup', 'withdrawal', 'transfer_in', 'transfer_out')
    );

DROP TABLE IF EXISTS holds;
-- +goose StatementEnd
