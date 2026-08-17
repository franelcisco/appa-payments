package routes

import (
	"github.com/gin-gonic/gin"

	"appa_payments/internal/domains"
	"appa_payments/internal/handlers"
)

type CartPaymentRoutes struct {
	Handler   *handlers.CartPaymentHandler
	CartQuote domains.CartQuoteRepository
}

// NewCartPaymentRoutes creates a new instance of CartPaymentRoutes
func NewCartPaymentRoutes(
	handler *handlers.CartPaymentHandler,
	cartQuote domains.CartQuoteRepository,
) *CartPaymentRoutes {
	return &CartPaymentRoutes{Handler: handler, CartQuote: cartQuote}
}

// SetRouter sets up the cart payment-related routes.
func (c *CartPaymentRoutes) SetRouter(router *gin.Engine) {
	cartRouter := router.Group("/cart-payments", c.CartQuote.Handler())
	{
		cartRouter.POST("/generate-otp", c.Handler.HandlerGenerateOTP)
		cartRouter.POST("/validate-direct-debit", c.Handler.HandlerValidateDirectDebit)
		cartRouter.POST("/validate-mobile-payment", c.Handler.HandlerValidateMobilePayment)
		cartRouter.POST("/attach-order", c.Handler.HandlerAttachOrder)
		cartRouter.POST("/direct-debit-account", c.Handler.HandlerDirectDebitAccount)
		cartRouter.POST("/direct-debit-account/request-otp", c.Handler.HandlerRequestDirectDebitAccountOTP)
		cartRouter.POST("/direct-debit-account/otp", c.Handler.HandlerValidateDirectDebitAccountOTP)
	}
}
