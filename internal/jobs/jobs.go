package jobs

import (
	"context"

	"go.uber.org/zap"

	"appa_payments/internal/services"
)

// JobHandler wraps scheduled background jobs for cron registration.
type JobHandler struct {
	recurrentRetryService *services.RecurrentRetryService
	logger                *zap.Logger
}

func NewJobHandler(
	recurrentRetryService *services.RecurrentRetryService,
	logger *zap.Logger,
) *JobHandler {
	return &JobHandler{
		recurrentRetryService: recurrentRetryService,
		logger:                logger,
	}
}

// HandleRetryPendingRecurrentCharges runs the daily retry of pending recurrent
// direct-debit charges. Signature matches robfig/cron/v3's AddFunc (func()).
func (h *JobHandler) HandleRetryPendingRecurrentCharges() {
	h.logger.Info("jobs: starting recurrent pending charges retry")
	h.recurrentRetryService.RetryPendingCharges(context.Background())
	h.logger.Info("jobs: finished recurrent pending charges retry")
}
