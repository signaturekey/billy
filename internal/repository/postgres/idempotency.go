package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type idempotencyRepository struct{}

func NewIdempotencyRepository() *idempotencyRepository {
	return &idempotencyRepository{}
}

func (repo *idempotencyRepository) CreateProcessing(
	ctx context.Context,
	tx pgx.Tx,
	record entity.IdempotencyKey,
) error {
	const query = `
		INSERT INTO idempotency_keys (
			key,
			operation_type,
			request_hash,
			status,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key, operation_type) DO NOTHING
		RETURNING key
	`

	var key string
	if err := tx.QueryRow(
		ctx,
		query,
		record.Key,
		record.OperationType,
		record.RequestHash,
		entity.IdempotencyStatusProcessing,
		record.ExpiresAt,
	).Scan(&key); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrIdempotencyKeyExists
		}
		return fmt.Errorf("insert idempotency key: %w", err)
	}

	return nil
}

func (repo *idempotencyRepository) GetByKey(
	ctx context.Context,
	tx pgx.Tx,
	key string,
	operationType string,
) (entity.IdempotencyKey, error) {
	const query = `
		SELECT
			key,
			operation_type,
			request_hash,
			status,
			COALESCE(response_code, 0) AS response_code,
			COALESCE(response_body, '{}') AS response_body,
			created_at,
			updated_at,
			expires_at
		FROM idempotency_keys
		WHERE key = $1
			AND operation_type = $2
	`

	rows, err := tx.Query(ctx, query, key, operationType)
	if err != nil {
		return entity.IdempotencyKey{}, fmt.Errorf("query idempotency key: %w", err)
	}
	defer rows.Close()

	record, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.IdempotencyKey])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.IdempotencyKey{}, domainerrors.ErrIdempotencyNotFound
		}
		return entity.IdempotencyKey{}, fmt.Errorf("collect idempotency key: %w", err)
	}

	return record, nil
}

func (repo *idempotencyRepository) MarkCompleted(
	ctx context.Context,
	tx pgx.Tx,
	key string,
	operationType string,
	responseCode int,
	responseBody []byte,
) error {
	const query = `
		UPDATE idempotency_keys
		SET
			status = $3,
			response_code = $4,
			response_body = $5,
			updated_at = now()
		WHERE key = $1
			AND operation_type = $2
	`

	if _, err := tx.Exec(
		ctx,
		query,
		key,
		operationType,
		entity.IdempotencyStatusCompleted,
		responseCode,
		string(responseBody),
	); err != nil {
		return fmt.Errorf("mark idempotency key completed: %w", err)
	}

	return nil
}
