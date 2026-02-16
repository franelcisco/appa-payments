package r4bank

import (
	helpers "appa_payments/pkg"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type RestClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
	token   string
	secret  string
}

// NewClient creates a new instance of RestClient
func NewClient(
	endpoint string,
	token string,
	secret string,
	logger *zap.Logger,
) *RestClient {
	return &RestClient{
		baseURL: endpoint,
		client:  &http.Client{Timeout: 20 * time.Second},
		token:   token,
		secret:  secret,
		logger:  logger,
	}
}

// Do executes an HTTP request
func (r *RestClient) Do(
	ctx context.Context,
	payload any,
	endpoint string,
	method string,
) ([]byte, error) {
	var (
		body []byte
		err  error
	)

	if payload == nil {
		payload = map[string]string{}
	}

	body, err = json.Marshal(payload)
	if err != nil {
		r.logger.Error(err.Error(), zap.Any("payload", payload))
		return nil, fmt.Errorf("error marshaling payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s", r.baseURL, endpoint)
	req, err := http.NewRequestWithContext(
		ctx, method, url, bytes.NewReader(body),
	)
	if err != nil {
		r.logger.Error(err.Error(), zap.Any("payload", payload))
		return nil, err
	}

	auth := helpers.GenerateAuthToken(r.token, r.secret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)

	if endpoint == "r4/appa/validate-immediate-debit" {
		r.client.Timeout = 35 * time.Second
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Error(err.Error(), zap.Any("payload", payload))
		return nil, fmt.Errorf("error en request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if endpoint == "r4/appa/validate-immediate-debit" {
			return nil, fmt.Errorf("%s", string(data))
		}
		r.logger.Error("R4 API error: ", zap.String("body", string(data)), zap.Any("payload", payload))
		return nil, fmt.Errorf("R4 API error: %s", string(data))
	}

	return data, nil
}
