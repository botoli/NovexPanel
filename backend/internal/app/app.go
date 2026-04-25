package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"novexpanel/backend/internal/auth"
	"novexpanel/backend/internal/config"
	"novexpanel/backend/internal/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const contextUserIDKey = "user_id"

type App struct {
	cfg config.Config
	db  *gorm.DB
	hub *Hub

	apiLimiter  *fixedWindowRateLimiter
	authLimiter *fixedWindowRateLimiter
}

func New(cfg config.Config, db *gorm.DB) *App {
	return &App{
		cfg:         cfg,
		db:          db,
		hub:         NewHub(db),
		apiLimiter:  newFixedWindowRateLimiter(240, time.Minute),
		authLimiter: newFixedWindowRateLimiter(10, 5*time.Minute),
	}
}

func (a *App) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	corsCfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	if a.cfg.CORSAllowAll {
		corsCfg.AllowAllOrigins = true
		corsCfg.AllowCredentials = true
		corsCfg.AllowHeaders = []string{"*"}
	} else if len(a.cfg.SiteAllowedOrigins) == 1 && a.cfg.SiteAllowedOrigins[0] == "*" {
		corsCfg.AllowAllOrigins = true
		corsCfg.AllowCredentials = false
	} else {
		corsCfg.AllowOrigins = a.cfg.SiteAllowedOrigins
	}
	r.Use(cors.New(corsCfg))
	r.Use(a.securityHeadersMiddleware(), a.requestBodyLimitMiddleware(), a.globalRateLimitMiddleware())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/auth/register", a.authRateLimitMiddleware("register"), a.handleRegister)
	r.POST("/auth/login", a.authRateLimitMiddleware("login"), a.handleLogin)

	authGroup := r.Group("/")
	authGroup.Use(a.userAuthMiddleware())
	{
		authGroup.GET("/auth/me", a.handleMe)
		authGroup.POST("/auth/tokens", a.handleCreateAgentToken)
		authGroup.GET("/auth/tokens", a.handleListAgentTokens)
		authGroup.PATCH("/auth/tokens/:id", a.handleUpdateAgentTokenName)
		authGroup.DELETE("/auth/tokens/:id", a.handleRevokeAgentToken)

		authGroup.GET("/servers", a.handleListServers)
		authGroup.PATCH("/servers/:id", a.handlePatchServer)
		authGroup.GET("/servers/:id/metrics", a.handleServerMetricsHistory)
		authGroup.GET("/servers/:id/processes", a.handleServerProcesses)
		authGroup.POST("/servers/:id/command", a.handleServerCommand)
		authGroup.POST("/servers/:id/deploy", a.handleServerDeploy)
		authGroup.DELETE("/servers/:id/processes/:pid", a.handleKillServerProcess)
		authGroup.DELETE("/servers/:id", a.handleDeleteServer)

		authGroup.POST("/deploy", a.handleCreateDeploy)
		authGroup.GET("/deploys", a.handleListDeploys)
		authGroup.GET("/deploys/:id", a.handleGetDeploy)
		authGroup.GET("/deploys/:id/log", a.handleDeployLog)
		authGroup.DELETE("/deploys/:id", a.handleDeleteDeploy)
		authGroup.GET("/deploys/:id/logs", a.handleDeployLogs)
	}

	r.GET("/agent/ws", a.handleAgentWS)
	r.GET("/site/ws", a.handleSiteWS)
	r.GET("/terminal/:id", a.handleTerminalWS)

	return r
}

func (a *App) StartBackgroundJobs(ctx context.Context) {
	go a.metricsRetentionWorker(ctx, time.Hour)
}

func (a *App) userAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header"})
			return
		}

		claims, err := auth.ParseUserToken(a.cfg.JWTSecret, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(contextUserIDKey, claims.UserID)
		c.Next()
	}
}

func userIDFromContext(c *gin.Context) (uint, bool) {
	v, ok := c.Get(contextUserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok
}

func parseUintParam(c *gin.Context, key string) (uint, error) {
	raw := c.Param(key)
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed == 0 {
		return 0, errors.New("must be positive")
	}
	return uint(parsed), nil
}

func (a *App) requireServerForUser(userID, serverID uint) (*models.Server, error) {
	var server models.Server
	if err := a.db.Where("id = ? AND user_id = ?", serverID, userID).First(&server).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &server, nil
}

func (a *App) metricsRetentionWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	a.cleanupOldMetrics()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.cleanupOldMetrics()
		}
	}
}

func (a *App) cleanupOldMetrics() {
	cutoff := time.Now().Add(-time.Duration(a.cfg.MetricsHistoryDays) * 24 * time.Hour)
	a.db.Where("timestamp < ?", cutoff).Delete(&models.MetricPoint{})
}
