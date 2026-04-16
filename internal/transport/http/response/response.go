package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func JSON(ctx *gin.Context, statusCode int, payload any) {
	ctx.JSON(statusCode, payload)
}

func OK(ctx *gin.Context, payload any) {
	JSON(ctx, http.StatusOK, payload)
}

func Created(ctx *gin.Context, payload any) {
	JSON(ctx, http.StatusCreated, payload)
}

func Error(ctx *gin.Context, statusCode int, message string) {
	ctx.JSON(statusCode, ErrorResponse{Error: message})
}

func BadRequest(ctx *gin.Context, message string) {
	Error(ctx, http.StatusBadRequest, message)
}

func Unauthorized(ctx *gin.Context, message string) {
	Error(ctx, http.StatusUnauthorized, message)
}

func Forbidden(ctx *gin.Context, message string) {
	Error(ctx, http.StatusForbidden, message)
}

func NotFound(ctx *gin.Context, message string) {
	Error(ctx, http.StatusNotFound, message)
}

func Conflict(ctx *gin.Context, message string) {
	Error(ctx, http.StatusConflict, message)
}

func UnprocessableEntity(ctx *gin.Context, message string) {
	Error(ctx, http.StatusUnprocessableEntity, message)
}

func InternalError(ctx *gin.Context) {
	Error(ctx, http.StatusInternalServerError, "internal server error")
}
