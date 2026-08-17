package r4bank

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type R4Repository interface {
	GetBCVTasaUSD(ctx context.Context) (*BCVTasaUSDResponse, error)
	GenerateOTP(ctx context.Context, req OTPRequest) error
	ValidateImmediateDebit(ctx context.Context, req ValidateOTPRequest) (*ValidateDebitInmediateResponse, error)
	ChangePaid(ctx context.Context, req ChangePaidRequest) error
	GetOperationByID(ctx context.Context, operationID string) (*GetOperationResponse, error)
	DirectDebitAccount(ctx context.Context, req DirectDebitAccountRequest) (*DirectDebitAccountResponse, error)
}

type R4repository struct {
	r4EntryPoint string
	r4Client     *RestClient
	logger       *zap.Logger
}

const (
	// R4EntryPoint endpoints for R4Bank API
	r4bcvTasaEndpoint            = "r4/appa/bcv-tasa"
	r4GenerateOTPEndpoint        = "r4/appa/generate-otp"
	r4ValidateImmediateEndpoint  = "r4/appa/validate-immediate-debit"
	r4ChangePaidEndpoint         = "r4/appa/change-paid"
	r4GetOperationEndpoint       = "r4/appa/get-operation"
	r4DirectDebitAccountEndpoint = "r4/appa/direct-debit-account"
)

func NewR4Repository(logger *zap.Logger, r4EntryPoint, token, secret string) R4Repository {
	return &R4repository{
		r4EntryPoint: r4EntryPoint,
		r4Client:     NewClient(r4EntryPoint, token, secret, logger),
		logger:       logger,
	}
}

// GetBCVTasaUSD retrieves the BCV exchange rate for USD
func (r *R4repository) GetBCVTasaUSD(ctx context.Context) (*BCVTasaUSDResponse, error) {
	resp, err := r.r4Client.Do(ctx, nil, r4bcvTasaEndpoint, http.MethodGet)
	if err != nil {
		return nil, fmt.Errorf("error en request: %w", err)
	}

	var r4Resp BCVTasaUSDResponse
	if err := json.Unmarshal(resp, &r4Resp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &r4Resp, nil
}

// GenerateOTP generates a one-time password for direct debit transactions
func (r *R4repository) GenerateOTP(ctx context.Context, req OTPRequest) error {
	_, err := r.r4Client.Do(ctx, req, r4GenerateOTPEndpoint, http.MethodPost)
	if err != nil {
		r.logger.Error(err.Error(), zap.Any("request", req))
		return fmt.Errorf("error en request: %w", err)
	}

	return nil
}

// ValidateImmediateDebit validates an immediate debit transaction
func (r *R4repository) ValidateImmediateDebit(ctx context.Context, req ValidateOTPRequest) (*ValidateDebitInmediateResponse, error) {
	resp, err := r.r4Client.Do(ctx, req, r4ValidateImmediateEndpoint, http.MethodPost)
	if err != nil {
		return nil, fmt.Errorf("error en request: %w", err)
	}

	var r4Resp ValidateDebitInmediateResponse
	if err := json.Unmarshal(resp, &r4Resp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &r4Resp, nil
}

// ChangePaid processes a change paid transaction
func (r *R4repository) ChangePaid(ctx context.Context, req ChangePaidRequest) error {
	_, err := r.r4Client.Do(ctx, req, r4ChangePaidEndpoint, http.MethodPost)
	if err != nil {
		return fmt.Errorf("error en request: %w", err)
	}

	return nil
}

// GetOperationByID retrieves an operation by its ID
func (r *R4repository) GetOperationByID(ctx context.Context, operationID string) (*GetOperationResponse, error) {
	resp, err := r.r4Client.Do(ctx, nil, fmt.Sprintf("%s/%s", r4GetOperationEndpoint, operationID), http.MethodGet)
	if err != nil {
		return nil, fmt.Errorf("error en request: %w", err)
	}

	var operationResp GetOperationResponse
	if err := json.Unmarshal(resp, &operationResp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &operationResp, nil
}

// DirectDebitAccount processes a direct debit account charge
func (r *R4repository) DirectDebitAccount(ctx context.Context, req DirectDebitAccountRequest) (*DirectDebitAccountResponse, error) {
	resp, err := r.r4Client.Do(ctx, req, r4DirectDebitAccountEndpoint, http.MethodPost)
	if err != nil {
		return nil, fmt.Errorf("error en request: %w", err)
	}

	var accountResp DirectDebitAccountResponse
	if err := json.Unmarshal(resp, &accountResp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &accountResp, nil
}
