package bcv

import (
	"context"
	"time"

	"go.uber.org/zap"

	helpers "appa_payments/pkg"
	"appa_payments/pkg/r4bank"
)

type Client interface {
	Get(ctx context.Context) (float64, error)
}

type client struct {
	R4Repository r4bank.R4Repository
	BCVTasa      *Rate
	loc          *time.Location
	logger       *zap.Logger
}

func NewClient(R4Repository r4bank.R4Repository, loc *time.Location, logger *zap.Logger) Client {
	return &client{R4Repository: R4Repository, loc: loc, logger: logger}
}

// GetBCVTasa gets the BCV Tasa
func (c *client) Get(ctx context.Context) (float64, error) {
	if c.BCVTasa != nil && helpers.SameDay(c.BCVTasa.Date, time.Now()) {
		return c.BCVTasa.Rate, nil
	}

	// If BCVTasa is nil or the date is not today, fetch a new rate
	tasa, err := c.R4Repository.GetBCVTasaUSD(ctx)
	if err != nil {
		c.logger.Error(err.Error())
		return 0.0, err
	}

	c.BCVTasa = &Rate{
		Date: time.Now(),
		Rate: tasa.Rate,
	}

	return c.BCVTasa.Rate, nil
}
