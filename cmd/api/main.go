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
	"github.com/signaturekey/billy/internal/worker"
	"go.uber.org/zap"
)

const (
	holdExpirationInterval = 10 * time.Second
	holdExpirationBatch    = 100
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	startupCtx := context.Background()

	cfg := config.MustLoad()

	log := logger.New(cfg.App.Env)

	db, err := postgrespkg.New(startupCtx, cfg.Database)
	if err != nil {
		log.Error("postgres startup failed", zap.Error(err))
		_ = log.Sync()
		os.Exit(1)
	}
	defer db.Close()

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	h, holdExpirer := buildHTTPHandler(db, cfg.App.HoldTTL, log)
	holdExpirationWorker := worker.NewHoldExpirationWorker(
		holdExpirer,
		holdExpirationInterval,
		holdExpirationBatch,
		log,
	)
	go holdExpirationWorker.Run(appCtx)

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

	<-appCtx.Done()
	stop()

	log.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server error", zap.Error(err))
	}

	log.Info("server stopped")
}

func buildHTTPHandler(db *pgxpool.Pool, ttl time.Duration, log *zap.Logger) (http.Handler, worker.HoldExpirer) {
	txManager := postgres.NewTxManager(db)
	idempotencyRepository := postgres.NewIdempotencyRepository()
	idempotencyExecutor := service.NewIdempotencyExecutor(txManager, idempotencyRepository, 0)

	accountRepository := postgres.NewAccountRepository(db)
	ledgerRepository := postgres.NewLedgerRepository(db)
	accountService := service.NewAccountService(txManager, accountRepository, ledgerRepository)
	accountHandler := handler.NewAccountHandler(accountService, idempotencyExecutor)

	transferRepository := postgres.NewTransferRepository()
	transferService := service.NewTransferService(txManager, accountRepository, transferRepository, ledgerRepository)
	transferHandler := handler.NewTransferHandler(transferService, idempotencyExecutor)

	holdRepository := postgres.NewHoldRepository(db)
	holdService := service.NewHoldService(
		txManager,
		accountRepository,
		holdRepository,
		ledgerRepository,
		ttl,
	)
	holdHandler := handler.NewHoldHandler(holdService, idempotencyExecutor)

	r := transporthttp.NewRouter(
		accountHandler,
		transferHandler,
		holdHandler,
		log,
	)
	return r.Mount(), holdService
}
