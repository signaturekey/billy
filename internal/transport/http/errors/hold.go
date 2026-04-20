package errors

import (
	"errors"

	"github.com/gin-gonic/gin"
	domainerrors "github.com/signaturekey/billy/internal/domain/errors"
	"github.com/signaturekey/billy/internal/transport/http/response"
)

func WriteHoldError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, domainerrors.ErrInvalidAmount):
		response.BadRequest(ctx, "invalid amount")
	case errors.Is(err, domainerrors.ErrAccountNotFound):
		response.NotFound(ctx, "account not found")
	case errors.Is(err, domainerrors.ErrHoldNotFound):
		response.NotFound(ctx, "hold not found")
	case errors.Is(err, domainerrors.ErrForbidden):
		response.Forbidden(ctx, "forbidden")
	case errors.Is(err, domainerrors.ErrAccountBlocked):
		response.Conflict(ctx, "account blocked")
	case errors.Is(err, domainerrors.ErrHoldExpired):
		response.Gone(ctx, "hold expired")
	case errors.Is(err, domainerrors.ErrHoldAlreadyConfirmed):
		response.Conflict(ctx, "hold already confirmed")
	case errors.Is(err, domainerrors.ErrHoldAlreadyCancelled):
		response.Conflict(ctx, "hold already cancelled")
	case errors.Is(err, domainerrors.ErrInvalidHoldStateTransition):
		response.Conflict(ctx, "invalid hold state transition")
	case errors.Is(err, domainerrors.ErrInsufficientFunds):
		response.UnprocessableEntity(ctx, "insufficient funds")
	default:
		response.InternalError(ctx)
	}
}
