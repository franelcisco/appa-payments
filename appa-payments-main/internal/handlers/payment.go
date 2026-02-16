package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	"appa_payments/pkg/bcv"
)

// PaymentHandler handles payment-related HTTP requests
type PaymentHandler struct {
	Service   domains.PaymentService
	bcvClient bcv.Client
}

// NewPaymentHandler creates a new PaymentHandler
func NewPaymentHandler(service domains.PaymentService, bcvClient bcv.Client) *PaymentHandler {
	return &PaymentHandler{Service: service, bcvClient: bcvClient}
}

// GetBCVTasa handles requests to get the BCV exchange rate for USD
func (p *PaymentHandler) GetBCVTasa(c *gin.Context) {
	rate, err := p.bcvClient.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.BCVTasaUSDResponse{
		Date: time.Now().Format("2006-01-02"),
		Rate: rate,
	})
}

// HandlerGenerateOTP handles requests to generate an OTP for mobile payments
func (p *PaymentHandler) HandlerGenerateOTP(c *gin.Context) {
	var otpRequest models.OTPRequest
	if err := c.ShouldBindJSON(&otpRequest); err != nil {
		fmt.Printf("Error binding JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := p.Service.GenerateOTP(context.Background(), otpRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP generated successfully"})
}

// HandlerValidateDirectDebit handles requests to validate a direct debit transaction
func (p *PaymentHandler) HandlerValidateDirectDebit(c *gin.Context) {
	var validateRequest models.ValidateOTPRequest
	if err := c.ShouldBindJSON(&validateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := p.Service.ValidateDirectDebit(context.Background(), validateRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Direct debit validated successfully"})
}

// HandleValidateMobilePayment handles mobile payment validation
func (p *PaymentHandler) HandleValidateMobilePayment(c *gin.Context) {
	var mobilePaymentRequest models.ValidateMobilePaymentRequest
	if err := c.ShouldBindJSON(&mobilePaymentRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := p.Service.ValidateMobilePayment(context.Background(), mobilePaymentRequest)

	c.JSON(http.StatusOK, resp)
}

// HandleValidateMobilePaymentManual handles mobile payment validation
func (p *PaymentHandler) HandleValidateMobilePaymentManual(c *gin.Context) {
	orderName := c.PostForm("orderName")
	if orderName == "" {
		fmt.Println("Invalid orderName")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid orderName"})
		return
	}

	orderID := c.PostForm("orderId")
	if orderID == "" {
		fmt.Println("Invalid orderId")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid orderId"})
		return
	}

	file, err := c.FormFile("billImageFile")
	if err != nil {
		fmt.Printf("Error retrieving billImageFile: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid billImageFile"})
		return
	}

	err = p.Service.ValidateMobilePaymentManual(
		context.Background(),
		models.ValidateMobilePaymentManualRequest{
			OrderName:     orderName,
			OrderID:       orderID,
			BillImageFile: file,
		})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Zelle payment validated successfully"})
}
