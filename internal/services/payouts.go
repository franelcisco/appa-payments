package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	dbModels "appa_payments/pkg/db/models"
	"appa_payments/pkg/r4bank"
)

type payoutService struct {
	store  domains.PayoutStore
	r4Repo r4bank.R4Repository
	logger *zap.Logger
}

// NewPayoutService sends Vueltos for callers that pay people out (APPA claims),
// as opposed to the refund Vueltos the payment flows send on their own.
func NewPayoutService(store domains.PayoutStore, r4Repo r4bank.R4Repository, logger *zap.Logger) domains.PayoutService {
	return &payoutService{store: store, r4Repo: r4Repo, logger: logger}
}

// SendVuelto pays once per payout id. The order is the point:
//  1. reserve the id (unique) BEFORE calling R4 — a duplicate never reaches R4,
//     it gets the stored outcome back;
//  2. call R4 once, no retry;
//  3. store what happened. If that write fails the row stays "pending", which a
//     replay reports as "unknown" — the safe direction.
//
// A returned error means nothing was sent.
func (s *payoutService) SendVuelto(ctx context.Context, req models.VueltoPayoutRequest) (models.VueltoPayoutResponse, error) {
	existing, created, err := s.store.Reserve(ctx, &dbModels.R4AppaPayout{
		PayoutID: req.PayoutID,
		Status:   dbModels.PayoutPending,
		Bank:     req.Bank,
		Phone:    req.Phone,
		DNI:      req.DNI,
		Amount:   req.Amount,
		Concept:  req.Concept,
	})
	if errors.Is(err, domains.ErrPayoutConflictUnreadable) {
		// An earlier request owns this id and may have reached R4. Whatever this
		// answer is, it must not be "nothing was sent".
		s.logger.Error("payout id reserved but unreadable", zap.String("payoutId", req.PayoutID), zap.Error(err))
		return models.VueltoPayoutResponse{
			Outcome:  string(r4bank.VueltoUnknown),
			Detail:   "este pago ya estaba registrado y no se pudo leer su resultado",
			Replayed: true,
		}, nil
	}
	if err != nil {
		return models.VueltoPayoutResponse{}, err
	}

	if !created {
		if !samePayout(existing, req) {
			s.logger.Error("payout id reused with different details", zap.String("payoutId", req.PayoutID))
			return models.VueltoPayoutResponse{}, domains.ErrPayoutMismatch
		}
		return replay(existing), nil
	}

	// The caller hanging up must not abort a Vuelto in flight, nor lose its
	// result: from here on the request's cancellation is ignored.
	detached := context.WithoutCancel(ctx)

	result := s.r4Repo.SendVuelto(detached, r4bank.ChangePaidRequest{
		Bank:    req.Bank,
		Amount:  req.Amount,
		Phone:   req.Phone,
		DNI:     req.DNI,
		Concept: req.Concept,
	})

	// Bounded: a stalled database must not hold the caller past its own timeout
	// with a result we already have in hand (and have logged, in SendVuelto).
	finishCtx, cancel := context.WithTimeout(detached, finishTimeout)
	defer cancel()
	if err := s.store.Finish(finishCtx, req.PayoutID, string(result.Outcome), result.Reference, result.Detail); err != nil {
		s.logger.Error("could not store payout outcome; row stays pending",
			zap.String("payoutId", req.PayoutID),
			zap.String("outcome", string(result.Outcome)),
			zap.String("reference", result.Reference),
			zap.Error(err),
		)
	}

	return models.VueltoPayoutResponse{
		Outcome:   string(result.Outcome),
		Reference: result.Reference,
		Detail:    result.Detail,
	}, nil
}

const finishTimeout = 10 * time.Second

func samePayout(row *dbModels.R4AppaPayout, req models.VueltoPayoutRequest) bool {
	return row.Bank == req.Bank && row.Phone == req.Phone && row.DNI == req.DNI &&
		math.Abs(row.Amount-req.Amount) < 0.005
}

// replay answers a repeated payout id from the record. R4 is not called.
func replay(row *dbModels.R4AppaPayout) models.VueltoPayoutResponse {
	resp := models.VueltoPayoutResponse{Replayed: true, Reference: row.Reference, Detail: row.Detail}
	switch row.Status {
	case dbModels.PayoutConfirmed:
		resp.Outcome = string(r4bank.VueltoConfirmed)
	case dbModels.PayoutRejected:
		resp.Outcome = string(r4bank.VueltoRejected)
	default:
		// "unknown", or "pending": an earlier attempt is in flight or died
		// mid-call. Either way the money may have moved.
		resp.Outcome = string(r4bank.VueltoUnknown)
		if row.Status == dbModels.PayoutPending {
			resp.Detail = "un intento anterior no dejó resultado"
		}
	}
	return resp
}

type payoutStore struct {
	db *gorm.DB
}

func NewPayoutStore(db *gorm.DB) domains.PayoutStore {
	return &payoutStore{db: db}
}

// Reserve relies on the UNIQUE index on payout_id (see pkg/db/schema.sql).
// Without it ON CONFLICT errors and nothing is paid — it fails closed.
func (s *payoutStore) Reserve(ctx context.Context, payout *dbModels.R4AppaPayout) (*dbModels.R4AppaPayout, bool, error) {
	res := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "payout_id"}}, DoNothing: true}).
		Create(payout)
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 1 {
		return payout, true, nil
	}

	var existing dbModels.R4AppaPayout
	if err := s.db.WithContext(ctx).Where("payout_id = ?", payout.PayoutID).First(&existing).Error; err != nil {
		return nil, false, fmt.Errorf("%w: %v", domains.ErrPayoutConflictUnreadable, err)
	}
	return &existing, false, nil
}

func (s *payoutStore) Finish(ctx context.Context, payoutID, status, reference, detail string) error {
	res := s.db.WithContext(ctx).Model(&dbModels.R4AppaPayout{}).
		Where("payout_id = ? AND status = ?", payoutID, dbModels.PayoutPending).
		Updates(map[string]any{"status": status, "reference": reference, "detail": detail})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("payout %s was no longer pending; outcome %q not stored", payoutID, status)
	}
	return nil
}
