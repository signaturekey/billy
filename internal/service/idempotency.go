package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

const defaultIdempotencyTTL = 24 * time.Hour

type IdempotencyRepository interface {
	CreateProcessing(ctx context.Context, tx pgx.Tx, record entity.IdempotencyKey) error
	GetByKey(
		ctx context.Context,
		tx pgx.Tx,
		userID int64,
		key string,
		operationType string,
	) (entity.IdempotencyKey, error)
	MarkCompleted(
		ctx context.Context,
		tx pgx.Tx,
		userID int64,
		key string,
		operationType string,
		responseCode int,
		responseBody []byte,
	) error
}

type IdempotencyCache interface {
	Get(
		ctx context.Context,
		userID int64,
		key string,
		operationType string,
	) (entity.IdempotencyKey, error)
	Set(ctx context.Context, record entity.IdempotencyKey, ttl time.Duration) error
}

type IdempotencyExecutor struct {
	txManager TxManager
	keys      IdempotencyRepository
	cache     IdempotencyCache
	ttl       time.Duration
}

type IdempotencyResult struct {
	StatusCode int
	Body       []byte
	Replayed   bool
}

type IdempotentMutation func(ctx context.Context, tx pgx.Tx) (int, any, error)

func NewIdempotencyExecutor(
	txManager TxManager,
	keys IdempotencyRepository,
	cache IdempotencyCache,
	ttl time.Duration,
) *IdempotencyExecutor {
	if ttl <= 0 {
		ttl = defaultIdempotencyTTL
	}

	return &IdempotencyExecutor{
		txManager: txManager,
		keys:      keys,
		cache:     cache,
		ttl:       ttl,
	}
}

func (executor *IdempotencyExecutor) Execute(
	ctx context.Context,
	userID int64,
	key string,
	operationType string,
	requestHash string,
	mutate IdempotentMutation,
) (IdempotencyResult, error) {
	if cached, found, err := executor.getCached(ctx, userID, key, operationType, requestHash); found || err != nil {
		return cached, err
	}

	var result IdempotencyResult
	err := executor.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		err := executor.keys.CreateProcessing(ctx, tx, entity.IdempotencyKey{
			UserID:        userID,
			Key:           key,
			OperationType: operationType,
			RequestHash:   requestHash,
			ExpiresAt:     time.Now().Add(executor.ttl),
		})
		if err != nil {
			if errors.Is(err, domainerrors.ErrIdempotencyKeyExists) {
				existing, err := executor.keys.GetByKey(ctx, tx, userID, key, operationType)
				if err != nil {
					return err
				}

				if existing.RequestHash != requestHash {
					return domainerrors.ErrIdempotencyKeyConflict
				}

				if existing.Status != entity.IdempotencyStatusCompleted {
					return domainerrors.ErrIdempotencyInProgress
				}

				result = IdempotencyResult{
					StatusCode: existing.ResponseCode,
					Body:       existing.ResponseBody,
					Replayed:   true,
				}
				return nil
			}

			return err
		}

		statusCode, payload, err := mutate(ctx, tx)
		if err != nil {
			return err
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		if err := executor.keys.MarkCompleted(ctx, tx, userID, key, operationType, statusCode, body); err != nil {
			return err
		}

		result = IdempotencyResult{
			StatusCode: statusCode,
			Body:       body,
			Replayed:   false,
		}
		return nil
	})
	if err != nil {
		return IdempotencyResult{}, err
	}

	executor.cacheCompleted(ctx, userID, key, operationType, requestHash, result)

	return result, nil
}

func (executor *IdempotencyExecutor) getCached(
	ctx context.Context,
	userID int64,
	key string,
	operationType string,
	requestHash string,
) (IdempotencyResult, bool, error) {
	if executor.cache == nil {
		return IdempotencyResult{}, false, nil
	}

	cached, err := executor.cache.Get(ctx, userID, key, operationType)
	if err != nil || cached.Status != entity.IdempotencyStatusCompleted {
		return IdempotencyResult{}, false, nil
	}

	if cached.RequestHash != requestHash {
		return IdempotencyResult{}, true, domainerrors.ErrIdempotencyKeyConflict
	}

	return IdempotencyResult{
		StatusCode: cached.ResponseCode,
		Body:       cached.ResponseBody,
		Replayed:   true,
	}, true, nil
}

func (executor *IdempotencyExecutor) cacheCompleted(
	ctx context.Context,
	userID int64,
	key string,
	operationType string,
	requestHash string,
	result IdempotencyResult,
) {
	if executor.cache == nil {
		return
	}

	_ = executor.cache.Set(ctx, entity.IdempotencyKey{
		UserID:        userID,
		Key:           key,
		OperationType: operationType,
		RequestHash:   requestHash,
		Status:        entity.IdempotencyStatusCompleted,
		ResponseCode:  result.StatusCode,
		ResponseBody:  result.Body,
	}, executor.ttl)
}
