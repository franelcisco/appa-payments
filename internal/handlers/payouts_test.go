package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"appa_payments/internal/domains"
	"appa_payments/internal/handlers"
	"appa_payments/internal/models"
	"appa_payments/internal/routes"
	"appa_payments/internal/services"
	helpers "appa_payments/pkg"
	dbModels "appa_payments/pkg/db/models"
	"appa_payments/pkg/middleware"
	"appa_payments/pkg/r4bank"
)

const testSecret = "payout-secret"

// fakeR4 counts Vueltos. Embedding the interface keeps it compiling as the
// repository grows; anything but SendVuelto would panic, which is what we want.
type fakeR4 struct {
	r4bank.R4Repository
	calls  atomic.Int32
	result r4bank.VueltoResult
	delay  time.Duration
}

func (f *fakeR4) SendVuelto(context.Context, r4bank.ChangePaidRequest) r4bank.VueltoResult {
	f.calls.Add(1)
	time.Sleep(f.delay)
	return f.result
}

// memStore mirrors the UNIQUE(payout_id) + ON CONFLICT DO NOTHING of the real one.
type memStore struct {
	mu      sync.Mutex
	rows    map[string]*dbModels.R4AppaPayout
	failAll bool
	// conflictUnreadable: the id is taken but reading its row fails.
	conflictUnreadable bool
}

func (m *memStore) Reserve(_ context.Context, p *dbModels.R4AppaPayout) (*dbModels.R4AppaPayout, bool, error) {
	if m.failAll {
		return nil, false, errors.New("relation r4_appa_payouts does not exist")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if row, ok := m.rows[p.PayoutID]; ok {
		if m.conflictUnreadable {
			return nil, false, fmt.Errorf("%w: connection reset", domains.ErrPayoutConflictUnreadable)
		}
		cp := *row
		return &cp, false, nil
	}
	cp := *p
	m.rows[p.PayoutID] = &cp
	return p, true, nil
}

func (m *memStore) Finish(_ context.Context, id, status, ref, detail string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.rows[id]
	row.Status, row.Reference, row.Detail = status, ref, detail
	return nil
}

func newRouter(r4 *fakeR4, store *memStore, secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := services.NewPayoutService(store, r4, zap.NewNop())
	routes.NewPayoutRoute(handlers.NewPayoutHandler(svc, middleware.NewPayoutVerifier(secret), zap.NewNop())).SetRouter(router)
	return router
}

func post(router *gin.Engine, req models.VueltoPayoutRequest, signWith string) (int, map[string]any) {
	body, _ := json.Marshal(req)
	exp := time.Now().Add(2 * time.Minute).Unix()
	httpReq := httptest.NewRequest(http.MethodPost, "/payouts/vuelto", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if signWith != "" {
		httpReq.Header.Set("X-Payout-Exp", strconv.FormatInt(exp, 10))
		httpReq.Header.Set("X-Payout-Signature", helpers.GenerateAuthToken(signWith, middleware.PayoutMessage(req, exp)))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httpReq)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

var payout = models.VueltoPayoutRequest{
	PayoutID: "11111111-1111-1111-1111-111111111111",
	Bank:     "0102", Phone: "04141234567", DNI: "V12345678", Amount: 4520.5, Concept: "Reembolso APPA",
}

func TestVueltoPaysOnceAndReplays(t *testing.T) {
	r4 := &fakeR4{result: r4bank.VueltoResult{Outcome: r4bank.VueltoConfirmed, Reference: "555"}}
	router := newRouter(r4, &memStore{rows: map[string]*dbModels.R4AppaPayout{}}, testSecret)

	code, out := post(router, payout, testSecret)
	if code != 200 || out["outcome"] != "confirmed" || out["reference"] != "555" || out["replayed"] != false {
		t.Fatalf("first call: %d %v", code, out)
	}

	code, out = post(router, payout, testSecret)
	if code != 200 || out["outcome"] != "confirmed" || out["reference"] != "555" || out["replayed"] != true {
		t.Fatalf("replay: %d %v", code, out)
	}
	if n := r4.calls.Load(); n != 1 {
		t.Fatalf("R4 was called %d times for one payout id, want 1", n)
	}
}

func TestVueltoConcurrentSameIDCallsR4Once(t *testing.T) {
	r4 := &fakeR4{result: r4bank.VueltoResult{Outcome: r4bank.VueltoConfirmed, Reference: "777"}, delay: 50 * time.Millisecond}
	router := newRouter(r4, &memStore{rows: map[string]*dbModels.R4AppaPayout{}}, testSecret)

	// The store here is a mutex-backed fake, so this proves the SERVICE never
	// calls R4 for a request that lost the reservation and never tells a loser
	// "nothing was sent". That Postgres makes exactly one request win is proven
	// in services/payouts_store_test.go.
	var wg sync.WaitGroup
	var winners, losers atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, out := post(router, payout, testSecret)
			switch {
			case code == 200 && out["outcome"] == "confirmed" && out["replayed"] == false:
				winners.Add(1)
			case code == 200 && out["replayed"] == true && (out["outcome"] == "unknown" || out["outcome"] == "confirmed"):
				losers.Add(1) // in flight → unknown; already finished → confirmed
			default:
				t.Errorf("unexpected answer %d %v", code, out)
			}
		}()
	}
	wg.Wait()
	if n := r4.calls.Load(); n != 1 {
		t.Fatalf("R4 was called %d times under concurrency, want 1", n)
	}
	if winners.Load() != 1 || losers.Load() != 7 {
		t.Fatalf("winners=%d losers=%d, want 1 and 7", winners.Load(), losers.Load())
	}
}

func TestVueltoUnknownIsNeverRetried(t *testing.T) {
	r4 := &fakeR4{result: r4bank.VueltoResult{Outcome: r4bank.VueltoUnknown, Detail: "sin respuesta"}}
	router := newRouter(r4, &memStore{rows: map[string]*dbModels.R4AppaPayout{}}, testSecret)

	for i := 0; i < 3; i++ {
		if code, out := post(router, payout, testSecret); code != 200 || out["outcome"] != "unknown" {
			t.Fatalf("call %d: %d %v", i, code, out)
		}
	}
	if n := r4.calls.Load(); n != 1 {
		t.Fatalf("an unknown payout was re-sent: R4 called %d times", n)
	}
}

func TestVueltoRefusalsNeverReachR4(t *testing.T) {
	bad := func(mut func(*models.VueltoPayoutRequest)) models.VueltoPayoutRequest {
		r := payout
		mut(&r)
		return r
	}

	cases := []struct {
		name     string
		secret   string // configured on the server
		signWith string
		store    *memStore
		req      models.VueltoPayoutRequest
		wantCode int
		wantBody string
	}{
		{"unsigned", testSecret, "", nil, payout, 401, "payout_unsigned"},
		{"signed with the wrong secret", testSecret, "guess", nil, payout, 401, "payout_invalid"},
		{"no secret configured: fails closed", "", testSecret, nil, payout, 503, "payout_misconfigured"},
		{"ledger table missing: fails closed", testSecret, testSecret, &memStore{failAll: true}, payout, 503, "payout_ledger_unavailable"},
		{"payout id is not a uuid", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.PayoutID = "p:1" }), 400, "bad_request"},
		{"phone is not a pago móvil number", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.Phone = "02121234567" }), 400, "bad_request"},
		{"document with a separator", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.DNI = "V-12345678" }), 400, "bad_request"},
		{"bank is a name", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.Bank = "Banesco" }), 400, "bad_request"},
		{"sub-céntimo amount", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.Amount = 0.004 }), 400, "bad_request"},
		{"amount with a third decimal", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.Amount = 100.125 }), 400, "bad_request"},
		{"negative amount", testSecret, testSecret, nil, bad(func(r *models.VueltoPayoutRequest) { r.Amount = -5 }), 400, "bad_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r4 := &fakeR4{result: r4bank.VueltoResult{Outcome: r4bank.VueltoConfirmed, Reference: "1"}}
			store := tc.store
			if store == nil {
				store = &memStore{rows: map[string]*dbModels.R4AppaPayout{}}
			}
			code, out := post(newRouter(r4, store, tc.secret), tc.req, tc.signWith)
			if code != tc.wantCode || out["code"] != tc.wantBody || out["sent"] != false {
				t.Fatalf("got %d %v", code, out)
			}
			if n := r4.calls.Load(); n != 0 {
				t.Fatalf("R4 was called %d times on a refused request", n)
			}
		})
	}
}

func TestVueltoStalePendingReplaysAsUnknown(t *testing.T) {
	r4 := &fakeR4{}
	store := &memStore{rows: map[string]*dbModels.R4AppaPayout{payout.PayoutID: {
		PayoutID: payout.PayoutID, Status: dbModels.PayoutPending, Bank: payout.Bank, Phone: payout.Phone, DNI: payout.DNI, Amount: payout.Amount,
	}}}
	code, out := post(newRouter(r4, store, testSecret), payout, testSecret)
	if code != 200 || out["outcome"] != "unknown" || out["replayed"] != true || r4.calls.Load() != 0 {
		t.Fatalf("got %d %v, R4 calls %d", code, out, r4.calls.Load())
	}
}

// An id that is already taken belongs to an earlier request that may have
// reached R4. Neither answer below may carry `sent: false`.
func TestVueltoTakenIDNeverPromisesNothingWasSent(t *testing.T) {
	taken := func() map[string]*dbModels.R4AppaPayout {
		return map[string]*dbModels.R4AppaPayout{payout.PayoutID: {
			PayoutID: payout.PayoutID, Status: dbModels.PayoutPending, Bank: payout.Bank, Phone: payout.Phone, DNI: payout.DNI, Amount: payout.Amount,
		}}
	}

	t.Run("same id, different amount", func(t *testing.T) {
		r4 := &fakeR4{}
		changed := payout
		changed.Amount = 99999
		code, out := post(newRouter(r4, &memStore{rows: taken()}, testSecret), changed, testSecret)
		if _, promised := out["sent"]; code != 409 || out["code"] != "payout_id_mismatch" || promised || r4.calls.Load() != 0 {
			t.Fatalf("got %d %v, R4 calls %d", code, out, r4.calls.Load())
		}
	})

	t.Run("same id, record unreadable", func(t *testing.T) {
		r4 := &fakeR4{}
		code, out := post(newRouter(r4, &memStore{rows: taken(), conflictUnreadable: true}, testSecret), payout, testSecret)
		if _, promised := out["sent"]; code != 200 || out["outcome"] != "unknown" || out["replayed"] != true || promised || r4.calls.Load() != 0 {
			t.Fatalf("got %d %v, R4 calls %d", code, out, r4.calls.Load())
		}
	})
}
