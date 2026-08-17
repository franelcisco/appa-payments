package services

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	dbModels "appa_payments/pkg/db/models"
)

type webhookService struct {
	paymentService domains.PaymentService
	db             *gorm.DB
	logger         *zap.Logger
}

func NewWebhookService(
	paymentService domains.PaymentService,
	db *gorm.DB,
	logger *zap.Logger,
) domains.WebhookService {
	return &webhookService{
		paymentService: paymentService,
		db:             db,
		logger:         logger,
	}
}

// OrdersCreated handles a Shopify orders/create webhook delivery. It checks the
// database for a prior successful charge on the same order (dedup) and, if
// none, delegates to the payment service. The payment service is the single
// source of truth for the recurrent-app gate, the affiliation gate, and the
// OTP-bypass behaviour, so no order data is fetched here.
//
// Charge failures are persisted by the payment service and do not propagate as
// errors — the webhook always succeeds so Shopify does not retry. A declined
// (not erroring) charge is also recorded as a pending retry so the daily
// recurrent-retry job (see internal/services/recurrent_retry.go) can follow up.
func (s *webhookService) OrdersCreated(ctx context.Context, orderID string) error {
	alreadyCharged, err := s.paymentService.HasSuccessfulRecurrentCharge(ctx, orderID)
	if err != nil {
		s.logger.Error("webhook: dedup check failed",
			zap.String("orderID", orderID),
			zap.Error(err))
		return nil
	}
	if alreadyCharged {
		s.logger.Info("webhook: order already charged successfully, skipping",
			zap.String("orderID", orderID))
		return nil
	}

	chargeResp, err := s.paymentService.DirectDebitAccountWithOTP(ctx, models.DirectDebitAccountWithOTPRequest{
		OrderID: orderID,
		OTP:     "",
	})
	if err != nil {
		s.logger.Error("webhook: recurrent charge errored",
			zap.String("orderID", orderID),
			zap.Error(err))
		return nil
	}

	s.logger.Info("webhook: recurrent charge completed",
		zap.String("orderID", orderID),
		zap.Bool("success", chargeResp.Success),
		zap.String("code", chargeResp.Code))

	if !chargeResp.Success {
		if err := s.savePendingRetry(ctx, orderID, chargeResp.OrderName); err != nil {
			s.logger.Error("webhook: failed to save pending recurrent retry",
				zap.String("orderID", orderID),
				zap.Error(err))
		}
	}

	return nil
}

// savePendingRetry records a declined recurrent charge for the daily retry job.
// ON CONFLICT DO NOTHING means repeated declines for the same order do not
// duplicate rows or reset the retry window.
func (s *webhookService) savePendingRetry(ctx context.Context, orderID, orderName string) error {
	record := &dbModels.RecurrentPendingPayment{
		OrderID:   orderID,
		OrderName: orderName,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		DoNothing: true,
	}).Create(record).Error
}
