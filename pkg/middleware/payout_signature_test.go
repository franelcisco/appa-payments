package middleware

import (
	"errors"
	"testing"
	"time"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	helpers "appa_payments/pkg"
)

func TestPayoutVerifier(t *testing.T) {
	const secret = "s3cret"
	now := time.Unix(1_800_000_000, 0)
	req := models.VueltoPayoutRequest{PayoutID: "p-1", Bank: "0102", Phone: "04141234567", DNI: "V12345678", Amount: 4520.5, Concept: "Reembolso APPA"}
	sign := func(r models.VueltoPayoutRequest, exp int64) models.PayoutSignature {
		return models.PayoutSignature{Exp: exp, Signature: helpers.GenerateAuthToken(secret, PayoutMessage(r, exp))}
	}
	exp := now.Add(2 * time.Minute).Unix()

	if got := PayoutMessage(req, 99); got != "p-1:0102:04141234567:V12345678:4520.50:99:Reembolso APPA" {
		t.Fatalf("message format changed (APPA signs this exact string): %q", got)
	}

	tampered := func(mut func(*models.VueltoPayoutRequest)) models.VueltoPayoutRequest {
		r := req
		mut(&r)
		return r
	}

	cases := []struct {
		name    string
		secret  string
		sig     models.PayoutSignature
		req     models.VueltoPayoutRequest
		wantErr error
	}{
		{"valid", secret, sign(req, exp), req, nil},
		{"no secret configured", "", sign(req, exp), req, domains.ErrPayoutNoSecret},
		{"missing signature", secret, models.PayoutSignature{Exp: exp}, req, domains.ErrPayoutUnsigned},
		{"wrong secret", "other", sign(req, exp), req, domains.ErrPayoutInvalid},
		{"amount changed after signing", secret, sign(req, exp), tampered(func(r *models.VueltoPayoutRequest) { r.Amount = 99999 }), domains.ErrPayoutInvalid},
		{"phone changed after signing", secret, sign(req, exp), tampered(func(r *models.VueltoPayoutRequest) { r.Phone = "04249999999" }), domains.ErrPayoutInvalid},
		{"dni changed after signing", secret, sign(req, exp), tampered(func(r *models.VueltoPayoutRequest) { r.DNI = "V1" }), domains.ErrPayoutInvalid},
		{"concept changed after signing", secret, sign(req, exp), tampered(func(r *models.VueltoPayoutRequest) { r.Concept = "otra cosa" }), domains.ErrPayoutInvalid},
		{"payout id changed after signing", secret, sign(req, exp), tampered(func(r *models.VueltoPayoutRequest) { r.PayoutID = "p-2" }), domains.ErrPayoutInvalid},
		{"expired", secret, sign(req, now.Add(-time.Second).Unix()), req, domains.ErrPayoutExpired},
		{"expiry too far ahead", secret, sign(req, now.Add(time.Hour).Unix()), req, domains.ErrPayoutExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewPayoutVerifier(tc.secret).Verify(tc.sig, tc.req, now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}
