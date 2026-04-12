package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/signaturekey/billy/internal/config"
	"github.com/signaturekey/billy/internal/pkg/logger"
	postgrespkg "github.com/signaturekey/billy/internal/pkg/postgres"
	"github.com/signaturekey/billy/internal/repository/postgres"
	"github.com/signaturekey/billy/internal/service"
	transporthttp "github.com/signaturekey/billy/internal/transport/http"
	"github.com/signaturekey/billy/internal/transport/http/handler"
	"go.uber.org/zap"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	ctx := context.Background()

	cfg := config.MustLoad()

	log := logger.New(cfg.App.Env)

	db, err := postgrespkg.New(ctx, cfg.Database)
	if err != nil {
		log.Error("postgres startup failed", zap.Error(err))
		_ = log.Sync()
		os.Exit(1)
	}
	defer db.Close()

	h := buildHTTPHandler(db)

	addr := ":" + cfg.App.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("listening on", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server error", zap.Error(err))
	}

	log.Info("server stopped")
}

func buildHTTPHandler(db *pgxpool.Pool) http.Handler {
	accountRepository := postgres.NewAccountRepository(db)
	accountService := service.NewAccountService(accountRepository)
	accountHandler := handler.NewAccountHandler(accountService)

	r := transporthttp.NewRouter(accountHandler)
	return r.Mount()
}
