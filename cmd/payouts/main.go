// Command payouts serves only POST /payouts/vuelto, for a Cloud Run service that
// runs beside the main gateway (cmd/main.go) instead of replacing it.
//
// It deliberately starts nothing else: no store, payment or webhook routes, and
// above all no recurrent direct-debit retry cron. A second copy of cmd/main.go
// would run that cron too and charge customers twice.
//
// It needs only the database, R4 and PAYOUT_SECRET, so the Shopify, Drive and
// Mailgun keys never reach this service.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"appa_payments/internal/handlers"
	"appa_payments/internal/routes"
	"appa_payments/internal/services"
	"appa_payments/pkg/db"
	"appa_payments/pkg/logs"
	"appa_payments/pkg/middleware"
	"appa_payments/pkg/r4bank"
)

func main() {
	env := map[string]string{}
	for _, k := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "R4_ENTRY_POINT", "R4_API_ECOMMERCE", "R4_SECRET", "PAYOUT_SECRET"} {
		v := os.Getenv(k)
		if v == "" {
			log.Fatalf("%s is not configured", k)
		}
		env[k] = v
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger := logs.NewZapLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Printf("error syncing logger: %v\n", err)
		}
	}()

	sslmode := os.Getenv("SSL_MODE")
	if len(sslmode) > 0 {
		sslmode = "sslmode=" + sslmode
	}
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s",
		env["DB_HOST"], env["DB_PORT"], env["DB_USER"], env["DB_PASSWORD"], env["DB_NAME"], sslmode)
	gormDB, err := db.NewDBSQLHandler(connStr)
	if err != nil {
		logger.Fatal("create db handler", zap.Error(err))
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		logger.Fatal("create db connection", zap.Error(err))
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Printf("error closing db: %v\n", err)
		}
	}()

	r4Repository := r4bank.NewR4Repository(logger, env["R4_ENTRY_POINT"], env["R4_API_ECOMMERCE"], env["R4_SECRET"])
	r4bank.ConfigureVueltoRejectCodes(os.Getenv("VUELTO_REJECT_CODES"))
	payoutService := services.NewPayoutService(services.NewPayoutStore(gormDB), r4Repository, logger)
	payoutHandler := handlers.NewPayoutHandler(payoutService, middleware.NewPayoutVerifier(env["PAYOUT_SECRET"]), logger)

	// Server to server only (APPA's edge function), so no CORS.
	router := gin.Default()
	router.Use(gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "OK"})
	})
	routes.NewPayoutRoute(payoutHandler).SetRouter(router)

	if err := router.Run(":" + port); err != nil {
		logger.Fatal("router run", zap.Error(err))
	}
}
