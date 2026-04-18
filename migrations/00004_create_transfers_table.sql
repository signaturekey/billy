-- +goose Up
-- +goose StatementBegin
CREATE TABLE transfers (
    id BIGSERIAL PRIMARY KEY,
    from_account_id BIGINT NOT NULL REFERENCES accounts (id),
    to_account_id BIGINT NOT NULL REFERENCES accounts (id),
    amount BIGINT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transfers_different_accounts CHECK (from_account_id <> to_account_id),
    CONSTRAINT transfers_amount_positive CHECK (amount > 0),
    CONSTRAINT transfers_status_allowed CHECK (status IN ('completed'))
);

ALTER TABLE ledger_entries
    ADD COLUMN reference_type TEXT,
    ADD COLUMN reference_id BIGINT;

ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (
        type IN ('topup', 'withdrawal', 'transfer_in', 'transfer_out')
    );

CREATE INDEX transfers_from_account_id_idx ON transfers (from_account_id);
CREATE INDEX transfers_to_account_id_idx ON transfers (to_account_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS transfers_to_account_id_idx;
DROP INDEX IF EXISTS transfers_from_account_id_idx;

ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (type IN ('topup', 'withdrawal'));

ALTER TABLE ledger_entries
    DROP COLUMN reference_id,
    DROP COLUMN reference_type;

DROP TABLE IF EXISTS transfers;
-- +goose StatementEnd
