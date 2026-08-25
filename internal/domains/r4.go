package domains

import "errors"

const (
	R4CodeApproved   = "ACCP"
	R4CodeInProgress = "AC00"
	R4CodeInPending  = "11"

	R4CodeInsufficientFunds     = "AM04"
	R4CodeAffiliationRequested  = "MD01"
	R4CodeAffiliationNotAcepted = "MD09"
	R4CodeInvalidAccountNumber  = "AC01"
)

// IsR4BreakCode returns true if the code is one that indicates the payment is still in progress.
func IsR4BreakCode(code string) bool {
	return code == R4CodeInProgress || code == R4CodeInPending
}

// R4APIError is the error body the r4-service returns for the endpoints that
// move value. OperationID is omitted when the operation never got far enough to
// have one, and always for change-paid, which has none.
type R4APIError struct {
	Code        string
	OperationID string
	Detail      string
}

func (e *R4APIError) Error() string { return e.Detail }

// R4errorResponse is the raw shape of that body. Endpoints that move no value
// send only Error.
type R4errorResponse struct {
	Error       string `json:"error"`
	Code        string `json:"code"`
	OperationID string `json:"operation_id"`
}

// R4OperationIDFrom returns the r4-service operation id carried by err, if any.
func R4OperationIDFrom(err error) string {
	var apiErr *R4APIError
	if errors.As(err, &apiErr) {
		return apiErr.OperationID
	}
	return ""
}

// R4CodeFrom returns the r4-service error code carried by err, if any.
func R4CodeFrom(err error) string {
	var apiErr *R4APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}
