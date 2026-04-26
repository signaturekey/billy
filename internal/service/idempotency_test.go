package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestIdempotencyExecutorStoresCompletedResponse(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	cache := newIdempotencyTestCache()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, cache, time.Hour)

	result, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 10}, nil
	})
	require.NoError(t, err)
	assert.False(t, result.Replayed)
	assert.Equal(t, 201, result.StatusCode)
	assert.JSONEq(t, `{"id":10}`, string(result.Body))

	cached, ok := cache.records[idempotencyTestRecordKey(1, "key", "topup")]
	require.True(t, ok)
	assert.Equal(t, "hash-a", cached.RequestHash)
	assert.Equal(t, time.Hour, cache.ttl)
}

func TestIdempotencyExecutorReplaysCompletedDuplicate(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, nil, time.Hour)

	calls := 0
	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		calls++
		return 201, map[string]int{"id": 10}, nil
	})
	require.NoError(t, err)

	replayed, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		calls++
		return 201, map[string]int{"id": 11}, nil
	})
	require.NoError(t, err)
	assert.True(t, replayed.Replayed)
	assert.Equal(t, 1, calls)
	assert.JSONEq(t, `{"id":10}`, string(replayed.Body))
}

func TestIdempotencyExecutorRejectsSameKeyDifferentHash(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, nil, time.Hour)

	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 10}, nil
	})
	require.NoError(t, err)

	_, err = executor.Execute(context.Background(), 1, "key", "topup", "hash-b", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 11}, nil
	})
	require.ErrorIs(t, err, domainerrors.ErrIdempotencyKeyConflict)
}

func TestIdempotencyExecutorRejectsInProgressDuplicate(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	keys.records[idempotencyTestRecordKey(1, "key", "topup")] = entity.IdempotencyKey{
		UserID:        1,
		Key:           "key",
		OperationType: "topup",
		RequestHash:   "hash-a",
		Status:        entity.IdempotencyStatusProcessing,
	}
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, nil, time.Hour)

	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, nil, nil
	})
	require.ErrorIs(t, err, domainerrors.ErrIdempotencyInProgress)
}

func TestIdempotencyExecutorReplaysCachedResponseWithoutDatabase(t *testing.T) {
	t.Parallel()

	cache := newIdempotencyTestCache()
	cache.records[idempotencyTestRecordKey(1, "key", "topup")] = entity.IdempotencyKey{
		UserID:        1,
		Key:           "key",
		OperationType: "topup",
		RequestHash:   "hash-a",
		Status:        entity.IdempotencyStatusCompleted,
		ResponseCode:  201,
		ResponseBody:  []byte(`{"id":10}`),
	}
	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, cache, time.Hour)

	mutated := false
	result, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		mutated = true
		return 201, nil, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Replayed)
	assert.False(t, mutated)
	assert.Empty(t, keys.records)
	assert.JSONEq(t, `{"id":10}`, string(result.Body))
}

func TestIdempotencyExecutorRejectsCachedResponseForDifferentRequest(t *testing.T) {
	t.Parallel()

	cache := newIdempotencyTestCache()
	cache.records[idempotencyTestRecordKey(1, "key", "topup")] = entity.IdempotencyKey{
		UserID:        1,
		Key:           "key",
		OperationType: "topup",
		RequestHash:   "hash-a",
		Status:        entity.IdempotencyStatusCompleted,
	}
	executor := NewIdempotencyExecutor(
		idempotencyTestTxManager{},
		newIdempotencyTestRepository(),
		cache,
		time.Hour,
	)

	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-b", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, nil, nil
	})
	require.ErrorIs(t, err, domainerrors.ErrIdempotencyKeyConflict)
}

func TestIdempotencyExecutorFallsBackWhenCacheIsUnavailable(t *testing.T) {
	t.Parallel()

	cache := newIdempotencyTestCache()
	cache.getErr = errors.New("redis unavailable")
	cache.setErr = errors.New("redis unavailable")
	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, cache, time.Hour)

	result, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 10}, nil
	})
	require.NoError(t, err)
	assert.False(t, result.Replayed)
	assert.Len(t, keys.records, 1)
}

type idempotencyTestTxManager struct{}

func (idempotencyTestTxManager) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}

type idempotencyTestRepository struct {
	records map[string]entity.IdempotencyKey
}

type idempotencyTestCache struct {
	records map[string]entity.IdempotencyKey
	getErr  error
	setErr  error
	ttl     time.Duration
}

func newIdempotencyTestCache() *idempotencyTestCache {
	return &idempotencyTestCache{records: make(map[string]entity.IdempotencyKey)}
}

func (cache *idempotencyTestCache) Get(
	_ context.Context,
	userID int64,
	key string,
	operationType string,
) (entity.IdempotencyKey, error) {
	if cache.getErr != nil {
		return entity.IdempotencyKey{}, cache.getErr
	}

	record, ok := cache.records[idempotencyTestRecordKey(userID, key, operationType)]
	if !ok {
		return entity.IdempotencyKey{}, domainerrors.ErrIdempotencyNotFound
	}
	return record, nil
}

func (cache *idempotencyTestCache) Set(
	_ context.Context,
	record entity.IdempotencyKey,
	ttl time.Duration,
) error {
	if cache.setErr != nil {
		return cache.setErr
	}

	cache.records[idempotencyTestRecordKey(record.UserID, record.Key, record.OperationType)] = record
	cache.ttl = ttl
	return nil
}

func newIdempotencyTestRepository() *idempotencyTestRepository {
	return &idempotencyTestRepository{records: make(map[string]entity.IdempotencyKey)}
}

func (repo *idempotencyTestRepository) CreateProcessing(
	_ context.Context,
	_ pgx.Tx,
	record entity.IdempotencyKey,
) error {
	key := idempotencyTestRecordKey(record.UserID, record.Key, record.OperationType)
	if _, ok := repo.records[key]; ok {
		return domainerrors.ErrIdempotencyKeyExists
	}
	record.Status = entity.IdempotencyStatusProcessing
	repo.records[key] = record
	return nil
}

func (repo *idempotencyTestRepository) GetByKey(
	_ context.Context,
	_ pgx.Tx,
	userID int64,
	key string,
	operationType string,
) (entity.IdempotencyKey, error) {
	record, ok := repo.records[idempotencyTestRecordKey(userID, key, operationType)]
	if !ok {
		return entity.IdempotencyKey{}, domainerrors.ErrIdempotencyNotFound
	}
	return record, nil
}

func (repo *idempotencyTestRepository) MarkCompleted(
	_ context.Context,
	_ pgx.Tx,
	userID int64,
	key string,
	operationType string,
	responseCode int,
	responseBody []byte,
) error {
	recordKey := idempotencyTestRecordKey(userID, key, operationType)
	record, ok := repo.records[recordKey]
	if !ok {
		return domainerrors.ErrIdempotencyNotFound
	}
	record.Status = entity.IdempotencyStatusCompleted
	record.ResponseCode = responseCode
	record.ResponseBody = responseBody
	repo.records[recordKey] = record
	return nil
}

func idempotencyTestRecordKey(userID int64, key string, operationType string) string {
	return fmt.Sprintf("%d:%s:%s", userID, key, operationType)
}
