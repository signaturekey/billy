package http

import (
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/signaturekey/billy/internal/transport/http/handler"
	"github.com/signaturekey/billy/internal/transport/http/middleware"
)

type Router struct {
	accountHandler  *handler.AccountHandler
	transferHandler *handler.TransferHandler
}

func NewRouter(accountHandler *handler.AccountHandler, transferHandler *handler.TransferHandler) *Router {
	return &Router{
		accountHandler:  accountHandler,
		transferHandler: transferHandler,
	}
}

func (r *Router) Mount() stdhttp.Handler {
	g := gin.New()
	g.Use(gin.Recovery())

	g.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(stdhttp.StatusOK, gin.H{"status": "ok"})
	})

	v1 := g.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware())
	v1.POST("/accounts", r.accountHandler.Create)
	v1.GET("/accounts/:id", r.accountHandler.GetByID)
	v1.GET("/accounts/:id/balance", r.accountHandler.GetBalance)
	v1.GET("/accounts/:id/operations", r.accountHandler.ListOperations)
	v1.POST("/accounts/:id/topups", r.accountHandler.TopUp)
	v1.POST("/accounts/:id/withdrawals", r.accountHandler.Withdraw)
	v1.POST("/transfers", r.transferHandler.Create)

	return g
}
