package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
)

func TestTransferRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	accounts := NewAccountRepository(pool)
	transfers := NewTransferRepository()

	from := createIntegrationAccount(t, accounts, 10, "USD", 100, 0)
	to := createIntegrationAccount(t, accounts, 20, "USD", 50, 0)

	tx := beginIntegrationTx(t, pool)
	created, err := transfers.Create(ctx, tx, entity.Transfer{
		FromAccountID: from.ID,
		ToAccountID:   to.ID,
		Amount:        25,
		Status:        entity.TransferStatusCompleted,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	assert.NotZero(t, created.ID)
	assert.Equal(t, from.ID, created.FromAccountID)
	assert.Equal(t, to.ID, created.ToAccountID)
	assert.Equal(t, int64(25), created.Amount)

	tx = beginIntegrationTx(t, pool)
	_, err = transfers.Create(ctx, tx, entity.Transfer{
		FromAccountID: from.ID,
		ToAccountID:   from.ID,
		Amount:        25,
		Status:        entity.TransferStatusCompleted,
	})
	require.Error(t, err)
	assert.ErrorIs(t, mapPgError(err), ErrCheckViolation)
	require.NoError(t, tx.Rollback(ctx))
}
