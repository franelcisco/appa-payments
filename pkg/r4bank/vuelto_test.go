package r4bank

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	helpers "appa_payments/pkg"

	"go.uber.org/zap"
)

func TestClassifyVuelto(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		callErr error
		want    VueltoOutcome
		wantRef string
	}{
		{"200 with reference", 200, `{"reference":"81234567"}`, nil, VueltoConfirmed, "81234567"},
		{"R4 said a configured definite-no code", 500, `{"error":"R4 Change Paid API returned an error","code":"51"}`, nil, VueltoRejected, ""},
		// "Not 00" is not "no": without a code we can vouch for, a person reconciles.
		{"R4 non-00 with no code passed on", 500, `{"error":"R4 Change Paid API returned an error"}`, nil, VueltoUnknown, ""},
		{"R4 code that is not a definite no (AC00 = waiting on the bank)", 500, `{"error":"R4 Change Paid API returned an error","code":"AC00"}`, nil, VueltoUnknown, ""},
		{"service refused payload", 400, `{"error":"Invalid request payload"}`, nil, VueltoRejected, ""},
		{"service refused auth", 401, `{"abono":false}`, nil, VueltoRejected, ""},
		// Every case below may have moved money.
		{"transport error", 0, ``, errors.New("context deadline exceeded"), VueltoUnknown, ""},
		{"service could not reach R4", 500, `{"error":"error en request: timeout"}`, nil, VueltoUnknown, ""},
		{"undecodable R4 answer", 500, `{"error":"error decodificando respuesta: EOF"}`, nil, VueltoUnknown, ""},
		{"gateway error, no body", 502, ``, nil, VueltoUnknown, ""},
		{"2xx without reference", 200, `{}`, nil, VueltoUnknown, ""},
		{"2xx with reference 0", 200, `{"reference":"0"}`, nil, VueltoUnknown, ""},
		{"2xx with garbage", 200, `<html>`, nil, VueltoUnknown, ""},
	}
	ConfigureVueltoRejectCodes(" 51, 56 ,00")
	defer ConfigureVueltoRejectCodes("")
	if _, ok := vueltoRejectCodes["00"]; ok {
		t.Fatal(`"00" is success and must never be configurable as a rejection`)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyVuelto(tc.status, []byte(tc.body), tc.callErr)
			if got.Outcome != tc.want || got.Reference != tc.wantRef {
				t.Fatalf("got %+v, want outcome %q ref %q", got, tc.want, tc.wantRef)
			}
		})
	}
}

// SendVuelto against a stand-in for the R4 service: checks the wire format the
// real service expects, and that a hang becomes "unknown" rather than an error
// someone might retry.
func TestSendVuelto(t *testing.T) {
	var gotAuth, gotPath string
	mode := "ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		switch mode {
		case "ok":
			w.Write([]byte(`{"reference":"555"}`))
		case "rejected":
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"Invalid request payload"}`))
		case "hang":
			time.Sleep(300 * time.Millisecond)
		}
	}))
	defer srv.Close()

	old := vueltoTimeout
	vueltoTimeout = 100 * time.Millisecond
	defer func() { vueltoTimeout = old }()

	repo := NewR4Repository(zap.NewNop(), srv.URL, "commerce-token", "shared-secret")
	req := ChangePaidRequest{Bank: "0102", Amount: 4520.5, Phone: "04141234567", DNI: "V12345678", Concept: "Reembolso APPA"}

	if got := repo.SendVuelto(context.Background(), req); got.Outcome != VueltoConfirmed || got.Reference != "555" {
		t.Fatalf("ok: got %+v", got)
	}
	if gotPath != "/r4/appa/change-paid" {
		t.Fatalf("path = %q", gotPath)
	}
	// hex HMAC-SHA256(key=commerce token, msg=secret) — what the R4 service validates.
	if want := helpers.GenerateAuthToken("commerce-token", "shared-secret"); gotAuth != want {
		t.Fatalf("authorization = %q, want %q", gotAuth, want)
	}

	mode = "rejected"
	if got := repo.SendVuelto(context.Background(), req); got.Outcome != VueltoRejected {
		t.Fatalf("rejected: got %+v", got)
	}

	mode = "hang"
	if got := repo.SendVuelto(context.Background(), req); got.Outcome != VueltoUnknown {
		t.Fatalf("hang: got %+v", got)
	}
}
