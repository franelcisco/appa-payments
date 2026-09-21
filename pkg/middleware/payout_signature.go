package middleware

import (
	"crypto/hmac"
	"fmt"
	"time"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	helpers "appa_payments/pkg"
)

// payoutMaxTTL caps how far ahead a signature may expire, so a leaked one is
// useless within minutes even if the caller asked for longer.
const payoutMaxTTL = 5 * time.Minute

type payoutVerifier struct {
	secret string
}

// NewPayoutVerifier guards POST /payouts/vuelto. Every other payment route here
// is public because the worst a stranger can do is ask about a payment. This one
// SENDS money, so it fails closed: no secret configured, no payouts.
func NewPayoutVerifier(secret string) domains.PayoutVerifier {
	return &payoutVerifier{secret: secret}
}

// PayoutMessage is what gets signed: the whole instruction, not just the id, so
// a captured signature can't be reused to pay someone else or another amount.
// The handler only lets through ids, banks, phones and documents that cannot
// contain ':', so the fields can't bleed into each other; the free-text concept
// goes last, where a ':' inside it changes nothing.
func PayoutMessage(req models.VueltoPayoutRequest, exp int64) string {
	return fmt.Sprintf("%s:%s:%s:%s:%.2f:%d:%s", req.PayoutID, req.Bank, req.Phone, req.DNI, req.Amount, exp, req.Concept)
}

func (v *payoutVerifier) Verify(sig models.PayoutSignature, req models.VueltoPayoutRequest, now time.Time) error {
	if v.secret == "" {
		return domains.ErrPayoutNoSecret
	}
	if sig.Signature == "" || sig.Exp == 0 {
		return domains.ErrPayoutUnsigned
	}

	expected := helpers.GenerateAuthToken(v.secret, PayoutMessage(req, sig.Exp))
	if !hmac.Equal([]byte(expected), []byte(sig.Signature)) {
		return domains.ErrPayoutInvalid
	}

	exp := time.Unix(sig.Exp, 0)
	if !now.Before(exp) || exp.After(now.Add(payoutMaxTTL)) {
		return domains.ErrPayoutExpired
	}
	return nil
}
