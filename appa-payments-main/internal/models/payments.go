package models

import (
	"mime/multipart"
)

type BCVTasaUSDResponse struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

// MobilePayValidationRequest para pagos por pago móvil
type OTPRequest struct {
	Bank    string `json:"bank"`
	Amount  string `json:"amount"`
	Phone   string `json:"phone"`
	DNI     string `json:"dni"`
	DNIType string `json:"dniType"`
	OrderID string `json:"orderId"`
}

type ValidateOTPRequest struct {
	Bank      string `json:"bank"`
	Amount    string `json:"amount"`
	Phone     string `json:"phone"`
	DNI       string `json:"dni"`
	DNIType   string `json:"dniType"`
	Name      string `json:"name"`
	OTP       string `json:"otp"`
	Concept   string `json:"concept"`
	OrderID   string `json:"orderId"`
	OrderName string `json:"orderName"`
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
	Bank      string `json:"bank"`
	Phone     string `json:"phone"`
	Reference string `json:"reference"`
	Date      string `json:"date"`
	DNI       string `json:"dni"`
	DNIType   string `json:"dniType"`
	Automatic bool   `json:"automatic"`
	OrderID   string `json:"orderId"`
	OrderName string `json:"orderName"`
}

type MobilePaymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type ValidateMobilePaymentManualRequest struct {
	BillImageFile *multipart.FileHeader `json:"billImageFile"`
	OrderID       string                `json:"orderId"`
	OrderName     string                `json:"orderName"`
}
