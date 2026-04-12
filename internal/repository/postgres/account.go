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
