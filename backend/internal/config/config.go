package config

import (
	"os"
	"strconv"
)

// Config holds all environment-based configuration.
type Config struct {
	Port           string
	DatabaseURL    string
	DatabaseURLMig string
	DBMaxConns     int32
	JWTSecret      string
	AppEnv         string
	AllowedOrigins string
	CookieSecure   bool
	TZ             string
}

// Load reads configuration from environment variables.
func Load() *Config {
	maxConns, _ := strconv.ParseInt(getEnv("DB_MAX_CONNS", "10"), 10, 32)
	if maxConns < 1 {
		maxConns = 10
	}

	cookieSecure, _ := strconv.ParseBool(getEnv("COOKIE_SECURE", "false"))

	return &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		DatabaseURLMig: os.Getenv("DATABASE_URL_MIGRATE"),
		DBMaxConns:     int32(maxConns),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AppEnv:         getEnv("APP_ENV", "development"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		CookieSecure:   cookieSecure,
		TZ:             getEnv("TZ", "Asia/Jakarta"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

