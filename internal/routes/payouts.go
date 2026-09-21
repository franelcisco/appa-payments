package routes

import (
	"github.com/gin-gonic/gin"

	"appa_payments/internal/handlers"
)

type PayoutRoute struct {
	Handler *handlers.PayoutHandler
}

func NewPayoutRoute(handler *handlers.PayoutHandler) *PayoutRoute {
	return &PayoutRoute{Handler: handler}
}

// SetRouter registers the only route here that sends money OUT. The handler
// verifies a signature over the whole instruction; see pkg/middleware.
func (p *PayoutRoute) SetRouter(router gin.IRoutes) {
	router.POST("/payouts/vuelto", p.Handler.HandleVuelto)
}
