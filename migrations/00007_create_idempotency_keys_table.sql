-- +goose Up
-- +goose StatementBegin
CREATE TABLE idempotency_keys (
    key TEXT NOT NULL,
    operation_type TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    response_code INT,
    response_body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (key, operation_type),
    CONSTRAINT idempotency_keys_status_allowed CHECK (status IN ('processing', 'completed')),
    CONSTRAINT idempotency_keys_completed_response_required CHECK (
        status <> 'completed'
        OR (response_code IS NOT NULL AND response_body IS NOT NULL)
    )
);

CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS idempotency_keys;
-- +goose StatementEnd
