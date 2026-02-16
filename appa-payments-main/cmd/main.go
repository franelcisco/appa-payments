package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"appa_payments/internal/config"
	"appa_payments/internal/handlers"
	"appa_payments/internal/routes"
	"appa_payments/internal/services"
	"appa_payments/pkg/bcv"
	"appa_payments/pkg/db"
	"appa_payments/pkg/drive"
	"appa_payments/pkg/logs"
	"appa_payments/pkg/r4bank"
	"appa_payments/pkg/shopify"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	logger := logs.NewZapLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Printf("error syncing logger: %v\n", err)
		}
	}()

	sslmode := cfg.SSLMode
	fmt.Printf("sslmode -> %s\n", sslmode)
	if len(sslmode) > 0 {
		sslmode = "sslmode=" + sslmode
	}

	//connect the database
	connStr := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s %s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, sslmode)
	// gorm connect
	gormDB, err := db.NewDBSQLHandler(connStr)
	if err != nil {
		logger.Fatal("create db handler", zap.Error(err))
	}

	db, err := gormDB.DB()
	if err != nil {
		logger.Fatal("create db connection", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Printf("error db body: %v\n", err)
		}
	}()

	loc, err := time.LoadLocation("America/Caracas")
	if err != nil {
		logger.Fatal("could not load Venezuela time zone", zap.Error(err))
	}

	router := gin.Default()
	router.Use(gin.Recovery())

	if cfg.Debug == "1" {
		router.Use(cors.Default())
	} else {
		router.Use(cors.New(cors.Config{
			AllowOrigins: cfg.CORSAllowedOrigins,
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Content-Type", "Authorization"},
		}))
	}
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "OK",
		})
	})

	// initialize resources
	shopifyRepo := shopify.NewRepository(
		cfg.ShopifyStoreName, cfg.ShopifyAPIVersion, cfg.ShopifyAdminToken, logger,
	)

	r4Repository := r4bank.NewR4Repository(logger, cfg.R4EntryPoint, cfg.R4APIEcommerce, cfg.R4Secret)

	bcvClient := bcv.NewClient(r4Repository, loc, logger)
	_, err = bcvClient.Get(context.Background())
	if err != nil {
		logger.Error("could not connect to BCV client", zap.Error(err))
	}

	driveClient, err := drive.NewClient(
		context.Background(), cfg.GoogleCredentials, cfg.GoogleDriveFolderID, cfg.GoogleDriveToken, logger,
	)
	if err != nil {
		logger.Error("could not create drive client", zap.Error(err))
	}

	// initialize services
	storeService := services.NewStoreService(shopifyRepo, r4Repository, gormDB, bcvClient, logger)
	paymentService := services.NewPaymentService(gormDB, shopifyRepo, r4Repository, bcvClient, driveClient, loc, logger)

	// initialize handlers
	storeHandler := handlers.NewStoreHandler(storeService)
	paymentHandler := handlers.NewPaymentHandler(paymentService, bcvClient)

	// initialize routes
	storeRoutes := routes.NewStoreRoute(storeHandler)
	paymentRoute := routes.NewPaymentRoute(paymentHandler)

	// set routes
	storeRoutes.SetRouter(router)
	paymentRoute.SetRouter(router)

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
