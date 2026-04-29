package app

import (
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type serviceActionRequest struct {
	Provider string `json:"provider"`
	Service  string `json:"service"`
	Action   string `json:"action"`
	Graceful bool   `json:"graceful"`
}

func (a *App) handleListServices(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	providerRaw := strings.TrimSpace(c.Query("provider"))
	provider, err := resolveServiceProvider(providerRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "list_services", map[string]any{"provider": provider.Name()}, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServiceAction(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req serviceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	provider, err := resolveServiceProvider(req.Provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service := strings.TrimSpace(req.Service)
	action := strings.TrimSpace(strings.ToLower(req.Action))
	if service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service is required"})
		return
	}
	switch action {
	case "start", "stop", "restart", "reload":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}
	payload := map[string]any{
		"provider": provider.Name(),
		"service":  service,
		"action":   action,
		"graceful": req.Graceful,
	}
	raw, agentErr := a.hub.RequestAgent(serverID, "service_action", payload, 30*time.Second)
	success := agentErr == nil
	errText := ""
	if agentErr != nil {
		errText = publicAgentError(agentErr)
	}
	_ = a.db.Create(&models.ServiceActionLog{
		UserID:       userID,
		ServerID:     serverID,
		Provider:     provider.Name(),
		ServiceName:  service,
		Action:       action,
		Graceful:     req.Graceful,
		Success:      success,
		ErrorMessage: errText,
	}).Error
	if agentErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": errText})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServiceLogs(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	provider, err := resolveServiceProvider(c.Query("provider"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service := strings.TrimSpace(c.Query("service"))
	if service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service is required"})
		return
	}
	lines := strings.TrimSpace(c.Query("lines"))
	if lines == "" {
		lines = "200"
	}
	raw, err := a.hub.RequestAgent(serverID, "service_logs", map[string]any{
		"provider": provider.Name(),
		"service":  service,
		"lines":    lines,
	}, 25*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, decErr := decodeRawJSON(raw)
	if decErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServiceDependencies(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	provider, err := resolveServiceProvider(c.Query("provider"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "service_graph", map[string]any{
		"provider": provider.Name(),
	}, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, decErr := decodeRawJSON(raw)
	if decErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServiceRestartHistory(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	provider, err := resolveServiceProvider(c.Query("provider"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service := strings.TrimSpace(c.Query("service"))
	if service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service is required"})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "service_restart_history", map[string]any{
		"provider": provider.Name(),
		"service":  service,
	}, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, decErr := decodeRawJSON(raw)
	if decErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServiceAuditLogs(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, parseErr := parsePositiveInt(raw); parseErr == nil && parsed > 0 {
			limit = int(parsed)
		}
	}
	if limit > 500 {
		limit = 500
	}
	var items []models.ServiceActionLog
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(limit).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load audit logs"})
		return
	}
	c.JSON(http.StatusOK, items)
}
