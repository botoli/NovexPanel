package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
	fileLockMu  sync.Mutex
	fileLocks   map[string]fileLockState
}

type fileLockState struct {
	Token     string
	UserID    uint
	ExpiresAt time.Time
}

func New(cfg config.Config, db *gorm.DB) *App {
	return &App{
		cfg:         cfg,
		db:          db,
		hub:         NewHub(db),
		apiLimiter:  newFixedWindowRateLimiter(240, time.Minute),
		authLimiter: newFixedWindowRateLimiter(10, 5*time.Minute),
		fileLocks:   make(map[string]fileLockState),
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
	r.GET("/integrations/github/callback", a.handleGitHubOAuthCallback)

	authGroup := r.Group("/")
	authGroup.Use(a.userAuthMiddleware())
	{
		authGroup.GET("/auth/me", a.handleMe)
		authGroup.PATCH("/auth/me", a.handleUpdateMe)
		authGroup.POST("/auth/tokens", a.handleCreateAgentToken)
		authGroup.GET("/auth/tokens", a.handleListAgentTokens)
		authGroup.PATCH("/auth/tokens/:id", a.handleUpdateAgentTokenName)
		authGroup.DELETE("/auth/tokens/:id", a.handleRevokeAgentToken)

		authGroup.GET("/servers", a.handleListServers)
		authGroup.PATCH("/servers/:id", a.handlePatchServer)
		authGroup.GET("/servers/:id/metrics", a.handleServerMetricsHistory)
		authGroup.GET("/servers/:id/processes", a.handleServerProcesses)
		authGroup.POST("/servers/:id/command", a.handleServerCommand)
		authGroup.GET("/servers/:id/services", a.handleListServices)
		authGroup.POST("/servers/:id/services/action", a.handleServiceAction)
		authGroup.GET("/servers/:id/services/logs", a.handleServiceLogs)
		authGroup.GET("/servers/:id/services/dependencies", a.handleServiceDependencies)
		authGroup.GET("/servers/:id/services/restart-history", a.handleServiceRestartHistory)
		authGroup.GET("/servers/:id/services/audit", a.handleServiceAuditLogs)
		authGroup.GET("/servers/:id/secrets", a.handleListSecrets)
		authGroup.POST("/servers/:id/secrets", a.handleCreateSecret)
		authGroup.PATCH("/servers/:id/secrets/:secretId", a.handleUpdateSecret)
		authGroup.POST("/servers/:id/secrets/:secretId/rotate", a.handleRotateSecret)
		authGroup.POST("/servers/:id/secrets/:secretId/revoke", a.handleRevokeSecret)
		authGroup.POST("/servers/:id/secrets/:secretId/reveal", a.handleRevealSecret)
		authGroup.POST("/servers/:id/secrets/inject", a.handleInjectSecrets)
		authGroup.GET("/servers/:id/secrets/audit", a.handleSecretsAudit)
		authGroup.GET("/servers/:id/files/tree", a.handleFileTree)
		authGroup.GET("/servers/:id/files/content", a.handleFileContent)
		authGroup.POST("/servers/:id/files/validate", a.handleFileValidate)
		authGroup.POST("/servers/:id/files/lock", a.handleFileLock)
		authGroup.POST("/servers/:id/files/unlock", a.handleFileUnlock)
		authGroup.GET("/servers/:id/files/history", a.handleFileHistory)
		authGroup.GET("/servers/:id/files/audit", a.handleFileAudit)
		authGroup.POST("/servers/:id/files/apply", a.handleFileApply)
		authGroup.POST("/servers/:id/files/rollback", a.handleFileRollback)
		authGroup.POST("/servers/:id/deploy", a.handleServerDeploy)
		authGroup.POST("/servers/:id/runbooks", a.handleCreateRunbook)
		authGroup.GET("/servers/:id/runbooks", a.handleListRunbooks)
		authGroup.GET("/servers/:id/runbooks/:runbookId", a.handleGetRunbook)
		authGroup.PATCH("/servers/:id/runbooks/:runbookId", a.handlePatchRunbook)
		authGroup.DELETE("/servers/:id/runbooks/:runbookId", a.handleDeleteRunbook)
		authGroup.GET("/servers/:id/runbooks/:runbookId/versions", a.handleListRunbookVersions)
		authGroup.POST("/servers/:id/runbooks/:runbookId/versions", a.handleCreateRunbookVersion)
		authGroup.POST("/servers/:id/runbooks/:runbookId/rollback-version", a.handleRunbookRollbackVersion)
		authGroup.GET("/servers/:id/runbooks/:runbookId/diff", a.handleRunbookDiff)
		authGroup.POST("/servers/:id/runbooks/:runbookId/dry-run", a.handleRunbookDryRun)
		authGroup.POST("/servers/:id/runbooks/:runbookId/execute", a.handleExecuteRunbook)
		authGroup.GET("/servers/:id/runbooks/:runbookId/executions", a.handleListRunbookExecutions)
		authGroup.GET("/servers/:id/runbooks/executions/:executionId/logs", a.handleRunbookExecutionLogs)
		authGroup.GET("/servers/:id/runbooks/:runbookId/audit", a.handleRunbookAudit)
		authGroup.POST("/servers/:id/runbooks/rollback", a.handleRollbackRunbookExecution)
		authGroup.DELETE("/servers/:id/processes/:pid", a.handleKillServerProcess)
		authGroup.POST("/servers/:id/processes/:pid/stop", a.handleStopServerProcess)
		authGroup.POST("/servers/:id/processes/:pid/restart", a.handleRestartServerProcess)
		authGroup.DELETE("/servers/:id", a.handleDeleteServer)

		authGroup.POST("/deploy", a.handleCreateDeploy)
		authGroup.GET("/deploys", a.handleListDeploys)
		authGroup.GET("/deploys/:id", a.handleGetDeploy)
		authGroup.GET("/deploys/:id/log", a.handleDeployLog)
		authGroup.POST("/deploys/:id/stop", a.handleStopDeploy)
		authGroup.POST("/deploys/:id/redeploy", a.handleRedeploy)
		authGroup.DELETE("/deploys/:id", a.handleDeleteDeploy)
		authGroup.GET("/deploys/:id/logs", a.handleDeployLogs)

		authGroup.GET("/integrations/github/start", a.handleGitHubOAuthStart)
		authGroup.GET("/integrations/github", a.handleGetGitHubConnection)
		authGroup.GET("/integrations/github/repos", a.handleListGitHubRepos)
		authGroup.POST("/integrations/github/disconnect", a.handleDisconnectGitHub)

		authGroup.GET("/settings/members", a.handleListMembers)
		authGroup.POST("/settings/members", a.handleCreateMember)
		authGroup.DELETE("/settings/members/:id", a.handleDeleteMember)
		authGroup.POST("/settings/api-tokens", a.handleCreateAPIToken)
		authGroup.GET("/settings/api-tokens", a.handleListAPITokens)
		authGroup.DELETE("/settings/api-tokens/:id", a.handleRevokeAPIToken)
	}

	r.GET("/agent/ws", a.handleAgentWS)
	r.GET("/site/ws", a.handleSiteWS)
	r.GET("/terminal/:id", a.handleTerminalWS)

	return r
}

func (a *App) StartBackgroundJobs(ctx context.Context) {
	go a.metricsRetentionWorker(ctx, time.Hour)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.rotateExpiredSecretsTick()
			}
		}
	}()
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
