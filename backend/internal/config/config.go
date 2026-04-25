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
	HTTPAddr            string
	DatabaseURL         string
	JWTSecret           string
	JWTExpiry           time.Duration
	AgentTokenExpiry    time.Duration
	MetricsHistoryDays  int
	SiteAllowedOrigins  []string
	MaxRequestBodyBytes int64
	WSReadLimitBytes    int64
	CORSAllowAll        bool
	// CommandAllowlist: comma-separated allowed command prefixes (see app package). Empty uses built-in defaults. "*" disables filtering.
	CommandAllowlist string
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
	if len(strings.TrimSpace(cfg.JWTSecret)) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	jwtExpiry, err := parseDurationWithDays(getEnv("JWT_EXPIRY", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse JWT_EXPIRY: %w", err)
	}
	if jwtExpiry <= 0 {
		return Config{}, fmt.Errorf("JWT_EXPIRY must be > 0")
	}
	if jwtExpiry > 30*24*time.Hour {
		return Config{}, fmt.Errorf("JWT_EXPIRY must be <= 30d")
	}
	cfg.JWTExpiry = jwtExpiry

	agentExpiry, err := parseDurationWithDays(getEnv("AGENT_TOKEN_EXPIRY", "365d"))
	if err != nil {
		return Config{}, fmt.Errorf("parse AGENT_TOKEN_EXPIRY: %w", err)
	}
	if agentExpiry <= 0 {
		return Config{}, fmt.Errorf("AGENT_TOKEN_EXPIRY must be > 0")
	}
	cfg.AgentTokenExpiry = agentExpiry

	daysRaw := getEnv("METRICS_HISTORY_DAYS", "7")
	days, err := strconv.Atoi(daysRaw)
	if err != nil || days < 1 {
		return Config{}, fmt.Errorf("METRICS_HISTORY_DAYS must be integer > 0")
	}
	cfg.MetricsHistoryDays = days

	maxBodyBytes, err := parseInt64WithMin(getEnv("MAX_REQUEST_BODY_BYTES", "8388608"), 1024)
	if err != nil {
		return Config{}, fmt.Errorf("MAX_REQUEST_BODY_BYTES: %w", err)
	}
	cfg.MaxRequestBodyBytes = maxBodyBytes

	wsReadLimitBytes, err := parseInt64WithMin(getEnv("WS_READ_LIMIT_BYTES", "262144"), 4096)
	if err != nil {
		return Config{}, fmt.Errorf("WS_READ_LIMIT_BYTES: %w", err)
	}
	cfg.WSReadLimitBytes = wsReadLimitBytes

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

	cfg.CommandAllowlist = strings.TrimSpace(getEnv("COMMAND_ALLOWLIST", ""))

	cfg.CORSAllowAll = strings.EqualFold(strings.TrimSpace(getEnv("CORS_ALLOW_ALL", "false")), "true")

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

func parseInt64WithMin(raw string, minValue int64) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed < minValue {
		return 0, fmt.Errorf("must be >= %d", minValue)
	}
	return parsed, nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
