package services

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	dbModels "appa_payments/pkg/db/models"
)

const _shopifyPendingStatus = "PENDING"

// recurrentRetryWorkers bounds how many charges run concurrently — each R4
// charge call takes close to a minute, so this keeps a large backlog from
// stretching the daily job for hours while not hammering R4 with unbounded
// concurrent requests. Mirrors webhookWorkers in internal/handlers/webhook.go.
const recurrentRetryWorkers = 4

// RecurrentRetryService retries recurrent direct-debit charges that were
// declined at webhook time, per docs/superpowers/specs/2026-07-08-recurrent-directdebit-retry-design.md.
type RecurrentRetryService struct {
	db             *gorm.DB
	paymentService domains.PaymentService
	storeService   domains.StoreService
	location       *time.Location
	logger         *zap.Logger
}

func NewRecurrentRetryService(
	db *gorm.DB,
	paymentService domains.PaymentService,
	storeService domains.StoreService,
	location *time.Location,
	logger *zap.Logger,
) *RecurrentRetryService {
	return &RecurrentRetryService{
		db:             db,
		paymentService: paymentService,
		storeService:   storeService,
		location:       location,
		logger:         logger,
	}
}

// RetryPendingCharges loads every pending recurrent direct-debit charge and
// retries each one across a bounded worker pool (recurrentRetryWorkers). Only
// orders still PENDING in Shopify get charged; any other status (PAID,
// CANCELLED, REFUNDED, etc.) deletes the record without charging. A charge
// success also deletes the record — there is no give-up window, retries
// continue indefinitely while the order stays pending. Blocks until every
// record has been processed. Meant to be
// invoked by the daily cron job (internal/jobs).
func (s *RecurrentRetryService) RetryPendingCharges(ctx context.Context) {
	var pending []dbModels.RecurrentPendingPayment
	if err := s.db.WithContext(ctx).Find(&pending).Error; err != nil {
		s.logger.Error("recurrent retry: failed to load pending payments", zap.Error(err))
		return
	}

	now := time.Now().In(s.location)

	jobs := make(chan dbModels.RecurrentPendingPayment)
	var wg sync.WaitGroup
	for range recurrentRetryWorkers {
		wg.Go(func() {
			for record := range jobs {
				s.retryOneSafe(ctx, record, now)
			}
		})
	}

	for _, record := range pending {
		jobs <- record
	}
	close(jobs)
	wg.Wait()
}

// retryOneSafe runs retryOne with panic recovery so one bad record can't take
// down its worker goroutine and strand the rest of the queue.
func (s *RecurrentRetryService) retryOneSafe(ctx context.Context, record dbModels.RecurrentPendingPayment, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("recurrent retry: worker panicked",
				zap.String("orderID", record.OrderID),
				zap.Any("panic", r))
		}
	}()
	s.retryOne(ctx, record, now)
}

func (s *RecurrentRetryService) retryOne(ctx context.Context, record dbModels.RecurrentPendingPayment, now time.Time) {
	logger := s.logger.With(
		zap.String("orderID", record.OrderID),
		zap.String("orderName", record.OrderName),
	)

	order, err := s.storeService.GetOrderByID(ctx, record.OrderID)
	if err != nil {
		logger.Error("recurrent retry: failed to fetch order from Shopify, will retry next day", zap.Error(err))
		return
	}

	// Only PENDING orders are chargeable. PAID, CANCELLED, REFUNDED, etc. are
	// all resolved-or-dead states — stop retrying and drop the record.
	if order.DisplayFinancialStatus != _shopifyPendingStatus {
		if err := s.deletePending(ctx, record.OrderID); err != nil {
			logger.Error("recurrent retry: failed to delete non-pending order's record", zap.Error(err))
			return
		}
		logger.Info("recurrent retry: order no longer pending, deleted pending payment",
			zap.String("financialStatus", order.DisplayFinancialStatus))
		return
	}

	resp, err := s.paymentService.DirectDebitAccountWithOTP(ctx, models.DirectDebitAccountWithOTPRequest{
		OrderID: record.OrderID,
		OTP:     "",
	})
	if err != nil {
		logger.Error("recurrent retry: charge errored, will retry next day", zap.Error(err))
		return
	}

	if resp.Success {
		if err := s.deletePending(ctx, record.OrderID); err != nil {
			logger.Error("recurrent retry: failed to delete resolved pending payment", zap.Error(err))
			return
		}
		logger.Info("recurrent retry: charge succeeded, deleted pending payment")
		return
	}

	if err := s.bumpAttempt(ctx, record.OrderID, now); err != nil {
		logger.Error("recurrent retry: failed to bump attempt count", zap.Error(err))
	}
	logger.Info("recurrent retry: charge declined, will retry next day", zap.String("code", resp.Code))
}

func (s *RecurrentRetryService) deletePending(ctx context.Context, orderID string) error {
	return s.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Delete(&dbModels.RecurrentPendingPayment{}).Error
}

func (s *RecurrentRetryService) bumpAttempt(ctx context.Context, orderID string, now time.Time) error {
	return s.db.WithContext(ctx).
		Model(&dbModels.RecurrentPendingPayment{}).
		Where("order_id = ?", orderID).
		Updates(map[string]any{
			"attempts":        gorm.Expr("attempts + 1"),
			"last_attempt_at": now,
			"updated_at":      now,
		}).Error
}
