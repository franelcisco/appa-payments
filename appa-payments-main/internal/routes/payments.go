package routes

import (
	"github.com/gin-gonic/gin"

	"appa_payments/internal/handlers"
)

// PaymentRoutes struct to hold payment-related routes
type PaymentRoute struct {
	Handler *handlers.PaymentHandler
}

// NewPaymentRoutes creates a new instance of PaymentRoutes
func NewPaymentRoute(handler *handlers.PaymentHandler) *PaymentRoute {
	return &PaymentRoute{Handler: handler}
}

// SetRouter sets up the payment-related routes
func (p *PaymentRoute) SetRouter(router gin.IRoutes) {
	router.GET("/payments/bcv-tasa", p.Handler.GetBCVTasa)
	router.POST("/payments/generate-otp", p.Handler.HandlerGenerateOTP)
	router.POST("/payments/validate-direct-debit", p.Handler.HandlerValidateDirectDebit)
	router.POST("/payments/validate-mobile-payment", p.Handler.HandleValidateMobilePayment)
	router.POST("/payments/validate-mobile-payment-manual", p.Handler.HandleValidateMobilePaymentManual)
}
