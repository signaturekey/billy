package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/signaturekey/billy/internal/domain/entity"
	"github.com/signaturekey/billy/internal/pkg/pagination"
	"github.com/signaturekey/billy/internal/transport/http/dto"
	transporterrors "github.com/signaturekey/billy/internal/transport/http/errors"
	"github.com/signaturekey/billy/internal/transport/http/middleware"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

type AccountService interface {
	Create(ctx context.Context, userID int64, currency string) (entity.Account, error)
	GetByID(ctx context.Context, userID int64, accountID int64) (entity.Account, error)
	GetBalance(ctx context.Context, userID int64, accountID int64) (entity.AccountBalance, error)
	TopUp(ctx context.Context, userID int64, accountID int64, amount int64) (entity.LedgerEntry, error)
	TopUpInTx(ctx context.Context, tx pgx.Tx, userID int64, accountID int64, amount int64) (entity.LedgerEntry, error)
	Withdraw(ctx context.Context, userID int64, accountID int64, amount int64) (entity.LedgerEntry, error)
	WithdrawInTx(ctx context.Context, tx pgx.Tx, userID int64, accountID int64, amount int64) (entity.LedgerEntry, error)
	ListOperations(
		ctx context.Context,
		userID int64,
		accountID int64,
		limit int,
		offset int,
	) ([]entity.LedgerEntry, error)
}

type AccountHandler struct {
	service     AccountService
	idempotency IdempotencyExecutor
}

func NewAccountHandler(service AccountService, idempotency IdempotencyExecutor) *AccountHandler {
	return &AccountHandler{
		service:     service,
		idempotency: idempotency,
	}
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

func (handler *AccountHandler) TopUp(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	idempotencyKey, ok := requireIdempotencyKey(ctx)
	if !ok {
		return
	}

	var request dto.TopUpRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	id := middleware.CurrentUserID(ctx)
	hash, err := requestHash(struct {
		UserID    int64 `json:"user_id"`
		AccountID int64 `json:"account_id"`
		Amount    int64 `json:"amount"`
	}{
		UserID:    id,
		AccountID: accountID,
		Amount:    request.Amount,
	})
	if err != nil {
		response.InternalError(ctx)
		return
	}

	result, err := handler.idempotency.Execute(
		ctx.Request.Context(),
		idempotencyKey,
		"topup",
		hash,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) {
			entry, err := handler.service.TopUpInTx(ctx, tx, id, accountID, request.Amount)
			if err != nil {
				return mutationError(err)
			}
			return created(dto.NewLedgerEntryResponse(entry))
		},
	)
	if err != nil {
		if writeIdempotencyError(ctx, err) {
			return
		}
		transporterrors.WriteAccountError(ctx, err)
		return
	}

	writeIdempotencyResult(ctx, result)
}

func (handler *AccountHandler) Withdraw(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	idempotencyKey, ok := requireIdempotencyKey(ctx)
	if !ok {
		return
	}

	var request dto.WithdrawalRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.BadRequest(ctx, "invalid request body")
		return
	}

	id := middleware.CurrentUserID(ctx)
	hash, err := requestHash(struct {
		UserID    int64 `json:"user_id"`
		AccountID int64 `json:"account_id"`
		Amount    int64 `json:"amount"`
	}{
		UserID:    id,
		AccountID: accountID,
		Amount:    request.Amount,
	})
	if err != nil {
		response.InternalError(ctx)
		return
	}

	result, err := handler.idempotency.Execute(
		ctx.Request.Context(),
		idempotencyKey,
		"withdrawal",
		hash,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) {
			entry, err := handler.service.WithdrawInTx(ctx, tx, id, accountID, request.Amount)
			if err != nil {
				return mutationError(err)
			}
			return created(dto.NewLedgerEntryResponse(entry))
		},
	)
	if err != nil {
		if writeIdempotencyError(ctx, err) {
			return
		}
		transporterrors.WriteAccountError(ctx, err)
		return
	}

	writeIdempotencyResult(ctx, result)
}

func (handler *AccountHandler) ListOperations(ctx *gin.Context) {
	accountID, ok := parseAccountID(ctx)
	if !ok {
		return
	}

	paginationParams := pagination.FromContext(ctx)

	id := middleware.CurrentUserID(ctx)
	entries, err := handler.service.ListOperations(
		ctx.Request.Context(),
		id,
		accountID,
		paginationParams.Limit,
		paginationParams.Offset,
	)
	if err != nil {
		transporterrors.WriteAccountError(ctx, err)
		return
	}

	response.OK(ctx, dto.NewOperationsResponse(entries))
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
