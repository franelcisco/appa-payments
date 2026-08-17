package routes

import (
	"appa_payments/internal/handlers"

	"github.com/gin-gonic/gin"
)

// StoreRoute defines the routes for the store
type StoreRoute struct {
	Handler *handlers.StoreHandler
}

// NewStoreRoute creates a new StoreRoute
func NewStoreRoute(handler *handlers.StoreHandler) *StoreRoute {
	return &StoreRoute{Handler: handler}
}

// SetRouter sets up the routes for the store
func (s *StoreRoute) SetRouter(router *gin.Engine) {
	router.GET("/orders/:id", s.Handler.GetOrderByID)
	router.GET("/orders/confirmation/:name", s.Handler.GetOrderByName)
	router.PUT("/customers/parent", s.Handler.HandleUpdateCustomerParentID)
}
