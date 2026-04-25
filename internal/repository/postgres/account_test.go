package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestAccountRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	accounts := NewAccountRepository(pool)
	seedIntegrationUser(t, pool, 10)

	created, err := accounts.Create(ctx, entity.Account{
		UserID:         10,
		Currency:       "USD",
		Balance:        100,
		ReservedAmount: 25,
		Status:         entity.AccountStatusActive,
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, int64(10), created.UserID)
	assert.Equal(t, "USD", created.Currency)
	assert.Equal(t, int64(100), created.Balance)
	assert.Equal(t, int64(25), created.ReservedAmount)

	found, err := accounts.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	tx := beginIntegrationTx(t, pool)
	locked, err := accounts.GetForUpdate(ctx, tx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, locked.ID)
	require.NoError(t, accounts.UpdateBalance(ctx, tx, created.ID, 140))
	require.NoError(t, accounts.UpdateAmounts(ctx, tx, created.ID, 120, 40))
	require.NoError(t, tx.Commit(ctx))

	updated, err := accounts.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(120), updated.Balance)
	assert.Equal(t, int64(40), updated.ReservedAmount)

	_, err = accounts.GetByID(ctx, created.ID+1000)
	require.ErrorIs(t, err, domainerrors.ErrAccountNotFound)
}

func TestAccountRepositoryRejectsInvalidAmounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	accounts := NewAccountRepository(pool)
	seedIntegrationUser(t, pool, 10)

	_, err := accounts.Create(ctx, entity.Account{
		UserID:         10,
		Currency:       "USD",
		Balance:        10,
		ReservedAmount: 11,
		Status:         entity.AccountStatusActive,
	})
	require.Error(t, err)
	assert.ErrorIs(t, mapPgError(err), ErrCheckViolation)
}
