package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/signaturekey/billy/internal/domain/entity"
	"github.com/signaturekey/billy/internal/transport/http/dto"
	transporterrors "github.com/signaturekey/billy/internal/transport/http/errors"
	"github.com/signaturekey/billy/internal/transport/http/middleware"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type TransferService interface {
	Create(
		ctx context.Context,
		userID int64,
		fromAccountID int64,
		toAccountID int64,
		amount int64,
	) (entity.Transfer, error)
	CreateInTx(
		ctx context.Context,
		tx pgx.Tx,
		userID int64,
		fromAccountID int64,
		toAccountID int64,
		amount int64,
	) (entity.Transfer, error)
}

type TransferHandler struct {
	service     TransferService
	idempotency IdempotencyExecutor
}

func NewTransferHandler(service TransferService, idempotency IdempotencyExecutor) *TransferHandler {
	return &TransferHandler{
		service:     service,
		idempotency: idempotency,
	}
}

func (handler *TransferHandler) Create(ctx *gin.Context) {
	idempotencyKey, ok := requireIdempotencyKey(ctx)
	if !ok {
		return
	}

	var request dto.CreateTransferRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hash, err := requestHash(struct {
		UserID        int64 `json:"user_id"`
		FromAccountID int64 `json:"from_account_id"`
		ToAccountID   int64 `json:"to_account_id"`
		Amount        int64 `json:"amount"`
	}{
		UserID:        userID,
		FromAccountID: request.FromAccountID,
		ToAccountID:   request.ToAccountID,
		Amount:        request.Amount,
	})
	if err != nil {
		response.InternalError(ctx)
		return
	}

	result, err := handler.idempotency.Execute(
		ctx.Request.Context(),
		userID,
		idempotencyKey,
		"transfer",
		hash,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) {
			transfer, err := handler.service.CreateInTx(
				ctx,
				tx,
				userID,
				request.FromAccountID,
				request.ToAccountID,
				request.Amount,
			)
			if err != nil {
				return mutationError(err)
			}
			return created(dto.NewTransferResponse(transfer))
		},
	)
	if err != nil {
		if writeIdempotencyError(ctx, err) {
			return
		}
		transporterrors.WriteTransferError(ctx, err)
		return
	}

	writeIdempotencyResult(ctx, result)
}
