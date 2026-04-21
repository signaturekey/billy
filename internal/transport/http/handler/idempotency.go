package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
	"github.com/signaturekey/billy/internal/service"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

const idempotencyKeyHeader = "Idempotency-Key"

type IdempotencyExecutor interface {
	Execute(
		ctx context.Context,
		key string,
		operationType string,
		requestHash string,
		mutate service.IdempotentMutation,
	) (service.IdempotencyResult, error)
}

func requireIdempotencyKey(ctx *gin.Context) (string, bool) {
	key := strings.TrimSpace(ctx.GetHeader(idempotencyKeyHeader))
	if key == "" {
		response.BadRequest(ctx, "missing idempotency key")
		return "", false
	}

	return key, true
}

func requestHash(payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func writeIdempotencyResult(ctx *gin.Context, result service.IdempotencyResult) {
	ctx.Data(result.StatusCode, "application/json; charset=utf-8", result.Body)
}

func writeIdempotencyError(ctx *gin.Context, err error) bool {
	switch {
	case errors.Is(err, domainerrors.ErrIdempotencyKeyConflict):
		response.Conflict(ctx, "idempotency key conflict")
		return true
	case errors.Is(err, domainerrors.ErrIdempotencyInProgress):
		response.Conflict(ctx, "idempotency request in progress")
		return true
	default:
		return false
	}
}

func created(payload any) (int, any, error) {
	return http.StatusCreated, payload, nil
}

func okResponse(payload any) (int, any, error) {
	return http.StatusOK, payload, nil
}

func mutationError(err error) (int, any, error) {
	return 0, nil, err
}
