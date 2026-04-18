package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/signaturekey/billy/internal/domain/entity"
)

type ledgerRepository struct {
	pool *pgxpool.Pool
}

func NewLedgerRepository(pool *pgxpool.Pool) *ledgerRepository {
	return &ledgerRepository{pool: pool}
}

func (repo *ledgerRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	entry entity.LedgerEntry,
) (entity.LedgerEntry, error) {
	const query = `
		INSERT INTO ledger_entries (
			account_id,
			type,
			amount,
			currency,
			balance_before,
			balance_after,
			reference_type,
			reference_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING
			id,
			account_id,
			type,
			amount,
			currency,
			balance_before,
			balance_after,
			created_at
	`

	var referenceType any
	if entry.ReferenceType != "" {
		referenceType = entry.ReferenceType
	}

	var referenceID any
	if entry.ReferenceID != 0 {
		referenceID = entry.ReferenceID
	}

	var created entity.LedgerEntry
	if err := tx.QueryRow(
		ctx,
		query,
		entry.AccountID,
		entry.Type,
		entry.Amount,
		entry.Currency,
		entry.BalanceBefore,
		entry.BalanceAfter,
		referenceType,
		referenceID,
	).Scan(
		&created.ID,
		&created.AccountID,
		&created.Type,
		&created.Amount,
		&created.Currency,
		&created.BalanceBefore,
		&created.BalanceAfter,
		&created.CreatedAt,
	); err != nil {
		return entity.LedgerEntry{}, fmt.Errorf("insert ledger entry: %w", err)
	}

	created.ReferenceType = entry.ReferenceType
	created.ReferenceID = entry.ReferenceID

	return created, nil
}

func (repo *ledgerRepository) ListByAccount(
	ctx context.Context,
	accountID int64,
	limit int,
	offset int,
) ([]entity.LedgerEntry, error) {
	const query = `
		SELECT
			id,
			account_id,
			type,
			amount,
			currency,
			balance_before,
			balance_after,
			created_at
		FROM ledger_entries
		WHERE account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := repo.pool.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query account operations: %w", err)
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, pgx.RowToStructByName[entity.LedgerEntry])
	if err != nil {
		return nil, fmt.Errorf("collect account operations: %w", err)
	}

	return entries, nil
}
