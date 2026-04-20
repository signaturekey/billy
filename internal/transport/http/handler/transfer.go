package handler

import (
	"context"

	"github.com/gin-gonic/gin"
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
}

type TransferHandler struct {
	service TransferService
}

func NewTransferHandler(service TransferService) *TransferHandler {
	return &TransferHandler{service: service}
}

func (handler *TransferHandler) Create(ctx *gin.Context) {
	var request dto.CreateTransferRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	userID := middleware.CurrentUserID(ctx)
	transfer, err := handler.service.Create(
		ctx.Request.Context(),
		userID,
		request.FromAccountID,
		request.ToAccountID,
		request.Amount,
	)
	if err != nil {
		transporterrors.WriteTransferError(ctx, err)
		return
	}

	response.Created(ctx, dto.NewTransferResponse(transfer))
}
