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
	RequestDirectDebitAccountOTP(ctx context.Context, orderID string, typeOrder *models.OrderType) error
	DirectDebitAccount(ctx context.Context, req models.DirectDebitAccountRequest) (*models.ProcessDirectDebitAccountResponse, error)
	DirectDebitAccountWithOTP(ctx context.Context, req models.DirectDebitAccountWithOTPRequest) (*models.ProcessDirectDebitAccountResponse, error)
	HasSuccessfulRecurrentCharge(ctx context.Context, orderID string) (bool, error)
}

// CartPaymentService defines methods for cart payment processing
type CartPaymentService interface {
	GenerateOTP(ctx context.Context, quote models.CartQuote, req models.CartOTPRequest) error
	ValidateDirectDebit(ctx context.Context, quote models.CartQuote, req models.CartValidateOTPRequest) (*models.CartDirectDebitResult, error)
	ValidateMobilePayment(ctx context.Context, quote models.CartQuote, req models.CartValidateMobilePaymentRequest) (*models.CartMobilePaymentResult, error)
	DirectDebitAccount(ctx context.Context, quote models.CartQuote, req models.CartDirectDebitAccountRequest) (*models.CartDirectDebitAccountResult, error)
	RequestDirectDebitAccountOTP(ctx context.Context, quote models.CartQuote, req models.CartDirectDebitAccountOTPRequest) error
	ValidateDirectDebitAccountOTP(ctx context.Context, quote models.CartQuote, req models.CartValidateDirectDebitAccountOTPRequest) (*models.CartDirectDebitAccountResult, error)
	AttachOrder(ctx context.Context, cartID string, req models.CartAttachOrderRequest) error
}
