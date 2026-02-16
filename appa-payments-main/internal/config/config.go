package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Port  string
	Debug string

	CORSAllowedOrigins []string

	// DATABASE
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	SSLMode    string

	// Shopify API credentials
	ShopifyAPIVersion string
	ShopifyAdminToken string
	ShopifyStoreName  string

	// R4 API credentials
	R4EntryPoint   string
	R4Secret       string
	R4APIEcommerce string

	// Google Drive
	GoogleDriveFolderID string
	GoogleCredentials   string
	GoogleDriveToken    string
}

// Load reads configuration from environment variables and returns a Config struct
func Load() (*Config, error) {
	corsOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")

	cfg := &Config{
		Port:               os.Getenv("PORT"),
		Debug:              os.Getenv("DEBUG"),
		CORSAllowedOrigins: corsOrigins,
		// DATABASE
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		SSLMode:    os.Getenv("SSL_MODE"),

		ShopifyAPIVersion: os.Getenv("SHOPIFY_API_VERSION"),
		ShopifyAdminToken: os.Getenv("SHOPIFY_ADMIN_TOKEN"),
		ShopifyStoreName:  os.Getenv("SHOPIFY_STORE_NAME"),

		R4EntryPoint:   os.Getenv("R4_ENTRY_POINT"),
		R4Secret:       os.Getenv("R4_SECRET"),
		R4APIEcommerce: os.Getenv("R4_API_ECOMMERCE"),

		GoogleDriveFolderID: os.Getenv("GOOGLE_DRIVE_FOLDER_ID"),
		GoogleCredentials:   os.Getenv("GOOGLE_CREDENTIALS"),
		GoogleDriveToken:    os.Getenv("GOOGLE_DRIVE_TOKEN"),
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {

	if cfg.DBHost == "" {
		return fmt.Errorf("DBHost is not configured")
	}
	if cfg.DBPort == "" {
		return fmt.Errorf("DBPort is not configured")
	}
	if cfg.DBUser == "" {
		return fmt.Errorf("DBUser is not configured")
	}
	if cfg.DBPassword == "" {
		return fmt.Errorf("DBPassword is not configured")
	}
	if cfg.DBName == "" {
		return fmt.Errorf("DBName is not configured")
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		return fmt.Errorf("CORSAllowedOrigins is not configured")
	}

	if cfg.ShopifyAPIVersion == "" {
		return fmt.Errorf("ShopifyAPIVersion is not configured")
	}
	if cfg.ShopifyAdminToken == "" {
		return fmt.Errorf("ShopifyAdminToken is not configured")
	}
	if cfg.ShopifyStoreName == "" {
		return fmt.Errorf("ShopifyStoreName is not configured")
	}

	if cfg.R4EntryPoint == "" {
		return fmt.Errorf("R4EntryPoint is not configured")
	}
	if cfg.R4Secret == "" {
		return fmt.Errorf("R4Secret is not configured")
	}
	if cfg.R4APIEcommerce == "" {
		return fmt.Errorf("R4APIEcommerce is not configured")
	}

	if cfg.GoogleDriveFolderID == "" {
		return fmt.Errorf("GoogleDriveFolderID is not configured")
	}
	if cfg.GoogleCredentials == "" {
		return fmt.Errorf("GoogleCredentials is not configured")
	}
	if cfg.GoogleDriveToken == "" {
		return fmt.Errorf("GoogleDriveToken is not configured")
	}

	return nil
}
