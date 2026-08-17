package bcv

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

// GetBCV fetches the exchange rate from the BCV Website
func (c *client) Get(ctx context.Context) (float64, error) {

	if c.BCVTasa != nil && helpers.SameDay(c.BCVTasa.Date, time.Now()) {
		return c.BCVTasa.Rate, nil
	}

	var rate float64
	tasa, err := c.R4Repository.GetBCVTasaUSD(ctx)
	if err != nil {
		c.logger.Error("failed to get BCV tasa from R4Bank repository: " + err.Error())
		rate, err = c.fetchRate()
		if err != nil {
			c.logger.Error(err.Error())
			return 0.0, err
		}
	} else {
		rate = tasa.Rate
	}

	// If BCVTasa is nil or the date is not today, fetch a new rate

	c.BCVTasa = &Rate{
		Date: time.Now(),
		Rate: rate,
	}

	if rate == 0 {
		return 0, fmt.Errorf("no exchange rate found on BCV page")
	}
	return rate, nil
}

func (c *client) fetchRate() (float64, error) {
	url := "https://www.bcv.org.ve/"

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			// Use Cloudflare DNS server 1.1.1.1
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver:  resolver,
	}

	httpTransport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Create custom HTTP client that skips TLS verification
	client := &http.Client{
		Transport: httpTransport,
	}
	res, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to fetch BCV: %s", res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return 0, err
	}

	var rate float64
	doc.Find(".recuadrotsmc strong").Each(func(i int, s *goquery.Selection) {
		textValue := strings.TrimSpace(s.Text())
		textValue = strings.ReplaceAll(textValue, ",", ".")
		value, err := strconv.ParseFloat(textValue, 64)
		if err != nil {
			return
		}

		rate = value
	})
	return rate, nil
}
