package domains

import "slices"

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

var directDebitAccountResponseCodes = map[string]string{
	R4CodeInsufficientFunds:     ResponseCodeInsufficientFunds,
	R4CodeAffiliationRequested:  ResponseCodeAffiliationPending,
	R4CodeAffiliationNotAcepted: ResponseCodeAffiliationRefused,
	R4CodeInvalidAccountNumber:  ResponseCodeInvalidAccount,
}

// DirectDebitAccountResponseCode maps an R4 code to ours. ok is false for a
// code with no mapping, which callers must treat as unexpected.
func DirectDebitAccountResponseCode(r4Code string) (string, bool) {
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
