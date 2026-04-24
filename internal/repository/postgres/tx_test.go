package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
)

func TestTxManagerCommitsAndRollsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	manager := NewTxManager(pool)
	accounts := NewAccountRepository(pool)

	err := manager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO accounts (user_id, currency, balance, reserved_amount, status)
			VALUES ($1, $2, $3, $4, $5)
		`, int64(10), "USD", int64(100), int64(0), entity.AccountStatusActive)
		return err
	})
	require.NoError(t, err)

	committed, err := accounts.GetByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(10), committed.UserID)

	rollbackErr := errors.New("rollback")
	err = manager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, `
			INSERT INTO accounts (user_id, currency, balance, reserved_amount, status)
			VALUES ($1, $2, $3, $4, $5)
		`, int64(20), "EUR", int64(100), int64(0), entity.AccountStatusActive)
		require.NoError(t, execErr)
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&count))
	assert.Equal(t, 1, count)
}
