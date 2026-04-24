package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestHoldRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	accounts := NewAccountRepository(pool)
	holds := NewHoldRepository(pool)

	account := createIntegrationAccount(t, accounts, 10, "USD", 100, 0)
	now := time.Now().UTC()

	tx := beginIntegrationTx(t, pool)
	expired, err := holds.Create(ctx, tx, entity.Hold{
		AccountID: account.ID,
		Amount:    30,
		Status:    entity.HoldStatusPending,
		ExpiresAt: now.Add(-time.Hour),
	})
	require.NoError(t, err)
	future, err := holds.Create(ctx, tx, entity.Hold{
		AccountID: account.ID,
		Amount:    20,
		Status:    entity.HoldStatusPending,
		ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	confirmed, err := holds.Create(ctx, tx, entity.Hold{
		AccountID: account.ID,
		Amount:    10,
		Status:    entity.HoldStatusConfirmed,
		ExpiresAt: now.Add(-time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	expiredPending, err := holds.ListExpiredPending(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, expiredPending, 1)
	assert.Equal(t, expired.ID, expiredPending[0].ID)

	tx = beginIntegrationTx(t, pool)
	locked, err := holds.GetByIDForUpdate(ctx, tx, future.ID)
	require.NoError(t, err)
	assert.Equal(t, future.ID, locked.ID)
	updated, err := holds.UpdateStatus(ctx, tx, future.ID, entity.HoldStatusCancelled)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	assert.Equal(t, entity.HoldStatusCancelled, updated.Status)

	tx = beginIntegrationTx(t, pool)
	_, err = holds.GetByIDForUpdate(ctx, tx, confirmed.ID+1000)
	require.ErrorIs(t, err, domainerrors.ErrHoldNotFound)
	require.NoError(t, tx.Rollback(ctx))

	expiredPending, err = holds.ListExpiredPending(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, expiredPending, 1)
	assert.Equal(t, expired.ID, expiredPending[0].ID)
}
