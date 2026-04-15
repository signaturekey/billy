package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

type accountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *accountRepository {
	return &accountRepository{pool: pool}
}

func (repo *accountRepository) Create(ctx context.Context, account entity.Account) (entity.Account, error) {
	const query = `
		INSERT INTO accounts (
			user_id,
			currency,
			balance,
			reserved_amount,
			status
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			user_id,
			currency,
			balance,
			reserved_amount,
			status,
			created_at,
			updated_at
	`

	rows, err := repo.pool.Query(
		ctx,
		query,
		account.UserID,
		account.Currency,
		account.Balance,
		account.ReservedAmount,
		account.Status,
	)
	if err != nil {
		if errors.Is(mapPgError(err), ErrDuplicate) {
			return entity.Account{}, domainerrors.ErrAccountAlreadyExists
		}
		return entity.Account{}, fmt.Errorf("insert account: %w", err)
	}
	defer rows.Close()

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.Account])
	if err != nil {
		if errors.Is(mapPgError(err), ErrDuplicate) {
			return entity.Account{}, domainerrors.ErrAccountAlreadyExists
		}
		return entity.Account{}, fmt.Errorf("collect inserted account: %w", err)
	}

	return created, nil
}

func (repo *accountRepository) GetByID(ctx context.Context, id int64) (entity.Account, error) {
	const query = `
		SELECT
			id,
			user_id,
			currency,
			balance,
			reserved_amount,
			status,
			created_at,
			updated_at
		FROM accounts
		WHERE id = $1
	`

	rows, err := repo.pool.Query(ctx, query, id)
	if err != nil {
		return entity.Account{}, fmt.Errorf("query account by id: %w", err)
	}
	defer rows.Close()

	account, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Account{}, domainerrors.ErrAccountNotFound
		}
		return entity.Account{}, fmt.Errorf("collect account by id: %w", err)
	}

	return account, nil
}

func (repo *accountRepository) TopUp(ctx context.Context, accountID int64, amount int64) (entity.LedgerEntry, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return entity.LedgerEntry{}, fmt.Errorf("begin topup transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const updateAccountQuery = `
		UPDATE accounts
		SET
			balance = balance + $2,
			updated_at = now()
		WHERE id = $1
		RETURNING
			currency,
			balance
	`

	var currency string
	var balanceAfter int64
	if err := tx.QueryRow(ctx, updateAccountQuery, accountID, amount).Scan(&currency, &balanceAfter); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.LedgerEntry{}, domainerrors.ErrAccountNotFound
		}
		return entity.LedgerEntry{}, fmt.Errorf("update account balance: %w", err)
	}

	const insertLedgerEntryQuery = `
		INSERT INTO ledger_entries (
			account_id,
			type,
			amount,
			currency,
			balance_after
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			account_id,
			type,
			amount,
			currency,
			balance_after,
			created_at
	`

	rows, err := tx.Query(
		ctx,
		insertLedgerEntryQuery,
		accountID,
		entity.LedgerEntryTypeTopup,
		amount,
		currency,
		balanceAfter,
	)
	if err != nil {
		return entity.LedgerEntry{}, fmt.Errorf("insert ledger entry: %w", err)
	}
	defer rows.Close()

	entry, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.LedgerEntry])
	if err != nil {
		return entity.LedgerEntry{}, fmt.Errorf("collect inserted ledger entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.LedgerEntry{}, fmt.Errorf("commit topup transaction: %w", err)
	}

	return entry, nil
}
