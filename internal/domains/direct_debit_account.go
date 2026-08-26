package domains

import (
	"errors"
	"slices"
)

// Codes sent to the checkout, which maps them to what the buyer reads.
const (
	ResponseCodeOK                 = "OK"
	ResponseCodeAffiliationExists  = "AAF01"
	ResponseCodeInvalidOTP         = "OTP01"
	ResponseCodeInsufficientFunds  = "ERR01"
	ResponseCodeAffiliationPending = "ERR02"
	ResponseCodeAffiliationRefused = "ERR03"
	ResponseCodeInvalidAccount     = "ERR04"
)

var DirectDebitAccountGenericError = errors.New("ocurrió un error al procesar la solicitud")

// DirectDebitAccountRequest is the internal request used by the payment service
// to process a direct debit account charge (first-time or recurring).
type DirectDebitAccountRequest struct {
	Amount      float64
	Account     string
	DNI         string
	DisplayName string
	CustomerID  string
	OrderName   string
	OrderID     string
	IsRecurring bool
}

var directDebitAccountResponseCodes = map[R4Code]string{
	R4CodeInsufficientFunds:     ResponseCodeInsufficientFunds,
	R4CodeAffiliationRequested:  ResponseCodeAffiliationPending,
	R4CodeAffiliationNotAcepted: ResponseCodeAffiliationRefused,
	R4CodeInvalidAccountNumber:  ResponseCodeInvalidAccount,
}

// DirectDebitAccountResponseCode maps a response code from the R4 service to a code that can be sent to the checkout.
func DirectDebitAccountResponseCode(r4Code R4Code) (string, bool) {
	code, ok := directDebitAccountResponseCodes[r4Code]
	return code, ok
}

// IsAffiliationPending reports whether a response code means the account is
// not affiliated yet — a state the buyer can still resolve, not a refusal.
func IsAffiliationPending(responseCode string) bool {
	return slices.Contains(
		[]string{ResponseCodeAffiliationPending, ResponseCodeAffiliationRefused},
		responseCode,
	)
}
