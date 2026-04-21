package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/signaturekey/billy/internal/transport/http/response"
	"go.uber.org/zap"
)

func RecoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(ctx *gin.Context) {
		defer func() {
			if value := recover(); value != nil {
				logger.Error("http panic recovered",
					zap.Any("panic", value),
					zap.String("request_id", RequestID(ctx)),
					zap.String("method", ctx.Request.Method),
					zap.String("path", ctx.Request.URL.Path),
					zap.ByteString("stack", debug.Stack()),
				)

				ctx.AbortWithStatusJSON(http.StatusInternalServerError, response.ErrorResponse{
					Error: "internal server error",
				})
			}
		}()

		ctx.Next()
	}
}
