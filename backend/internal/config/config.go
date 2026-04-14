package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	JWTSecret          string
	JWTExpiry          time.Duration
	AgentTokenExpiry   time.Duration
	MetricsHistoryDays int
	SiteAllowedOrigins []string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8380"),
		DatabaseURL: getEnv("DATABASE_URL", "novex.db"),
		JWTSecret:   getEnv("JWT_SECRET", ""),
	}

	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required") 
	}

	jwtExpiry, err := parseDurationWithDays(getEnv("JWT_EXPIRY", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse JWT_EXPIRY: %w", err)
	}
	cfg.JWTExpiry = jwtExpiry

	agentExpiry, err := parseDurationWithDays(getEnv("AGENT_TOKEN_EXPIRY", "365d"))
	if err != nil {
		return Config{}, fmt.Errorf("parse AGENT_TOKEN_EXPIRY: %w", err)
	}
	cfg.AgentTokenExpiry = agentExpiry

	daysRaw := getEnv("METRICS_HISTORY_DAYS", "7")
	days, err := strconv.Atoi(daysRaw)
	if err != nil || days < 1 {
		return Config{}, fmt.Errorf("METRICS_HISTORY_DAYS must be integer > 0")
	}
	cfg.MetricsHistoryDays = days

	origins := strings.TrimSpace(getEnv("SITE_ALLOWED_ORIGINS", "*"))
	if origins == "*" || origins == "" {
		cfg.SiteAllowedOrigins = []string{"*"}
	} else {
		parts := strings.Split(origins, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			out = []string{"*"}
		}
		cfg.SiteAllowedOrigins = out
	}

	return cfg, nil
}

func parseDurationWithDays(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, "d") {
		daysRaw := strings.TrimSuffix(raw, "d")
		days, err := strconv.Atoi(daysRaw)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(raw)
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
