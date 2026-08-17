package domains

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"appa_payments/internal/models"
)

// CartQuoteHeaders are the headers Handler reads. CORS has to allow them by
// name: they are custom, so a browser drops the request at preflight and the
// middleware never runs.
var CartQuoteHeaders = []string{
	"X-Cart-Id",
	"X-Cart-Amount",
	"X-Cart-Exp",
	"X-Cart-Signature",
}

// CartQuoteRepository verifies the amounts create-order-from-cart signs, so the
// browser can carry one without being able to alter it.
type CartQuoteRepository interface {
	Verify(quote models.SignedCartQuote, now time.Time) (models.CartQuote, error)
	Handler() gin.HandlerFunc
}

var (
	ErrQuoteNoSecret = errors.New("cart quote secret not configured")
	ErrQuoteUnsigned = errors.New("cart quote signature missing")
	ErrQuoteInvalid  = errors.New("cart quote signature does not verify")
	ErrQuoteExpired  = errors.New("cart quote expired")
)
