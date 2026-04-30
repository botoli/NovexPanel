package app

import (
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type aiRiskAnalyzeRequest struct {
	Command  string `json:"command"`
	ServerID uint   `json:"server_id"`
	Cwd      string `json:"cwd"`
}

type aiGuardExecuteRequest struct {
	Command  string `json:"command"`
	ServerID uint   `json:"server_id"`
	RiskAck  bool   `json:"risk_ack"`
	Reason   string `json:"reason"`
}

func (a *App) handleAICommandGuardAnalyze(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req aiRiskAnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.Command = strings.TrimSpace(req.Command)
	if req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}
	if _, err := a.requireServerForUser(userID, req.ServerID); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	riskLevel, blocked, findings, policyMatches := assessCommandRisk(req.Command)
	requiresConfirmation := riskLevel == "high" || riskLevel == "critical"
	summary := "Command looks safe for execution."
	if blocked {
		summary = "Command is blocked by policy."
	} else if requiresConfirmation {
		summary = "Command is risky and requires explicit confirmation."
	}

	c.JSON(http.StatusOK, gin.H{
		"risk_level":            riskLevel,
		"summary":               summary,
		"findings":              findings,
		"blocked":               blocked,
		"requires_confirmation": requiresConfirmation,
		"policy_matches":        policyMatches,
		"sandbox_preview": []string{
			"Validate command syntax and shell operators",
			"Check command against deny-list and risky patterns",
			"Require explicit ack for high-risk operations",
		},
	})
}

func (a *App) handleAICommandGuardExecute(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req aiGuardExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.Command = strings.TrimSpace(req.Command)
	if req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}
	if _, err := a.requireServerForUser(userID, req.ServerID); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	riskLevel, blocked, findings, _ := assessCommandRisk(req.Command)
	if blocked {
		logAIGuardAudit(a, userID, req.ServerID, req.Command, riskLevel, "blocked")
		c.JSON(http.StatusForbidden, gin.H{"error": "command is blocked by policy"})
		return
	}
	if (riskLevel == "high" || riskLevel == "critical") && !req.RiskAck {
		logAIGuardAudit(a, userID, req.ServerID, req.Command, riskLevel, "confirmation_required")
		c.JSON(http.StatusForbidden, gin.H{"error": "explicit risk acknowledgement required"})
		return
	}

	raw, err := a.hub.RequestAgent(req.ServerID, "run_command", map[string]any{"command": req.Command}, 60*time.Second)
	if err != nil {
		logAIGuardAudit(a, userID, req.ServerID, req.Command, riskLevel, "failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		logAIGuardAudit(a, userID, req.ServerID, req.Command, riskLevel, "failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	logAIGuardAudit(a, userID, req.ServerID, req.Command, riskLevel, "executed")
	c.JSON(http.StatusOK, gin.H{
		"execution_id": time.Now().UTC().Format("20060102150405.000000000"),
		"status":       "finished",
		"trace": []string{
			"risk analysis passed",
			"command dispatched to server agent",
			"agent responded successfully",
		},
		"result":   decoded,
		"findings": findings,
	})
}

func (a *App) handleAICommandAudit(c *gin.Context) {
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
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
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
	var logs []models.CommandLog
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(limit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load command audit"})
		return
	}
	out := make([]gin.H, 0, len(logs))
	for _, item := range logs {
		level, blocked, _, _ := assessCommandRisk(item.Command)
		decision := "executed"
		if blocked {
			decision = "blocked"
		}
		out = append(out, gin.H{
			"id":         item.ID,
			"command":    item.Command,
			"risk_level": level,
			"decision":   decision,
			"created_at": item.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func assessCommandRisk(command string) (string, bool, []gin.H, []string) {
	cmd := strings.ToLower(strings.TrimSpace(command))
	findings := make([]gin.H, 0)
	policyMatches := make([]string, 0)
	risk := "low"
	blocked := false

	blockedTokens := []string{"rm -rf /", "mkfs", "dd if=", ":(){", "shutdown -h", "reboot"}
	for _, token := range blockedTokens {
		if strings.Contains(cmd, token) {
			blocked = true
			risk = "critical"
			policyMatches = append(policyMatches, "blocked-destructive")
			findings = append(findings, gin.H{
				"code":           "destructive_command",
				"title":          "Destructive operation detected",
				"details":        "Command matches destructive pattern: " + token,
				"affected_paths": []string{},
			})
			return risk, blocked, findings, policyMatches
		}
	}

	if strings.Contains(cmd, "sudo ") {
		risk = "medium"
		policyMatches = append(policyMatches, "privileged-command")
		findings = append(findings, gin.H{
			"code":           "privileged_command",
			"title":          "Privileged operation",
			"details":        "Command requests elevated privileges.",
			"affected_paths": []string{},
		})
	}
	if strings.Contains(cmd, "systemctl restart") || strings.Contains(cmd, "docker restart") {
		if risk == "low" {
			risk = "medium"
		}
		policyMatches = append(policyMatches, "service-impact")
		findings = append(findings, gin.H{
			"code":           "service_restart",
			"title":          "Service impact",
			"details":        "Command can impact service availability.",
			"affected_paths": []string{},
		})
	}
	if strings.Contains(cmd, "chmod 777") || strings.Contains(cmd, "iptables") || strings.Contains(cmd, "ufw ") {
		risk = "high"
		policyMatches = append(policyMatches, "security-sensitive")
		findings = append(findings, gin.H{
			"code":           "security_sensitive",
			"title":          "Security-sensitive command",
			"details":        "Command modifies security or network policy.",
			"affected_paths": []string{},
		})
	}
	return risk, blocked, findings, policyMatches
}

func logAIGuardAudit(a *App, userID, serverID uint, command, riskLevel, decision string) {
	logLine := truncateForCommandLog(command, maxCommandLogLength)
	_ = a.db.Create(&models.CommandLog{
		UserID:   userID,
		ServerID: serverID,
		Command:  "[" + riskLevel + "][" + decision + "] " + logLine,
	}).Error
}
