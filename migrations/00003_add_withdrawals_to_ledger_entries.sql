-- +goose Up
-- +goose StatementBegin
ALTER TABLE ledger_entries
    ADD COLUMN balance_before BIGINT;

UPDATE ledger_entries
SET balance_before = balance_after - amount
WHERE type = 'topup';

ALTER TABLE ledger_entries
    ALTER COLUMN balance_before SET NOT NULL,
    ADD CONSTRAINT ledger_entries_balance_before_non_negative CHECK (balance_before >= 0);

ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (type IN ('topup', 'withdrawal'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (type IN ('topup')) NOT VALID;

ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_balance_before_non_negative,
    DROP COLUMN balance_before;
-- +goose StatementEnd
