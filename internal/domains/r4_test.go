package domains

import "testing"

// Pinned because these strings are not ours to choose — they are the bank's.
func TestR4CodeValues(t *testing.T) {
	cases := map[R4Code]string{
		R4CodeApproved:              "ACCP",
		R4CodeInProgress:            "AC00",
		R4CodeInPending:             "11",
		R4CodeInsufficientFunds:     "AM04",
		R4CodeAffiliationRequested:  "MD01",
		R4CodeAffiliationNotAcepted: "MD09",
		R4CodeInvalidAccountNumber:  "AC01",
		R4CodeUpstreamRejected:      "FAI01",
		R4InvalidAmount:             "MD15",
		R4InvalidOTP:                "TKCM",
		R4InvalidTime:               "VE01",
		R4InvalidClientData:         "BE01",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Fatalf("code %q, want %q", got, want)
		}
	}
}

// Every code we translate needs its own text: a description that falls back to
// "Desconocido" is what the buyer reads when their charge fails, so a missing
// entry is a silent regression, not a cosmetic one.
func TestGetR4CodeDescription(t *testing.T) {
	described := []R4Code{
		R4CodeApproved, R4CodeInProgress, R4CodeInPending, R4CodeInsufficientFunds,
		R4CodeAffiliationRequested, R4CodeAffiliationNotAcepted, R4CodeInvalidAccountNumber,
		R4CodeUpstreamRejected, R4InvalidAmount, R4InvalidOTP, R4InvalidTime, R4InvalidClientData,
	}
	for _, code := range described {
		if got := code.GetR4CodeDescription(); got == R4CodeUnknownDescription {
			t.Fatalf("GetR4CodeDescription(%q) = %q, want a description", code, got)
		}
	}

	// An unmapped code still has to answer something, because the caller now
	// passes it through to the checkout instead of failing the request.
	for _, code := range []R4Code{"ZZ99", ""} {
		if got := code.GetR4CodeDescription(); got != R4CodeUnknownDescription {
			t.Fatalf("GetR4CodeDescription(%q) = %q, want %q", code, got, R4CodeUnknownDescription)
		}
	}
}

func TestIsR4BreakCode(t *testing.T) {
	cases := map[R4Code]bool{
		R4CodeInProgress:           true,
		R4CodeInPending:            true,
		R4CodeApproved:             false,
		R4CodeInsufficientFunds:    false,
		R4CodeInvalidAccountNumber: false,
		"ZZ99":                     false,
		"":                         false,
	}
	for code, want := range cases {
		if got := IsR4BreakCode(code); got != want {
			t.Fatalf("IsR4BreakCode(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestDirectDebitAccountResponseCode(t *testing.T) {
	cases := []struct {
		r4Code R4Code
		want   string
		wantOK bool
	}{
		{"AM04", "ERR01", true},
		{"MD01", "ERR02", true},
		{"MD09", "ERR03", true},
		{"AC01", "ERR04", true},
		// An unmapped code must not resolve to something plausible: the caller
		// passes the raw R4 code through rather than tell the buyer a reason we
		// invented.
		{"ACCP", "", false},
		{"ZZ99", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := DirectDebitAccountResponseCode(tc.r4Code)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("DirectDebitAccountResponseCode(%q) = (%q, %v), want (%q, %v)",
				tc.r4Code, got, ok, tc.want, tc.wantOK)
		}
	}
}

// Pinned because these strings are the r4-service's, not ours: they arrive in
// its error body and get stored verbatim.
func TestInternalCodeValues(t *testing.T) {
	cases := map[R4Code]string{
		R4CodeTimeout:                 "INT01",
		R4CodeUpstreamError:           "INT02",
		R4CodeInvalidUpstreamResponse: "INT03",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Fatalf("code %q, want %q", got, want)
		}
	}

	for _, code := range []R4Code{R4CodeTimeout, R4CodeUpstreamError, R4CodeInvalidUpstreamResponse} {
		if got := code.GetR4CodeDescription(); got == R4CodeUnknownDescription {
			t.Fatalf("GetR4CodeDescription(%q) = %q, want a description", code, got)
		}
	}
}

// A code that is reconcilable is one whose row may still be updated. Getting
// this wrong either re-queries an operation the bank already settled or leaves
// an undetermined charge stuck forever.
func TestIsReconcilableCode(t *testing.T) {
	cases := map[R4Code]bool{
		R4CodeInProgress:              true,
		R4CodeInPending:               true,
		R4CodeTimeout:                 true,
		R4CodeUpstreamError:           true,
		R4CodeInvalidUpstreamResponse: true,
		R4CodeApproved:                false,
		R4CodeInsufficientFunds:       false,
		R4CodeInvalidAccountNumber:    false,
		R4CodeUpstreamRejected:        false,
		"ZZ99":                        false,
		"":                            false,
	}
	for code, want := range cases {
		if got := IsReconcilableCode(code); got != want {
			t.Fatalf("IsReconcilableCode(%q) = %v, want %v", code, got, want)
		}
	}
}
