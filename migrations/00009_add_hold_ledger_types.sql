-- +goose Up
-- +goose StatementBegin
ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (
        type IN (
            'topup',
            'withdrawal',
            'transfer_in',
            'transfer_out',
            'hold_created',
            'hold_confirmed',
            'hold_cancelled',
            'hold_expired'
        )
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ledger_entries
    DROP CONSTRAINT ledger_entries_type_allowed,
    ADD CONSTRAINT ledger_entries_type_allowed CHECK (
        type IN ('topup', 'withdrawal', 'transfer_in', 'transfer_out', 'hold_confirmed')
    );
-- +goose StatementEnd
