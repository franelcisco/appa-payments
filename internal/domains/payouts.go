package domains

import (
	"context"
	"errors"
	"time"

	"appa_payments/internal/models"
	dbModels "appa_payments/pkg/db/models"
)

// PayoutService sends a Vuelto exactly once per payout id.
type PayoutService interface {
	SendVuelto(ctx context.Context, req models.VueltoPayoutRequest) (models.VueltoPayoutResponse, error)
}

// PayoutStore is the idempotency record. Reserve must be atomic: of two
// concurrent calls with the same payout id, exactly one gets created == true.
type PayoutStore interface {
	Reserve(ctx context.Context, payout *dbModels.R4AppaPayout) (existing *dbModels.R4AppaPayout, created bool, err error)
	Finish(ctx context.Context, payoutID, status, reference, detail string) error
}

// PayoutVerifier checks that the request was signed by a holder of the payout
// secret, for exactly this payee and amount, and has not expired.
type PayoutVerifier interface {
	Verify(sig models.PayoutSignature, req models.VueltoPayoutRequest, now time.Time) error
}

var (
	ErrPayoutNoSecret = errors.New("payout secret not configured")
	ErrPayoutUnsigned = errors.New("payout signature missing")
	ErrPayoutInvalid  = errors.New("payout signature does not verify")
	ErrPayoutExpired  = errors.New("payout signature expired")
	// ErrPayoutMismatch: the payout id was already used for a different payee or
	// amount. Never pay, never replay.
	ErrPayoutMismatch = errors.New("payout id already used with different details")
	// ErrPayoutConflictUnreadable: the id is already reserved (an earlier request
	// owns it and may be at R4 right now) but its row could not be read.
	ErrPayoutConflictUnreadable = errors.New("payout id already reserved; its record could not be read")
)
