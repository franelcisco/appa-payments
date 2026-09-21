package services

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/lib/pq" // cmd/main.go registers it for the real binary

	"appa_payments/pkg/db"
	dbModels "appa_payments/pkg/db/models"
)

// The pay-once guarantee rests on Postgres, so it is tested against Postgres:
//
//	PAYOUT_TEST_DSN="host=127.0.0.1 port=5432 user=postgres dbname=postgres sslmode=disable" go test ./internal/services/
//
// The database needs r4_appa_payouts from pkg/db/schema.sql. Skipped otherwise.
func TestPayoutStoreReserveIsAtomic(t *testing.T) {
	dsn := os.Getenv("PAYOUT_TEST_DSN")
	if dsn == "" {
		t.Skip("PAYOUT_TEST_DSN not set")
	}
	gormDB, err := db.NewDBSQLHandler(dsn)
	if err != nil {
		t.Fatal(err)
	}
	store := NewPayoutStore(gormDB)
	ctx := context.Background()
	const id = "store-test-1"
	gormDB.Exec("DELETE FROM r4_appa_payouts WHERE payout_id = ?", id)

	var created atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.Reserve(ctx, &dbModels.R4AppaPayout{
				PayoutID: id, Status: dbModels.PayoutPending, Bank: "0102", Phone: "04141234567", DNI: "V1", Amount: 18030.75,
			})
			if err != nil {
				t.Error(err)
			}
			if ok {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := created.Load(); n != 1 {
		t.Fatalf("%d of 16 concurrent reservations won, want exactly 1", n)
	}

	if err := store.Finish(ctx, id, dbModels.PayoutConfirmed, "555", ""); err != nil {
		t.Fatal(err)
	}
	// Finish only moves a pending row: a late second writer can't overwrite it,
	// and is told so rather than failing silently.
	if err := store.Finish(ctx, id, dbModels.PayoutRejected, "", "late"); err == nil {
		t.Fatal("a second Finish on a row that is no longer pending must return an error")
	}
	existing, ok, err := store.Reserve(ctx, &dbModels.R4AppaPayout{PayoutID: id, Status: dbModels.PayoutPending, Bank: "0102", Phone: "04141234567", DNI: "V1", Amount: 18030.75})
	if err != nil || ok {
		t.Fatalf("re-reserve: created=%v err=%v", ok, err)
	}
	if existing.Status != dbModels.PayoutConfirmed || existing.Reference != "555" || existing.Amount != 18030.75 {
		t.Fatalf("stored row = %+v", existing)
	}
}
