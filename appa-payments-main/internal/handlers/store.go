package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
)

// StoreHandler handles store-related HTTP requests
type StoreHandler struct {
	Service domains.StoreService
}

// NewStoreHandler creates a new StoreHandler
func NewStoreHandler(service domains.StoreService) *StoreHandler {
	return &StoreHandler{Service: service}
}

// GetOrderByID handles requests to get an order by its ID
func (s *StoreHandler) GetOrderByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	order, err := s.Service.GetOrderByID(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetOrderByName handles requests to get an order by its name
func (s *StoreHandler) GetOrderByName(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order name is required"})
		return
	}

	order, err := s.Service.GetOrderByName(context.Background(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

// HandleUpdateCustomerParentID handles requests to update a customer's parent ID
func (s *StoreHandler) HandleUpdateCustomerParentID(c *gin.Context) {
	var req models.UpdateCustomerParentIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.Service.UpdateCustomerParentID(context.Background(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
