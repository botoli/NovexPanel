package app

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsPongWait     = 90 * time.Second
	wsPingInterval = 30 * time.Second
)

func (a *App) newWSUpgrader(requireOrigin bool) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			return a.isRequestOriginAllowed(origin, requireOrigin)
		},
	}
}

func (a *App) wsReadLimit(defaultLimit int64) int64 {
	if a.cfg.WSReadLimitBytes > 0 {
		return a.cfg.WSReadLimitBytes
	}
	if defaultLimit > 0 {
		return defaultLimit
	}
	return 256 * 1024
}

func (a *App) isRequestOriginAllowed(origin string, requireOrigin bool) bool {
	if a.cfg.CORSAllowAll {
		if origin == "" {
			return !requireOrigin
		}
		return true
	}

	origin = strings.TrimSpace(origin)
	if origin == "" {
		return !requireOrigin
	}

	normalizedOrigin, ok := normalizeOrigin(origin)
	if !ok {
		return false
	}

	if len(a.cfg.SiteAllowedOrigins) == 0 {
		return false
	}

	for _, allowed := range a.cfg.SiteAllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" {
			return true
		}

		normalizedAllowed, normalized := normalizeOrigin(allowed)
		if !normalized {
			if strings.EqualFold(allowed, origin) {
				return true
			}
			continue
		}
		if normalizedAllowed == normalizedOrigin {
			return true
		}
	}

	return false
}

func normalizeOrigin(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", false
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), true
}

func configureWSReadSettings(conn *websocket.Conn, readLimit int64) {
	if readLimit > 0 {
		conn.SetReadLimit(readLimit)
	}
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
}
