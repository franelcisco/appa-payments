package handlers

import (
	"errors"
	"math"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
)

type PayoutHandler struct {
	service  domains.PayoutService
	verifier domains.PayoutVerifier
	logger   *zap.Logger
}

func NewPayoutHandler(service domains.PayoutService, verifier domains.PayoutVerifier, logger *zap.Logger) *PayoutHandler {
	return &PayoutHandler{service: service, verifier: verifier, logger: logger}
}

// The signer is trusted for WHO gets paid, not for hygiene. These shapes keep a
// malformed instruction away from R4 and keep ':' out of the signed fields.
var (
	payoutIDPattern = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	bankPattern     = regexp.MustCompile(`^\d{4}$`)
	phonePattern    = regexp.MustCompile(`^04\d{9}$`)
	dniPattern      = regexp.MustCompile(`^[VEJPG]\d{5,10}$`)
)

const maxConceptLen = 60

func validPayoutRequest(req models.VueltoPayoutRequest) bool {
	cents := req.Amount * 100
	return payoutIDPattern.MatchString(req.PayoutID) &&
		bankPattern.MatchString(req.Bank) &&
		phonePattern.MatchString(req.Phone) &&
		dniPattern.MatchString(req.DNI) &&
		len(req.Concept) <= maxConceptLen &&
		req.Amount >= 0.01 && !math.IsInf(req.Amount, 0) &&
		// whole céntimos only: what is signed (%.2f) is then exactly what is stored and sent
		math.Abs(cents-math.Round(cents)) < 1e-6
}

// notSent is every answer where R4 was never called. `sent: false` is a promise
// the caller relies on to mark its own attempt as safely failed.
func notSent(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "sent": false})
}

// HandleVuelto sends one Vuelto. 200 carries an outcome (confirmed | rejected |
// unknown); any other status means nothing was sent.
func (h *PayoutHandler) HandleVuelto(c *gin.Context) {
	var sig models.PayoutSignature
	if err := c.ShouldBindHeader(&sig); err != nil {
		notSent(c, http.StatusUnauthorized, "payout_unsigned")
		return
	}
	var req models.VueltoPayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPayoutRequest(req) {
		notSent(c, http.StatusBadRequest, "bad_request")
		return
	}

	if err := h.verifier.Verify(sig, req, time.Now()); err != nil {
		h.logger.Error("payout signature refused", zap.String("payoutId", req.PayoutID), zap.Error(err))
		switch {
		case errors.Is(err, domains.ErrPayoutNoSecret):
			notSent(c, http.StatusServiceUnavailable, "payout_misconfigured")
		case errors.Is(err, domains.ErrPayoutExpired):
			notSent(c, http.StatusUnauthorized, "payout_expired")
		case errors.Is(err, domains.ErrPayoutUnsigned):
			notSent(c, http.StatusUnauthorized, "payout_unsigned")
		default:
			notSent(c, http.StatusUnauthorized, "payout_invalid")
		}
		return
	}

	resp, err := h.service.SendVuelto(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, domains.ErrPayoutMismatch) {
			// No `sent: false` here. THIS request sent nothing, but the id belongs
			// to an earlier one that may have; the caller must not read a promise
			// about that payout into this answer.
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "payout_id_mismatch"})
			return
		}
		h.logger.Error("payout ledger unavailable", zap.String("payoutId", req.PayoutID), zap.Error(err))
		notSent(c, http.StatusServiceUnavailable, "payout_ledger_unavailable")
		return
	}

	c.JSON(http.StatusOK, resp)
}
