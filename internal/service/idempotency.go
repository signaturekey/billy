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
	GetByKey(ctx context.Context, tx pgx.Tx, key string, operationType string) (entity.IdempotencyKey, error)
	MarkCompleted(
		ctx context.Context,
		tx pgx.Tx,
		key string,
		operationType string,
		responseCode int,
		responseBody []byte,
	) error
}

type IdempotencyExecutor struct {
	txManager TxManager
	keys      IdempotencyRepository
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
	ttl time.Duration,
) *IdempotencyExecutor {
	if ttl <= 0 {
		ttl = defaultIdempotencyTTL
	}

	return &IdempotencyExecutor{
		txManager: txManager,
		keys:      keys,
		ttl:       ttl,
	}
}

func (executor *IdempotencyExecutor) Execute(
	ctx context.Context,
	key string,
	operationType string,
	requestHash string,
	mutate IdempotentMutation,
) (IdempotencyResult, error) {
	var result IdempotencyResult
	err := executor.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		err := executor.keys.CreateProcessing(ctx, tx, entity.IdempotencyKey{
			Key:           key,
			OperationType: operationType,
			RequestHash:   requestHash,
			ExpiresAt:     time.Now().Add(executor.ttl),
		})
		if err != nil {
			if errors.Is(err, domainerrors.ErrIdempotencyKeyExists) {
				existing, err := executor.keys.GetByKey(ctx, tx, key, operationType)
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

		if err := executor.keys.MarkCompleted(ctx, tx, key, operationType, statusCode, body); err != nil {
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

	return result, nil
}
