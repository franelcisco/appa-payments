package models

import (
	"mime/multipart"
)

type BCVTasaUSDResponse struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

// OrderType tells a /payments/* endpoint what kind of Shopify object the id
// refers to. Nil is treated as OrderTypeComplete.
type OrderType string

const (
	OrderTypeComplete OrderType = "Complete"
	OrderTypeDraft    OrderType = "Draft"
	// OrderTypeCart is reserved. No endpoint implements it yet.
	OrderTypeCart OrderType = "Cart"
)

func OrderTypeOrDefault(t *OrderType) OrderType {
	if t == nil || *t == "" {
		return OrderTypeComplete
	}
	return *t
}

// MobilePayValidationRequest para pagos por pago móvil
type OTPRequest struct {
	Bank      string     `json:"bank"`
	Amount    string     `json:"amount"`
	Phone     string     `json:"phone"`
	DNI       string     `json:"dni"`
	DNIType   string     `json:"dniType"`
	OrderID   string     `json:"orderId"`
	TypeOrder *OrderType `json:"typeOrder,omitempty"`
}

type ValidateOTPRequest struct {
	Bank      string     `json:"bank"`
	Amount    string     `json:"amount"`
	Phone     string     `json:"phone"`
	DNI       string     `json:"dni"`
	DNIType   string     `json:"dniType"`
	Name      string     `json:"name"`
	OTP       string     `json:"otp"`
	Concept   string     `json:"concept"`
	OrderID   string     `json:"orderId"`
	OrderName string     `json:"orderName"`
	TypeOrder *OrderType `json:"typeOrder,omitempty"`
}

// ValidateCash para pagos en efectivo
type ValidateCash struct {
	Amount         float64               `json:"amount"`
	RequiresChange bool                  `json:"requiresChange"`
	BillImageFile  *multipart.FileHeader `json:"billImageFile"`
	OrderID        string                `json:"orderId"`
	OrderName      string                `json:"orderName"`
	ReturnData     *CashReturnData       `json:"returnData,omitempty"`
}

// CashReturnData representa los datos necesarios para la devolución en efectivo
type CashReturnData struct {
	Bank    string `json:"bank"`
	Phone   string `json:"phone"`
	DNI     string `json:"dni"`
	DNIType string `json:"dniType"`
}

type ValidateZelle struct {
	BillImageFile *multipart.FileHeader `json:"billImageFile"`
	OrderID       string                `json:"orderId"`
	OrderName     string                `json:"orderName"`
}

type ValidateMobilePaymentRequest struct {
	Bank      string     `json:"bank"`
	Phone     string     `json:"phone"`
	Reference string     `json:"reference"`
	Date      string     `json:"date"`
	DNI       string     `json:"dni"`
	DNIType   string     `json:"dniType"`
	Automatic bool       `json:"automatic"`
	OrderID   string     `json:"orderId"`
	OrderName string     `json:"orderName"`
	TypeOrder *OrderType `json:"typeOrder,omitempty"`
}

type MobilePaymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type ValidateMobilePaymentManualRequest struct {
	BillImageFile *multipart.FileHeader `json:"billImageFile"`
	OrderID       string                `json:"orderId"`
	OrderName     string                `json:"orderName"`
	TypeOrder     *OrderType            `json:"typeOrder,omitempty"`
}

type DirectDebitAccountRequest struct {
	DNI       string     `json:"dni"      binding:"required"`
	OrderID   string     `json:"orderId"  binding:"required"`
	Account   string     `json:"account"  binding:"required,min=20,max=20"`
	TypeOrder *OrderType `json:"typeOrder,omitempty"`
}

type DirectDebitAccountWithOTPRequest struct {
	OrderID   string     `json:"orderId" binding:"required"`
	OTP       string     `json:"otp"     binding:"omitempty"`
	TypeOrder *OrderType `json:"typeOrder,omitempty"`
}

type ProcessDirectDebitAccountResponse struct {
	Success   bool   `json:"success"`
	Code      string `json:"code,omitempty"`
	Reference string `json:"reference,omitempty"`
	OrderName string `json:"-"`
}

// DirectDebitAccount is the json payload stored in the customer metafield
type DirectDebitAccount struct {
	Account string `json:"account"`
	DNI     string `json:"dni"`
}
