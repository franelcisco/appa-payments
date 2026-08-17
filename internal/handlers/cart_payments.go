package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	"appa_payments/pkg/bcv"
	"appa_payments/pkg/middleware"
)

type CartPaymentHandler struct {
	Service   domains.CartPaymentService
	bcvClient bcv.Client
}

// NewCartPaymentHandler creates a new CartPaymentHandler
func NewCartPaymentHandler(service domains.CartPaymentService, bcvClient bcv.Client) *CartPaymentHandler {
	return &CartPaymentHandler{Service: service, bcvClient: bcvClient}
}

// HandlerAttachOrder backfills the Shopify order id/name a cart-keyed débito
// became, once the checkout has minted it.
func (h *CartPaymentHandler) HandlerAttachOrder(c *gin.Context) {
	var req models.CartAttachOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Service.AttachOrder(c.Request.Context(), middleware.CartQuoteFrom(c).CartID, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandlerGenerateOTP handles requests to generate an OTP for cart payments
func (h *CartPaymentHandler) HandlerGenerateOTP(c *gin.Context) {
	var otpRequest models.CartOTPRequest
	if err := c.ShouldBindJSON(&otpRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.Service.GenerateOTP(c.Request.Context(), middleware.CartQuoteFrom(c), otpRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP generated successfully"})
}

// HandlerValidateDirectDebit handles requests to validate direct debit for cart payments
func (h *CartPaymentHandler) HandlerValidateDirectDebit(c *gin.Context) {
	var validateRequest models.CartValidateOTPRequest
	if err := c.ShouldBindJSON(&validateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.Service.ValidateDirectDebit(c.Request.Context(), middleware.CartQuoteFrom(c), validateRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandlerValidateMobilePayment handles requests to validate mobile payment for cart payments
func (h *CartPaymentHandler) HandlerValidateMobilePayment(c *gin.Context) {
	var validateRequest models.CartValidateMobilePaymentRequest
	if err := c.ShouldBindJSON(&validateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.Service.ValidateMobilePayment(c.Request.Context(), middleware.CartQuoteFrom(c), validateRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandlerDirectDebitAccount handles requests to process direct debit account for cart payments
func (h *CartPaymentHandler) HandlerDirectDebitAccount(c *gin.Context) {
	var req models.CartDirectDebitAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.Service.DirectDebitAccount(c.Request.Context(), middleware.CartQuoteFrom(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandlerRequestDirectDebitAccountOTP handles requests to mail the OTP that
// authorizes a recurring domiciliación charge on an already-affiliated cart.
func (h *CartPaymentHandler) HandlerRequestDirectDebitAccountOTP(c *gin.Context) {
	var req models.CartDirectDebitAccountOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Service.RequestDirectDebitAccountOTP(c.Request.Context(), middleware.CartQuoteFrom(c), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP generated successfully"})
}

// HandlerValidateDirectDebitAccountOTP handles requests to confirm the OTP and
// charge the account already on file for a recurring domiciliación cart payment.
func (h *CartPaymentHandler) HandlerValidateDirectDebitAccountOTP(c *gin.Context) {
	var req models.CartValidateDirectDebitAccountOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.Service.ValidateDirectDebitAccountOTP(c.Request.Context(), middleware.CartQuoteFrom(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
