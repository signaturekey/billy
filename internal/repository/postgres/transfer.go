package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
)

type transferRepository struct{}

func NewTransferRepository() *transferRepository {
	return &transferRepository{}
}

func (repo *transferRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	transfer entity.Transfer,
) (entity.Transfer, error) {
	const query = `
		INSERT INTO transfers (
			from_account_id,
			to_account_id,
			amount,
			status
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			from_account_id,
			to_account_id,
			amount,
			status,
			created_at
	`

	var created entity.Transfer
	if err := tx.QueryRow(
		ctx,
		query,
		transfer.FromAccountID,
		transfer.ToAccountID,
		transfer.Amount,
		transfer.Status,
	).Scan(
		&created.ID,
		&created.FromAccountID,
		&created.ToAccountID,
		&created.Amount,
		&created.Status,
		&created.CreatedAt,
	); err != nil {
		return entity.Transfer{}, fmt.Errorf("insert transfer: %w", err)
	}

	return created, nil
}
