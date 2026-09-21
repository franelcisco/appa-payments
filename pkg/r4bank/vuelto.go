package r4bank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	helpers "appa_payments/pkg"

	"go.uber.org/zap"
)

// VueltoOutcome is what a payout over MBvuelto came to. ChangePaid (above, used
// for refunds) only says error / no error; a payout needs the R4 reference to
// reconcile against the bank statement, and needs "R4 said no" kept apart from
// "we never heard back" — the second one may have moved money.
type VueltoOutcome string

const (
	// VueltoConfirmed: R4 accepted it and gave a reference.
	VueltoConfirmed VueltoOutcome = "confirmed"
	// VueltoRejected: a definite no. No money moved; safe to try again.
	VueltoRejected VueltoOutcome = "rejected"
	// VueltoUnknown: no usable answer. The money MAY have moved. Never retried.
	VueltoUnknown VueltoOutcome = "unknown"
)

type VueltoResult struct {
	Outcome   VueltoOutcome
	Reference string
	Detail    string
}

// vueltoRejectCodes are the R4 answer codes that mean a DEFINITE no: nothing was
// debited and it is safe to try again. Empty by default, on purpose.
//
// The R4 service reports every R4 code other than "00" with one generic error
// and does not pass the code on. "Not 00" is not "no": R4's own table has codes
// like AC00 ("en espera de respuesta del banco") and network time-outs, where
// the transfer may still go through. Calling those "rejected" would invite a
// second payment. So until the R4 service returns the code AND the codes below
// are confirmed against R4's table, a non-00 answer is "unknown" and a person
// reconciles it. Set VUELTO_REJECT_CODES (comma separated) to turn this on.
var vueltoRejectCodes = map[string]struct{}{}

// ConfigureVueltoRejectCodes is called once from main, before serving.
func ConfigureVueltoRejectCodes(csv string) {
	codes := map[string]struct{}{}
	for _, c := range strings.Split(csv, ",") {
		if c = strings.ToUpper(strings.TrimSpace(c)); c != "" && c != "00" {
			codes[c] = struct{}{}
		}
	}
	vueltoRejectCodes = codes
}

// The R4 service waits 20s on R4. Waiting the same 20s here would turn a slow
// but successful Vuelto into an "unknown", so this call gets its own, longer
// client instead of the shared one (whose Timeout Do mutates per endpoint).
var vueltoTimeout = 30 * time.Second

type vueltoServiceResponse struct {
	Reference string `json:"reference"`
	Error     string `json:"error"`
	// Code is R4's own answer code. The R4 service does not send it today.
	Code string `json:"code"`
}

// ClassifyVuelto turns the R4 service's answer into an outcome. Anything that is
// not clearly a yes or clearly a no is Unknown — that is the safe direction.
func ClassifyVuelto(status int, body []byte, callErr error) VueltoResult {
	if callErr != nil {
		return VueltoResult{Outcome: VueltoUnknown, Detail: "sin respuesta del servicio R4: " + callErr.Error()}
	}

	var resp vueltoServiceResponse
	_ = json.Unmarshal(body, &resp)

	if status >= 200 && status < 300 {
		ref := strings.TrimSpace(resp.Reference)
		if ref == "" || ref == "0" {
			// A 2xx we can't reconcile against the statement isn't a confirmation.
			return VueltoResult{Outcome: VueltoUnknown, Detail: "respuesta 2xx sin referencia"}
		}
		return VueltoResult{Outcome: VueltoConfirmed, Reference: ref}
	}

	if code := strings.ToUpper(strings.TrimSpace(resp.Code)); code != "" {
		if _, definite := vueltoRejectCodes[code]; definite {
			return VueltoResult{Outcome: VueltoRejected, Detail: "R4 " + code + " " + resp.Error}
		}
	}
	// The R4 service refused the request itself (bad payload, bad auth) before
	// it ever called R4.
	if status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden {
		return VueltoResult{Outcome: VueltoRejected, Detail: fmt.Sprintf("HTTP %d %s", status, resp.Error)}
	}

	detail := resp.Error
	if detail == "" {
		detail = fmt.Sprintf("HTTP %d", status)
	}
	return VueltoResult{Outcome: VueltoUnknown, Detail: detail}
}

// SendVuelto pushes a pago móvil to the payee. One call, no retry: the caller
// owns the idempotency record and must have written it before calling this.
func (r *R4repository) SendVuelto(ctx context.Context, req ChangePaidRequest) VueltoResult {
	status, body, err := r.r4Client.doRaw(ctx, req, r4ChangePaidEndpoint, vueltoTimeout)
	result := ClassifyVuelto(status, body, err)
	// Logged in every case, confirmed included: if the ledger write that follows
	// is lost, this line is the only place the R4 reference survives.
	fields := []zap.Field{
		zap.String("outcome", string(result.Outcome)),
		zap.String("reference", result.Reference),
		zap.String("detail", result.Detail),
		zap.Int("status", status),
		zap.String("phone", req.Phone),
		zap.Float64("amount", req.Amount),
	}
	if result.Outcome == VueltoConfirmed {
		r.logger.Info("vuelto payout sent", fields...)
	} else {
		r.logger.Error("vuelto payout not confirmed", fields...)
	}
	return result
}

// doRaw is Do without the flattening: it hands back the status and body so the
// caller can tell a rejection from a lost answer.
func (r *RestClient) doRaw(ctx context.Context, payload any, endpoint string, timeout time.Duration) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("error marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s", r.baseURL, endpoint), bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", helpers.GenerateAuthToken(r.token, r.secret))

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		// Headers arrived but the body didn't: R4 may well have acted.
		return 0, nil, fmt.Errorf("reading response: %w", err)
	}
	return resp.StatusCode, data, nil
}
