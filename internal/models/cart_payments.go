package models

type SignedCartQuote struct {
	CartID    string `header:"X-Cart-Id" binding:"required"`
	Amount    string `header:"X-Cart-Amount" binding:"required"`
	Exp       int64  `header:"X-Cart-Exp" binding:"required"`
	Signature string `header:"X-Cart-Signature" binding:"required"`
}

type CartQuote struct {
	CartID string
	Amount float64
}

type CartOTPRequest struct {
	Bank    string `json:"bank"`
	Phone   string `json:"phone"`
	DNI     string `json:"dni"`
	DNIType string `json:"dniType"`
}

type CartValidateOTPRequest struct {
	Bank    string `json:"bank"`
	Phone   string `json:"phone"`
	DNI     string `json:"dni"`
	DNIType string `json:"dniType"`
	Name    string `json:"name"`
	OTP     string `json:"otp"`
	Concept string `json:"concept"`
}

type CartDirectDebitResult struct {
	Success   bool   `json:"success"`
	Code      string `json:"code"`
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

// CartAttachOrderRequest backfills the Shopify order a cart-keyed charge
// becomes, once create-order-from-cart has minted it.
type CartAttachOrderRequest struct {
	Reference     string `json:"reference" binding:"required"`
	OrderID       string `json:"orderId" binding:"required"`
	OrderName     string `json:"orderName"`
	ClientID      string `json:"clientId"`
	PaymentMethod string `json:"paymentMethod" binding:"required,oneof=direct_debit mobile_payment direct_debit_account"`
}

type CartValidateMobilePaymentRequest struct {
	Bank      string `json:"bank"`
	Phone     string `json:"phone"`
	Reference string `json:"reference"`
	Date      string `json:"date"`
	DNI       string `json:"dni"`
	DNIType   string `json:"dniType"`
	Automatic bool   `json:"automatic"`
}

type CartMobilePaymentResult struct {
	Success   bool   `json:"success"`
	Code      string `json:"code"`
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

type CartDirectDebitAccountRequest struct {
	DNI     string `json:"dni"     binding:"required"`
	Account string `json:"account" binding:"required,min=20,max=20"`
	Name    string `json:"name"`
}

type CartDirectDebitAccountResult struct {
	Success   bool   `json:"success"`
	Code      string `json:"code"`
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

// CartDirectDebitAccountOTPRequest asks for the OTP that authorizes a
// recurring domiciliación charge on a cart. The customer must already be
// affiliated (metafield set from a prior order/draft/cart charge) — ClientID
// is how we find that affiliation with no order to hang it off yet.
type CartDirectDebitAccountOTPRequest struct {
	ClientID string `json:"clientId" binding:"required"`
}

// CartValidateDirectDebitAccountOTPRequest confirms the OTP and charges the
// account already on file for ClientID.
type CartValidateDirectDebitAccountOTPRequest struct {
	ClientID string `json:"clientId" binding:"required"`
	OTP      string `json:"otp"      binding:"required"`
}
