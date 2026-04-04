package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Public URL for OAuth issuer
	PublicURL string

	// Admin credentials (single identity)
	AdminUser     string
	AdminPassword string // bcrypt hash

	// DynamoDB
	DynamoDBTable string

	// Secrets Manager ARNs
	JWTSigningKeyARN  string
	WGConfigSecretARN string

	// Token TTLs
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AuthCodeTTL     time.Duration

	// OpenTelemetry
	OTelEndpoint    string
	OTelServiceName string

	// Printer configuration
	PrinterIP   string
	PrinterName string

	// Cache TTLs
	CacheMemoryTTL time.Duration
	CacheDynamoTTL time.Duration

	// Alexa
	AlexaSkillID string
}

func Load() (*Config, error) {
	cfg := &Config{
		PublicURL:         envOrDefault("PUBLIC_URL", "http://localhost:3000"),
		AdminUser:         envOrDefault("ADMIN_USER", "admin"),
		AdminPassword:     os.Getenv("ADMIN_PASSWORD"),
		DynamoDBTable:     envOrDefault("DYNAMODB_TABLE", "mcp-brother-label-printer-direct-oauth"),
		JWTSigningKeyARN:  os.Getenv("JWT_SIGNING_KEY_ARN"),
		WGConfigSecretARN: os.Getenv("WG_CONFIG_SECRET_ARN"),
		OTelEndpoint:      envOrDefault("OTEL_ENDPOINT", "http://192.168.1.202:4318"),
		OTelServiceName:   envOrDefault("OTEL_SERVICE_NAME", "mcp-brother-label-printer-direct"),
		PrinterIP:         envOrDefault("PRINTER_IP", "192.168.1.243"),
		PrinterName:       envOrDefault("PRINTER_NAME", "Brother PT-P750W"),
		AccessTokenTTL:    envDuration("ACCESS_TOKEN_TTL", 3600),
		RefreshTokenTTL:   envDuration("REFRESH_TOKEN_TTL", 604800),
		AuthCodeTTL:       envDuration("AUTH_CODE_TTL", 300),
		CacheMemoryTTL:    envDuration("CACHE_MEMORY_TTL_SEC", 300),
		CacheDynamoTTL:    envDuration("CACHE_DYNAMO_TTL_SEC", 3600),
		AlexaSkillID:      os.Getenv("ALEXA_SKILL_ID"),
	}

	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, defaultSec int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			return time.Duration(sec) * time.Second
		}
	}
	return time.Duration(defaultSec) * time.Second
}
