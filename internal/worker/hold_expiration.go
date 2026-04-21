package worker

import (
	"context"
	"time"

	"go.uber.org/zap"
)

const (
	defaultHoldExpirationInterval = 10 * time.Second
	defaultHoldExpirationBatch    = 100
)

type HoldExpirer interface {
	ExpirePending(ctx context.Context, limit int) (int, error)
}

type HoldExpirationWorker struct {
	holds     HoldExpirer
	interval  time.Duration
	batchSize int
	logger    *zap.Logger
}

func NewHoldExpirationWorker(
	holds HoldExpirer,
	interval time.Duration,
	batchSize int,
	logger *zap.Logger,
) *HoldExpirationWorker {
	if interval <= 0 {
		interval = defaultHoldExpirationInterval
	}
	if batchSize <= 0 {
		batchSize = defaultHoldExpirationBatch
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &HoldExpirationWorker{
		holds:     holds,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger,
	}
}

func (worker *HoldExpirationWorker) Run(ctx context.Context) {
	worker.logger.Info("hold expiration worker started",
		zap.Duration("interval", worker.interval),
		zap.Int("batch_size", worker.batchSize),
	)
	defer worker.logger.Info("hold expiration worker stopped")

	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired, err := worker.holds.ExpirePending(ctx, worker.batchSize)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				worker.logger.Error("hold expiration tick failed", zap.Error(err))
				continue
			}
			if expired > 0 {
				worker.logger.Info("expired pending holds", zap.Int("count", expired))
			}
		}
	}
}
