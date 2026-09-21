package models

// VueltoPayoutRequest is the body of POST /payouts/vuelto. PayoutID is the
// caller's id for this payout (APPA sends its claim_payouts row id).
type VueltoPayoutRequest struct {
	PayoutID string  `json:"payoutId" binding:"required"`
	Bank     string  `json:"bank" binding:"required"`
	Phone    string  `json:"phone" binding:"required"`
	DNI      string  `json:"dni" binding:"required"`
	Amount   float64 `json:"amount" binding:"required,gt=0"`
	Concept  string  `json:"concept"`
}

// PayoutSignature travels in headers. Deliberately NOT in the CORS allow-list:
// no browser should ever call this route.
type PayoutSignature struct {
	Exp       int64  `header:"X-Payout-Exp"`
	Signature string `header:"X-Payout-Signature"`
}

// VueltoPayoutResponse. Outcome is confirmed | rejected | unknown. Replayed is
// true when this payout id had already been processed and R4 was NOT called.
type VueltoPayoutResponse struct {
	Outcome   string `json:"outcome"`
	Reference string `json:"reference,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Replayed  bool   `json:"replayed"`
}
