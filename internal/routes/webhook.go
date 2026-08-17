package routes

import (
	"appa_payments/internal/handlers"
	"appa_payments/pkg/middleware"

	"github.com/gin-gonic/gin"
)

// WebhookRoutes defines the routes for the webhook service
type WebhookRoutes struct {
	handler *handlers.WebhookHandler
}

// NewWebhookRoutes creates a new instance of WebhookRoutes
func NewWebhookRoutes(
	handler *handlers.WebhookHandler,
) *WebhookRoutes {
	return &WebhookRoutes{
		handler: handler,
	}
}

const shopifyHMACHeader = "X-Shopify-Hmac-Sha256"

// SetRouter sets up the routes for the webhook service
func (r *WebhookRoutes) SetRouter(router *gin.Engine, secretKey string) {
	router.POST("/webhook/order/created", middleware.ValidateHMAC(secretKey, shopifyHMACHeader), r.handler.HandleOrdersCreated)
}
