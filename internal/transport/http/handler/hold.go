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

type HoldService interface {
	Create(ctx context.Context, userID int64, accountID int64, amount int64) (entity.Hold, error)
	Confirm(ctx context.Context, userID int64, holdID int64) (entity.Hold, error)
	Cancel(ctx context.Context, userID int64, holdID int64) (entity.Hold, error)
}

type HoldHandler struct {
	service HoldService
}

func NewHoldHandler(service HoldService) *HoldHandler {
	return &HoldHandler{service: service}
}

func (handler *HoldHandler) Create(ctx *gin.Context) {
	var request dto.CreateHoldRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hold, err := handler.service.Create(ctx.Request.Context(), userID, request.AccountID, request.Amount)
	if err != nil {
		transporterrors.WriteHoldError(ctx, err)
		return
	}

	response.Created(ctx, dto.NewHoldResponse(hold))
}

func (handler *HoldHandler) Confirm(ctx *gin.Context) {
	holdID, ok := parseHoldID(ctx)
	if !ok {
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hold, err := handler.service.Confirm(ctx.Request.Context(), userID, holdID)
	if err != nil {
		transporterrors.WriteHoldError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewHoldResponse(hold))
}

func (handler *HoldHandler) Cancel(ctx *gin.Context) {
	holdID, ok := parseHoldID(ctx)
	if !ok {
		return
	}

	userID := middleware.CurrentUserID(ctx)
	hold, err := handler.service.Cancel(ctx.Request.Context(), userID, holdID)
	if err != nil {
		transporterrors.WriteHoldError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewHoldResponse(hold))
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
