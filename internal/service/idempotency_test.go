package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

func TestIdempotencyExecutorStoresCompletedResponse(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, time.Hour)

	result, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 10}, nil
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Replayed {
		t.Fatalf("result replayed = true, want false")
	}
	if result.StatusCode != 201 || string(result.Body) != `{"id":10}` {
		t.Fatalf("result = status %d body %s, want 201/{\"id\":10}", result.StatusCode, result.Body)
	}
}

func TestIdempotencyExecutorReplaysCompletedDuplicate(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, time.Hour)

	calls := 0
	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		calls++
		return 201, map[string]int{"id": 10}, nil
	})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}

	replayed, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		calls++
		return 201, map[string]int{"id": 11}, nil
	})
	if err != nil {
		t.Fatalf("replay execute: %v", err)
	}
	if !replayed.Replayed {
		t.Fatalf("replayed = false, want true")
	}
	if calls != 1 {
		t.Fatalf("mutate calls = %d, want 1", calls)
	}
	if string(replayed.Body) != `{"id":10}` {
		t.Fatalf("replayed body = %s, want first response", replayed.Body)
	}
}

func TestIdempotencyExecutorRejectsSameKeyDifferentHash(t *testing.T) {
	t.Parallel()

	keys := newIdempotencyTestRepository()
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, time.Hour)

	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 10}, nil
	})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}

	_, err = executor.Execute(context.Background(), 1, "key", "topup", "hash-b", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, map[string]int{"id": 11}, nil
	})
	if !errors.Is(err, domainerrors.ErrIdempotencyKeyConflict) {
		t.Fatalf("conflicting execute error = %v, want ErrIdempotencyKeyConflict", err)
	}
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
	executor := NewIdempotencyExecutor(idempotencyTestTxManager{}, keys, time.Hour)

	_, err := executor.Execute(context.Background(), 1, "key", "topup", "hash-a", func(context.Context, pgx.Tx) (int, any, error) {
		return 201, nil, nil
	})
	if !errors.Is(err, domainerrors.ErrIdempotencyInProgress) {
		t.Fatalf("in-progress duplicate error = %v, want ErrIdempotencyInProgress", err)
	}
}

type idempotencyTestTxManager struct{}

func (idempotencyTestTxManager) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}

type idempotencyTestRepository struct {
	records map[string]entity.IdempotencyKey
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
