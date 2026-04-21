package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type holdRepository struct {
	pool *pgxpool.Pool
}

func NewHoldRepository(pool *pgxpool.Pool) *holdRepository {
	return &holdRepository{pool: pool}
}

func (repo *holdRepository) ListExpiredPending(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]entity.Hold, error) {
	const query = `
		SELECT
			id,
			account_id,
			amount,
			status,
			expires_at,
			created_at,
			updated_at
		FROM holds
		WHERE status = $1
			AND expires_at < $2
		ORDER BY expires_at, id
		LIMIT $3
	`

	rows, err := repo.pool.Query(ctx, query, entity.HoldStatusPending, now, limit)
	if err != nil {
		return nil, fmt.Errorf("query expired pending holds: %w", err)
	}
	defer rows.Close()

	holds, err := pgx.CollectRows(rows, pgx.RowToStructByName[entity.Hold])
	if err != nil {
		return nil, fmt.Errorf("collect expired pending holds: %w", err)
	}

	return holds, nil
}

func (repo *holdRepository) Create(ctx context.Context, tx pgx.Tx, hold entity.Hold) (entity.Hold, error) {
	const query = `
		INSERT INTO holds (
			account_id,
			amount,
			status,
			expires_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			account_id,
			amount,
			status,
			expires_at,
			created_at,
			updated_at
	`

	rows, err := tx.Query(ctx, query, hold.AccountID, hold.Amount, hold.Status, hold.ExpiresAt)
	if err != nil {
		return entity.Hold{}, fmt.Errorf("insert hold: %w", err)
	}
	defer rows.Close()

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.Hold])
	if err != nil {
		return entity.Hold{}, fmt.Errorf("collect inserted hold: %w", err)
	}

	return created, nil
}

func (repo *holdRepository) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id int64) (entity.Hold, error) {
	const query = `
		SELECT
			id,
			account_id,
			amount,
			status,
			expires_at,
			created_at,
			updated_at
		FROM holds
		WHERE id = $1
		FOR UPDATE
	`

	rows, err := tx.Query(ctx, query, id)
	if err != nil {
		return entity.Hold{}, fmt.Errorf("query hold for update: %w", err)
	}
	defer rows.Close()

	hold, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.Hold])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Hold{}, domainerrors.ErrHoldNotFound
		}
		return entity.Hold{}, fmt.Errorf("collect hold for update: %w", err)
	}

	return hold, nil
}

func (repo *holdRepository) UpdateStatus(
	ctx context.Context,
	tx pgx.Tx,
	id int64,
	status entity.HoldStatus,
) (entity.Hold, error) {
	const query = `
		UPDATE holds
		SET
			status = $2,
			updated_at = now()
		WHERE id = $1
		RETURNING
			id,
			account_id,
			amount,
			status,
			expires_at,
			created_at,
			updated_at
	`

	rows, err := tx.Query(ctx, query, id, status)
	if err != nil {
		return entity.Hold{}, fmt.Errorf("update hold status: %w", err)
	}
	defer rows.Close()

	hold, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.Hold])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Hold{}, domainerrors.ErrHoldNotFound
		}
		return entity.Hold{}, fmt.Errorf("collect updated hold: %w", err)
	}

	return hold, nil
}
