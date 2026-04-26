package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/config"
	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestIdempotencyCacheRoundTrip(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client, err := NewRedisClient(config.RedisConfig{
		URL:          "redis://" + server.Addr() + "/0",
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	cache := NewIdempotencyCache(client)
	record := entity.IdempotencyKey{
		UserID:        42,
		Key:           "payment-request-123",
		OperationType: "transfer",
		RequestHash:   "request-hash",
		Status:        entity.IdempotencyStatusCompleted,
		ResponseCode:  201,
		ResponseBody:  []byte(`{"id":10}`),
	}
	ttl := time.Minute

	require.NoError(t, cache.Set(context.Background(), record, ttl))

	cached, err := cache.Get(context.Background(), record.UserID, record.Key, record.OperationType)
	require.NoError(t, err)
	assert.Equal(t, record.RequestHash, cached.RequestHash)
	assert.Equal(t, record.ResponseCode, cached.ResponseCode)
	assert.JSONEq(t, string(record.ResponseBody), string(cached.ResponseBody))
	assert.NotContains(t, server.Keys()[0], record.Key)

	server.FastForward(ttl + time.Second)
	_, err = cache.Get(context.Background(), record.UserID, record.Key, record.OperationType)
	require.ErrorIs(t, err, domainerrors.ErrIdempotencyNotFound)
}

func TestNewRedisClientRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	client, err := NewRedisClient(config.RedisConfig{URL: "://invalid"})
	require.Error(t, err)
	assert.Nil(t, client)
}
