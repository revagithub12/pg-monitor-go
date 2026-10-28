package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	TargetDBURL     string
	RedisAddr       string // optional: empty disables alert publishing to Redis
	JWTSecret       string
	AdminUser       string
	AdminPassword   string
	ViewerUser      string
	ViewerPassword  string
	CollectInterval time.Duration
}

func Load() Config {
	secs, err := strconv.Atoi(getenv("COLLECT_INTERVAL_SECONDS", "5"))
	if err != nil || secs < 1 {
		secs = 5
	}
	return Config{
		Port:            getenv("PORT", "8080"),
		TargetDBURL:     getenv("TARGET_DB_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"),
		RedisAddr:       getenv("REDIS_ADDR", ""),
		JWTSecret:       getenv("JWT_SECRET", "dev-only-secret-change-me"),
		AdminUser:       getenv("ADMIN_USER", "admin"),
		AdminPassword:   getenv("ADMIN_PASSWORD", "admin12345"),
		ViewerUser:      getenv("VIEWER_USER", "viewer"),
		ViewerPassword:  getenv("VIEWER_PASSWORD", "viewer12345"),
		CollectInterval: time.Duration(secs) * time.Second,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
