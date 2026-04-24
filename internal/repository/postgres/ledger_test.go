package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
)

func TestLedgerRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	accounts := NewAccountRepository(pool)
	ledger := NewLedgerRepository(pool)

	account := createIntegrationAccount(t, accounts, 10, "USD", 100, 0)
	tx := beginIntegrationTx(t, pool)
	first, err := ledger.Create(ctx, tx, entity.LedgerEntry{
		AccountID:     account.ID,
		Type:          entity.LedgerEntryTypeTopup,
		Amount:        50,
		Currency:      "USD",
		BalanceBefore: 0,
		BalanceAfter:  50,
	})
	require.NoError(t, err)
	second, err := ledger.Create(ctx, tx, entity.LedgerEntry{
		AccountID:     account.ID,
		Type:          entity.LedgerEntryTypeWithdrawal,
		Amount:        20,
		Currency:      "USD",
		BalanceBefore: 50,
		BalanceAfter:  30,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	entries, err := ledger.ListByAccount(ctx, account.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, second.ID, entries[0].ID)
	assert.Equal(t, first.ID, entries[1].ID)

	paged, err := ledger.ListByAccount(ctx, account.ID, 1, 1)
	require.NoError(t, err)
	require.Len(t, paged, 1)
	assert.Equal(t, first.ID, paged[0].ID)
}
