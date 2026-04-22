package app

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindowEntry struct {
	Count   int
	ResetAt time.Time
}

type fixedWindowRateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateWindowEntry
	limit   int
	window  time.Duration
}

func newFixedWindowRateLimiter(limit int, window time.Duration) *fixedWindowRateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &fixedWindowRateLimiter{
		entries: make(map[string]rateWindowEntry),
		limit:   limit,
		window:  window,
	}
}

func (l *fixedWindowRateLimiter) Allow(key string) bool {
	now := time.Now()
	if strings.TrimSpace(key) == "" {
		key = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || now.After(entry.ResetAt) {
		l.entries[key] = rateWindowEntry{Count: 1, ResetAt: now.Add(l.window)}
		if len(l.entries) > 20000 {
			l.cleanupExpired(now)
		}
		return true
	}

	if entry.Count >= l.limit {
		return false
	}

	entry.Count++
	l.entries[key] = entry
	return true
}

func (l *fixedWindowRateLimiter) RetryAfter(key string) int {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok {
		return int(l.window.Seconds())
	}
	if now.After(entry.ResetAt) {
		return 1
	}
	seconds := int(entry.ResetAt.Sub(now).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (l *fixedWindowRateLimiter) cleanupExpired(now time.Time) {
	for key, entry := range l.entries {
		if now.After(entry.ResetAt) {
			delete(l.entries, key)
		}
	}
}

func (a *App) requestBodyLimitMiddleware() gin.HandlerFunc {
	maxBytes := a.cfg.MaxRequestBodyBytes
	if maxBytes <= 0 {
		maxBytes = 8 << 20
	}

	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func (a *App) globalRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "/healthz" {
			c.Next()
			return
		}

		key := "api:" + c.ClientIP()
		if a.apiLimiter.Allow(key) {
			c.Next()
			return
		}

		retryAfter := a.apiLimiter.RetryAfter(key)
		c.Header("Retry-After", toString(retryAfter))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
	}
}

func (a *App) authRateLimitMiddleware(action string) gin.HandlerFunc {
	action = strings.TrimSpace(strings.ToLower(action))
	if action == "" {
		action = "auth"
	}

	return func(c *gin.Context) {
		key := "auth:" + action + ":" + c.ClientIP()
		if a.authLimiter.Allow(key) {
			c.Next()
			return
		}

		retryAfter := a.authLimiter.RetryAfter(key)
		c.Header("Retry-After", toString(retryAfter))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many auth attempts"})
	}
}

func (a *App) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")

		isHTTPS := c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
		if isHTTPS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

func toString(v int) string {
	if v <= 0 {
		return "1"
	}
	return strconv.Itoa(v)
}
