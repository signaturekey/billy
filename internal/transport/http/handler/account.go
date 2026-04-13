package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/signaturekey/billy/internal/domain/entity"
	"github.com/signaturekey/billy/internal/transport/http/dto"
	transporterrors "github.com/signaturekey/billy/internal/transport/http/errors"
	"github.com/signaturekey/billy/internal/transport/http/middleware"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type AccountService interface {
	Create(ctx context.Context, userID int64, currency string) (entity.Account, error)
	GetByID(ctx context.Context, userID int64, accountID int64) (entity.Account, error)
	GetBalance(ctx context.Context, userID int64, accountID int64) (entity.AccountBalance, error)
}

type AccountHandler struct {
	service AccountService
}

func NewAccountHandler(service AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

func (handler *AccountHandler) Create(ctx *gin.Context) {
	var request dto.CreateAccountRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	id := middleware.CurrentUserID(ctx)
	account, err := handler.service.Create(ctx.Request.Context(), id, request.Currency)
	if err != nil {
		transporterrors.WriteAccountError(ctx, err)
		return
	}

	response.Created(ctx, dto.NewAccountResponse(account))
}

func (handler *AccountHandler) GetByID(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	id := middleware.CurrentUserID(ctx)
	account, err := handler.service.GetByID(ctx.Request.Context(), id, accountID)
	if err != nil {
		transporterrors.WriteAccountError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewAccountResponse(account))
}

func (handler *AccountHandler) GetBalance(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	id := middleware.CurrentUserID(ctx)
	balance, err := handler.service.GetBalance(ctx.Request.Context(), id, accountID)
	if err != nil {
		transporterrors.WriteAccountError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewBalanceResponse(balance))
}

func parseAccountID(ctx *gin.Context) (int64, bool) {
	raw := ctx.Param("id")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		response.BadRequest(ctx, "invalid account id")
		return 0, false
	}

	return id, true
}
