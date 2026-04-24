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

func TestIdempotencyRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newIntegrationPool(t)
	keys := NewIdempotencyRepository()

	tx := beginIntegrationTx(t, pool)
	record := entity.IdempotencyKey{
		UserID:        10,
		Key:           "request-key",
		OperationType: "topup",
		RequestHash:   "hash-a",
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	require.NoError(t, keys.CreateProcessing(ctx, tx, record))
	require.ErrorIs(t, keys.CreateProcessing(ctx, tx, record), domainerrors.ErrIdempotencyKeyExists)

	found, err := keys.GetByKey(ctx, tx, record.UserID, record.Key, record.OperationType)
	require.NoError(t, err)
	assert.Equal(t, record.RequestHash, found.RequestHash)
	assert.Equal(t, entity.IdempotencyStatusProcessing, found.Status)

	require.NoError(t, keys.MarkCompleted(ctx, tx, record.UserID, record.Key, record.OperationType, 201, []byte(`{"id":10}`)))
	completed, err := keys.GetByKey(ctx, tx, record.UserID, record.Key, record.OperationType)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	assert.Equal(t, entity.IdempotencyStatusCompleted, completed.Status)
	assert.Equal(t, 201, completed.ResponseCode)
	assert.JSONEq(t, `{"id":10}`, string(completed.ResponseBody))

	tx = beginIntegrationTx(t, pool)
	_, err = keys.GetByKey(ctx, tx, record.UserID, "missing", record.OperationType)
	require.ErrorIs(t, err, domainerrors.ErrIdempotencyNotFound)
	require.NoError(t, tx.Rollback(ctx))
}
