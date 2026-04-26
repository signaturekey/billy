package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/signaturekey/billy/internal/config"
	"github.com/signaturekey/billy/internal/domain/entity"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
)

const idempotencyKeyPrefix = "billy:idempotency"

type IdempotencyCache struct {
	client redisclient.Cmdable
}

type idempotencyRecord struct {
	RequestHash  string          `json:"request_hash"`
	ResponseCode int             `json:"response_code"`
	ResponseBody json.RawMessage `json:"response_body"`
}

func NewRedisClient(cfg config.RedisConfig) (*redisclient.Client, error) {
	options, err := redisclient.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	options.DialTimeout = cfg.DialTimeout
	options.ReadTimeout = cfg.ReadTimeout
	options.WriteTimeout = cfg.WriteTimeout

	return redisclient.NewClient(options), nil
}

func NewIdempotencyCache(client redisclient.Cmdable) *IdempotencyCache {
	return &IdempotencyCache{client: client}
}

func (cache *IdempotencyCache) Get(
	ctx context.Context,
	userID int64,
	key string,
	operationType string,
) (entity.IdempotencyKey, error) {
	payload, err := cache.client.Get(ctx, idempotencyKey(userID, key, operationType)).Bytes()
	if err != nil {
		if errors.Is(err, redisclient.Nil) {
			return entity.IdempotencyKey{}, domainerrors.ErrIdempotencyNotFound
		}
		return entity.IdempotencyKey{}, fmt.Errorf("get cached idempotency result: %w", err)
	}

	var cached idempotencyRecord
	if err := json.Unmarshal(payload, &cached); err != nil {
		return entity.IdempotencyKey{}, fmt.Errorf("decode cached idempotency result: %w", err)
	}

	return entity.IdempotencyKey{
		UserID:        userID,
		Key:           key,
		OperationType: operationType,
		RequestHash:   cached.RequestHash,
		Status:        entity.IdempotencyStatusCompleted,
		ResponseCode:  cached.ResponseCode,
		ResponseBody:  cached.ResponseBody,
	}, nil
}

func (cache *IdempotencyCache) Set(
	ctx context.Context,
	record entity.IdempotencyKey,
	ttl time.Duration,
) error {
	payload, err := json.Marshal(idempotencyRecord{
		RequestHash:  record.RequestHash,
		ResponseCode: record.ResponseCode,
		ResponseBody: record.ResponseBody,
	})
	if err != nil {
		return fmt.Errorf("encode cached idempotency result: %w", err)
	}

	if err := cache.client.Set(
		ctx,
		idempotencyKey(record.UserID, record.Key, record.OperationType),
		payload,
		ttl,
	).Err(); err != nil {
		return fmt.Errorf("cache idempotency result: %w", err)
	}

	return nil
}

func idempotencyKey(userID int64, key string, operationType string) string {
	digest := sha256.Sum256([]byte(operationType + "\x00" + key))
	return fmt.Sprintf("%s:%d:%x", idempotencyKeyPrefix, userID, digest)
}
