package r4bank

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"appa_payments/internal/domains"

	"go.uber.org/zap"
)

func testLogger() *zap.Logger { return zap.NewNop() }

// A refusal reaches the caller as a response carrying the bank's code, not as
// an error: the code is what the checkout turns into a reason for the buyer.
func TestValidateImmediateDebit_RefusalCarriesCodeAndOperationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"saldo insuficiente","code":"AM04","operation_id":"op-77"}`))
	}))
	defer server.Close()

	repo := NewR4Repository(testLogger(), server.URL, "token", "secret")
	resp, err := repo.ValidateImmediateDebit(context.Background(), ValidateOTPRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != domains.R4CodeInsufficientFunds {
		t.Fatalf("code = %q, want %q", resp.Code, domains.R4CodeInsufficientFunds)
	}
	if resp.ID != "op-77" {
		t.Fatalf("operation id = %q, want %q", resp.ID, "op-77")
	}
	if resp.Status {
		t.Fatal("status = true, want false for a refusal")
	}
}

func TestDirectDebitAccount_RefusalCarriesCodeAndOperationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"no posee afiliación","code":"MD09","operation_id":"op-9"}`))
	}))
	defer server.Close()

	repo := NewR4Repository(testLogger(), server.URL, "token", "secret")
	resp, err := repo.DirectDebitAccount(context.Background(), DirectDebitAccountRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != domains.R4CodeAffiliationNotAcepted {
		t.Fatalf("code = %q, want %q", resp.Code, domains.R4CodeAffiliationNotAcepted)
	}
	if resp.ID != "op-9" {
		t.Fatalf("operation id = %q, want %q", resp.ID, "op-9")
	}
	if resp.Success {
		t.Fatal("success = true, want false for a refusal")
	}
}

// A 500 leaves no code behind, and inventing one would tell the buyer their
// charge failed when it may still be in flight.
func TestValidateImmediateDebit_InternalErrorStaysAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	repo := NewR4Repository(testLogger(), server.URL, "token", "secret")
	resp, err := repo.ValidateImmediateDebit(context.Background(), ValidateOTPRequest{})
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
	if resp != nil {
		t.Fatalf("expected no response, got %+v", resp)
	}
}

func TestDirectDebitAccount_InternalErrorStaysAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	repo := NewR4Repository(testLogger(), server.URL, "token", "secret")
	resp, err := repo.DirectDebitAccount(context.Background(), DirectDebitAccountRequest{})
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
	if resp != nil {
		t.Fatalf("expected no response, got %+v", resp)
	}
}
