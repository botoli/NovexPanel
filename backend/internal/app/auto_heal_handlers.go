package app

import (
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type autoHealRuleInput struct {
	Name            string  `json:"name"`
	Metric          string  `json:"metric"`
	Condition       string  `json:"condition"`
	Threshold       float64 `json:"threshold"`
	DurationSeconds int     `json:"duration_seconds"`
	Action          string  `json:"action"`
	RetryLimit      int     `json:"retry_limit"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Enabled         *bool   `json:"enabled"`
}

func (a *App) handleListAutoHealRules(c *gin.Context) {
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

	var rules []models.AutoHealRule
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load auto-heal rules"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (a *App) handleCreateAutoHealRule(c *gin.Context) {
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

	var req autoHealRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	rule, err := normalizeAutoHealInput(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.UserID = userID
	rule.ServerID = serverID

	if err := a.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create auto-heal rule"})
		return
	}

	_ = a.db.Create(&models.AutoHealEvent{
		UserID:    userID,
		ServerID:  serverID,
		RuleID:    &rule.ID,
		RuleName:  rule.Name,
		Status:    "created",
		Message:   "Rule created from UI",
		CreatedAt: time.Now().UTC(),
	}).Error

	c.JSON(http.StatusCreated, rule)
}

func (a *App) handleAutoHealHistory(c *gin.Context) {
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

	var events []models.AutoHealEvent
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(limit).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load auto-heal history"})
		return
	}
	c.JSON(http.StatusOK, events)
}

func normalizeAutoHealInput(req autoHealRuleInput) (models.AutoHealRule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.AutoHealRule{}, badRequestError{msg: "name is required"}
	}
	metric := strings.TrimSpace(strings.ToLower(req.Metric))
	if metric == "" {
		return models.AutoHealRule{}, badRequestError{msg: "metric is required"}
	}
	condition := strings.TrimSpace(strings.ToLower(req.Condition))
	switch condition {
	case "gt", "lt", "eq":
	default:
		return models.AutoHealRule{}, badRequestError{msg: "condition must be gt, lt or eq"}
	}
	action := strings.TrimSpace(strings.ToLower(req.Action))
	switch action {
	case "restart", "reload", "runbook", "alert", "rollback_deploy", "block_traffic":
	default:
		return models.AutoHealRule{}, badRequestError{msg: "unsupported action"}
	}
	if req.DurationSeconds <= 0 {
		return models.AutoHealRule{}, badRequestError{msg: "duration_seconds must be > 0"}
	}
	if req.RetryLimit < 0 {
		return models.AutoHealRule{}, badRequestError{msg: "retry_limit must be >= 0"}
	}
	if req.CooldownSeconds < 0 {
		return models.AutoHealRule{}, badRequestError{msg: "cooldown_seconds must be >= 0"}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return models.AutoHealRule{
		Name:            name,
		Metric:          metric,
		Condition:       condition,
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Action:          action,
		RetryLimit:      req.RetryLimit,
		CooldownSeconds: req.CooldownSeconds,
		Enabled:         enabled,
	}, nil
}

type badRequestError struct{ msg string }

func (e badRequestError) Error() string { return e.msg }
package app

import (
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type autoHealRuleInput struct {
	Name            string  `json:"name"`
	Metric          string  `json:"metric"`
	Condition       string  `json:"condition"`
	Threshold       float64 `json:"threshold"`
	DurationSeconds int     `json:"duration_seconds"`
	Action          string  `json:"action"`
	RetryLimit      int     `json:"retry_limit"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Enabled         *bool   `json:"enabled"`
}

func (a *App) handleListAutoHealRules(c *gin.Context) {
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

	var rules []models.AutoHealRule
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load auto-heal rules"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (a *App) handleCreateAutoHealRule(c *gin.Context) {
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

	var req autoHealRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	rule, err := normalizeAutoHealInput(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.UserID = userID
	rule.ServerID = serverID

	if err := a.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create auto-heal rule"})
		return
	}

	_ = a.db.Create(&models.AutoHealEvent{
		UserID:    userID,
		ServerID:  serverID,
		RuleID:    &rule.ID,
		RuleName:  rule.Name,
		Status:    "created",
		Message:   "Rule created from UI",
		CreatedAt: time.Now().UTC(),
	}).Error

	c.JSON(http.StatusCreated, rule)
}

func (a *App) handleAutoHealHistory(c *gin.Context) {
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

	var events []models.AutoHealEvent
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(limit).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load auto-heal history"})
		return
	}
	c.JSON(http.StatusOK, events)
}

func normalizeAutoHealInput(req autoHealRuleInput) (models.AutoHealRule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.AutoHealRule{}, errBadRequest("name is required")
	}
	metric := strings.TrimSpace(strings.ToLower(req.Metric))
	if metric == "" {
		return models.AutoHealRule{}, errBadRequest("metric is required")
	}
	condition := strings.TrimSpace(strings.ToLower(req.Condition))
	switch condition {
	case "gt", "lt", "eq":
	default:
		return models.AutoHealRule{}, errBadRequest("condition must be gt, lt or eq")
	}
	action := strings.TrimSpace(strings.ToLower(req.Action))
	switch action {
	case "restart", "reload", "runbook", "alert", "rollback_deploy", "block_traffic":
	default:
		return models.AutoHealRule{}, errBadRequest("unsupported action")
	}
	if req.DurationSeconds <= 0 {
		return models.AutoHealRule{}, errBadRequest("duration_seconds must be > 0")
	}
	if req.RetryLimit < 0 {
		return models.AutoHealRule{}, errBadRequest("retry_limit must be >= 0")
	}
	if req.CooldownSeconds < 0 {
		return models.AutoHealRule{}, errBadRequest("cooldown_seconds must be >= 0")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return models.AutoHealRule{
		Name:            name,
		Metric:          metric,
		Condition:       condition,
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Action:          action,
		RetryLimit:      req.RetryLimit,
		CooldownSeconds: req.CooldownSeconds,
		Enabled:         enabled,
	}, nil
}

type badRequestError struct{ msg string }

func (e badRequestError) Error() string { return e.msg }
func errBadRequest(msg string) error    { return badRequestError{msg: msg} }
