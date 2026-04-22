package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/signaturekey/billy/internal/domain/entity"
	"github.com/signaturekey/billy/internal/transport/http/dto"
	transporterrors "github.com/signaturekey/billy/internal/transport/http/errors"
	"github.com/signaturekey/billy/internal/transport/http/middleware"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type HoldService interface {
	Create(ctx context.Context, userID int64, accountID int64, amount int64) (entity.Hold, error)
	CreateInTx(ctx context.Context, tx pgx.Tx, userID int64, accountID int64, amount int64) (entity.Hold, error)
	Confirm(ctx context.Context, userID int64, holdID int64) (entity.Hold, error)
	ConfirmInTx(ctx context.Context, tx pgx.Tx, userID int64, holdID int64) (entity.Hold, error)
	Cancel(ctx context.Context, userID int64, holdID int64) (entity.Hold, error)
	CancelInTx(ctx context.Context, tx pgx.Tx, userID int64, holdID int64) (entity.Hold, error)
}

type HoldHandler struct {
	service     HoldService
	idempotency IdempotencyExecutor
}

func NewHoldHandler(service HoldService, idempotency IdempotencyExecutor) *HoldHandler {
	return &HoldHandler{
		service:     service,
		idempotency: idempotency,
	}
}

func (handler *HoldHandler) Create(ctx *gin.Context) {
	idempotencyKey, ok := requireIdempotencyKey(ctx)
	if !ok {
		return
	}

	var request dto.CreateHoldRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hash, err := requestHash(struct {
		UserID    int64 `json:"user_id"`
		AccountID int64 `json:"account_id"`
		Amount    int64 `json:"amount"`
	}{
		UserID:    userID,
		AccountID: request.AccountID,
		Amount:    request.Amount,
	})
	if err != nil {
		response.InternalError(ctx)
		return
	}

	result, err := handler.idempotency.Execute(
		ctx.Request.Context(),
		userID,
		idempotencyKey,
		"hold_create",
		hash,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) {
			hold, err := handler.service.CreateInTx(ctx, tx, userID, request.AccountID, request.Amount)
			if err != nil {
				return mutationError(err)
			}
			return created(dto.NewHoldResponse(hold))
		},
	)
	if err != nil {
		if writeIdempotencyError(ctx, err) {
			return
		}
		transporterrors.WriteHoldError(ctx, err)
		return
	}

	writeIdempotencyResult(ctx, result)
}

func (handler *HoldHandler) Confirm(ctx *gin.Context) {
	holdID, ok := parseHoldID(ctx)
	if !ok {
		return
	}

	idempotencyKey, ok := requireIdempotencyKey(ctx)
	if !ok {
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hash, err := requestHash(struct {
		UserID int64 `json:"user_id"`
		HoldID int64 `json:"hold_id"`
	}{
		UserID: userID,
		HoldID: holdID,
	})
	if err != nil {
		response.InternalError(ctx)
		return
	}

	result, err := handler.idempotency.Execute(
		ctx.Request.Context(),
		userID,
		idempotencyKey,
		"hold_confirm",
		hash,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) {
			hold, err := handler.service.ConfirmInTx(ctx, tx, userID, holdID)
			if err != nil {
				return mutationError(err)
			}
			return okResponse(dto.NewHoldResponse(hold))
		},
	)
	if err != nil {
		if writeIdempotencyError(ctx, err) {
			return
		}
		transporterrors.WriteHoldError(ctx, err)
		return
	}

	writeIdempotencyResult(ctx, result)
}

func (handler *HoldHandler) Cancel(ctx *gin.Context) {
	holdID, ok := parseHoldID(ctx)
	if !ok {
		return
	}

	idempotencyKey, ok := requireIdempotencyKey(ctx)
	if !ok {
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hash, err := requestHash(struct {
		UserID int64 `json:"user_id"`
		HoldID int64 `json:"hold_id"`
	}{
		UserID: userID,
		HoldID: holdID,
	})
	if err != nil {
		response.InternalError(ctx)
		return
	}

	result, err := handler.idempotency.Execute(
		ctx.Request.Context(),
		userID,
		idempotencyKey,
		"hold_cancel",
		hash,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) {
			hold, err := handler.service.CancelInTx(ctx, tx, userID, holdID)
			if err != nil {
				return mutationError(err)
			}
			return okResponse(dto.NewHoldResponse(hold))
		},
	)
	if err != nil {
		if writeIdempotencyError(ctx, err) {
			return
		}
		transporterrors.WriteHoldError(ctx, err)
		return
	}

	writeIdempotencyResult(ctx, result)
}

func parseHoldID(ctx *gin.Context) (int64, bool) {
	raw := ctx.Param("id")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		response.BadRequest(ctx, "invalid hold id")
		return 0, false
	}

	return id, true
}
