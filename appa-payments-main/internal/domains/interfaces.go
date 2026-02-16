package domains

import (
	"context"

	"appa_payments/internal/models"
)

// StoreService defines methods for order persistence
// Implement in infrastructure layer if needed
type StoreService interface {
	GetOrderByID(ctx context.Context, id string) (*models.OrderResponse, error)
	GetOrderByName(ctx context.Context, name string) (*models.OrderResponse, error)
	UpdateCustomerParentID(ctx context.Context, req models.UpdateCustomerParentIDRequest) error
}

// PaymentService defines payment validation logic
type PaymentService interface {
	GenerateOTP(ctx context.Context, req models.OTPRequest) error
	ValidateDirectDebit(ctx context.Context, req models.ValidateOTPRequest) error
	ValidateMobilePayment(ctx context.Context, req models.ValidateMobilePaymentRequest) *models.MobilePaymentResponse
	ValidateMobilePaymentManual(ctx context.Context, req models.ValidateMobilePaymentManualRequest) error
}
