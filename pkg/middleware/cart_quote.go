package middleware

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	helpers "appa_payments/pkg"
)

type cartQuoteRepository struct {
	secret string
	logger *zap.Logger
}

func NewCartQuoteRepository(secret string, logger *zap.Logger) domains.CartQuoteRepository {
	return &cartQuoteRepository{secret: secret, logger: logger}
}

const cartQuoteKey = "cartQuote"

func (r *cartQuoteRepository) Verify(
	quote models.SignedCartQuote,
	now time.Time,
) (models.CartQuote, error) {
	if r.secret == "" {
		return models.CartQuote{}, domains.ErrQuoteNoSecret
	}
	if quote.Signature == "" {
		return models.CartQuote{}, domains.ErrQuoteUnsigned
	}

	message := fmt.Sprintf("%s:%s:%d", quote.CartID, quote.Amount, quote.Exp)
	expected := helpers.GenerateAuthToken(r.secret, message)
	if !hmac.Equal([]byte(expected), []byte(quote.Signature)) {
		return models.CartQuote{}, domains.ErrQuoteInvalid
	}

	if !now.Before(time.Unix(quote.Exp, 0)) {
		return models.CartQuote{}, domains.ErrQuoteExpired
	}

	amount, err := strconv.ParseFloat(quote.Amount, 64)
	if err != nil || amount <= 0 {
		return models.CartQuote{}, domains.ErrQuoteInvalid
	}

	return models.CartQuote{CartID: quote.CartID, Amount: amount}, nil
}

func (r *cartQuoteRepository) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var signed models.SignedCartQuote
		if err := c.ShouldBindHeader(&signed); err != nil {
			r.logger.Error("failed to bind header", zap.Error(err))
			abort(c, http.StatusUnauthorized, "quote_unsigned")
			return
		}

		quote, err := r.Verify(signed, time.Now())
		if err != nil {
			r.logger.Error("failed to verify quote", zap.Error(err))
			status, code := http.StatusUnauthorized, "quote_invalid"
			switch {
			case errors.Is(err, domains.ErrQuoteNoSecret):
				status, code = http.StatusInternalServerError, "quote_misconfigured"
			case errors.Is(err, domains.ErrQuoteUnsigned):
				code = "quote_unsigned"
			case errors.Is(err, domains.ErrQuoteExpired):
				code = "quote_expired"
			}
			abort(c, status, code)
			return
		}

		c.Set(cartQuoteKey, quote)
		c.Next()
	}
}

func abort(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}

func CartQuoteFrom(c *gin.Context) models.CartQuote {
	return c.MustGet(cartQuoteKey).(models.CartQuote)
}
