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

func (repo *accountRepository) GetForUpdate(ctx context.Context, tx pgx.Tx, id int64) (entity.Account, error) {
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
		FOR UPDATE
	`

	rows, err := tx.Query(ctx, query, id)
	if err != nil {
		return entity.Account{}, fmt.Errorf("query account for update: %w", err)
	}
	defer rows.Close()

	account, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entity.Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Account{}, domainerrors.ErrAccountNotFound
		}
		return entity.Account{}, fmt.Errorf("collect account for update: %w", err)
	}

	return account, nil
}

func (repo *accountRepository) UpdateBalance(ctx context.Context, tx pgx.Tx, accountID int64, balance int64) error {
	const query = `
		UPDATE accounts
		SET
			balance = $2,
			updated_at = now()
		WHERE id = $1
	`

	if _, err := tx.Exec(ctx, query, accountID, balance); err != nil {
		return fmt.Errorf("update account balance: %w", err)
	}

	return nil
}

func (repo *accountRepository) UpdateAmounts(
	ctx context.Context,
	tx pgx.Tx,
	accountID int64,
	balance int64,
	reservedAmount int64,
) error {
	const query = `
		UPDATE accounts
		SET
			balance = $2,
			reserved_amount = $3,
			updated_at = now()
		WHERE id = $1
	`

	if _, err := tx.Exec(ctx, query, accountID, balance, reservedAmount); err != nil {
		return fmt.Errorf("update account amounts: %w", err)
	}

	return nil
}
