package r4bank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"appa_payments/internal/domains"
	helpers "appa_payments/pkg"

	"go.uber.org/zap"
)

const defaultRequestTimeout = 25 * time.Second

// Above the r4-service's 3m30s operation deadline so its concrete R4 error
// arrives instead of a generic client timeout; below Cloud Run's 300s inbound
// timeout so we answer our caller before the platform cuts the connection.
// No inbound request may chain two calls that use it.
const pollingRequestTimeout = 4 * time.Minute

var pollingEndpoints = map[string]bool{
	r4ValidateImmediateEndpoint:  true,
	r4DirectDebitAccountEndpoint: true,
}

func requestTimeout(endpoint string) time.Duration {
	if pollingEndpoints[endpoint] {
		return pollingRequestTimeout
	}
	return defaultRequestTimeout
}

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
		client:  &http.Client{},
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

	ctx, cancel := context.WithTimeout(ctx, requestTimeout(endpoint))
	defer cancel()

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

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Error(err.Error(), zap.Any("payload", payload))
		return nil, fmt.Errorf("error en request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body domains.R4errorResponse
		_ = json.Unmarshal(data, &body)

		if endpoint == r4ValidateImmediateEndpoint {
			return nil, &domains.R4APIError{Code: body.Code, OperationID: body.OperationID, Detail: string(data)}
		}
		r.logger.Error("R4 API error: ", zap.String("body", string(data)), zap.Any("payload", payload))
		return nil, &domains.R4APIError{
			Code:        body.Code,
			OperationID: body.OperationID,
			Detail:      fmt.Sprintf("R4 API error: %s", string(data)),
		}
	}

	return data, nil
}
