package domains

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
