package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(ctx *gin.Context) {
		startedAt := time.Now()
		writer := &statusWriter{
			ResponseWriter: ctx.Writer,
			status:         200,
		}
		ctx.Writer = writer

		ctx.Next()

		fields := []zap.Field{
			zap.String("method", ctx.Request.Method),
			zap.String("path", ctx.Request.URL.Path),
			zap.Int("status", writer.Status()),
			zap.Duration("duration", time.Since(startedAt)),
			zap.String("request_id", RequestID(ctx)),
			zap.String("remote_addr", ctx.ClientIP()),
		}

		if userID, ok := LookupCurrentUserID(ctx); ok {
			fields = append(fields, zap.Int64("user_id", userID))
		}

		logger.Info("http request", fields...)
	}
}

type statusWriter struct {
	gin.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.status = 200
	}

	return writer.ResponseWriter.Write(data)
}

func (writer *statusWriter) Status() int {
	if writer.status == 0 {
		return 200
	}

	return writer.status
}
