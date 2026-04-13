package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"novexpanel/backend/internal/auth"
	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (a *App) handleAgentWS(c *gin.Context) {
	tokenRaw := strings.TrimSpace(c.Query("token"))
	if tokenRaw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	tokenHash := auth.HashAgentToken(tokenRaw)
	var token models.AgentToken
	if err := a.db.Where("token_hash = ? AND revoked = ?", tokenHash, false).First(&token).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	now := time.Now()
	if token.ExpiresAt != nil && token.ExpiresAt.Before(now) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ip := c.ClientIP()
	if ip == "" {
		ip = parseRemoteIP(c.Request.RemoteAddr)
	}

	var server models.Server
	err = a.db.Where("token_id = ?", token.ID).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			server = models.Server{
				UserID:      token.UserID,
				TokenID:     token.ID,
				Name:        "Server " + strconv.FormatUint(uint64(token.ID), 10),
				IP:          ip,
				Online:      true,
				ConnectedAt: &now,
			}
			if err := a.db.Create(&server).Error; err != nil {
				return
			}
		} else {
			return
		}
	} else {
		if err := a.db.Model(&server).Updates(map[string]any{
			"ip":              ip,
			"online":          true,
			"connected_at":    &now,
			"disconnected_at": nil,
		}).Error; err != nil {
			return
		}
	}

	_ = a.db.Model(&token).Update("last_used_at", &now).Error

	client := a.hub.RegisterAgent(server.ID, token.UserID, conn)
	defer func() {
		a.hub.UnregisterAgent(server.ID, client)
		disconnectedAt := time.Now()
		_ = a.db.Model(&models.Server{}).Where("id = ?", server.ID).Updates(map[string]any{
			"online":          false,
			"disconnected_at": &disconnectedAt,
		}).Error
	}()

	_ = conn.SetReadDeadline(time.Time{})

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		a.handleAgentMessage(client, server.ID, payload)
	}
}

func (a *App) handleAgentMessage(client *AgentClient, serverID uint, payload []byte) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return
	}

	switch envelope.Type {
	case "command_response", "response":
		var msg struct {
			RequestID string          `json:"request_id"`
			Success   bool            `json:"success"`
			Error     string          `json:"error"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if msg.RequestID == "" {
			return
		}
		client.completeRequest(msg.RequestID, commandResult{
			Success: msg.Success,
			Data:    msg.Data,
			Err:     msg.Error,
		})
	case "metrics":
		var msg struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		a.persistMetrics(serverID, msg.Data)
		a.hub.BroadcastMetrics(serverID, msg.Data)
	case "terminal_output":
		var msg struct {
			SessionID string `json:"session_id"`
			Data      string `json:"data"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if msg.SessionID == "" {
			return
		}
		a.hub.ForwardTerminalOutput(msg.SessionID, serverID, msg.Data)
	case "deploy_log":
		var msg struct {
			DeployID uint   `json:"deploy_id"`
			Line     string `json:"line"`
			IsError  bool   `json:"is_error"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if msg.DeployID == 0 || msg.Line == "" {
			return
		}
		_ = a.db.Create(&models.DeployLog{DeployID: msg.DeployID, Line: msg.Line, IsError: msg.IsError}).Error
		a.hub.BroadcastDeployLog(msg.DeployID, msg.Line, msg.IsError)
	case "deploy_complete":
		var msg struct {
			DeployID uint   `json:"deploy_id"`
			Success  bool   `json:"success"`
			URL      string `json:"url"`
			Error    string `json:"error"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if msg.DeployID == 0 {
			return
		}

		status := "success"
		if !msg.Success {
			status = "failed"
		}
		now := time.Now()
		_ = a.db.Model(&models.Deploy{}).Where("id = ?", msg.DeployID).Updates(map[string]any{
			"status":        status,
			"result_url":    msg.URL,
			"error_message": msg.Error,
			"finished_at":   &now,
		}).Error
		a.hub.BroadcastDeployComplete(msg.DeployID, msg.Success, msg.URL, msg.Error)
	}
}

func (a *App) persistMetrics(serverID uint, raw json.RawMessage) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	copied := append([]byte(nil), raw...)

	var m struct {
		CPU struct {
			Usage float64 `json:"usage"`
		} `json:"cpu"`
		RAM struct {
			Percent float64 `json:"percent"`
		} `json:"ram"`
		Disk struct {
			Percent float64 `json:"percent"`
		} `json:"disk"`
	}
	_ = json.Unmarshal(copied, &m)

	now := time.Now()
	lastMetrics := datatypes.JSON(copied)
	_ = a.db.Model(&models.Server{}).Where("id = ?", serverID).Update("last_metrics", lastMetrics).Error
	_ = a.db.Create(&models.MetricPoint{
		ServerID:    serverID,
		Timestamp:   now,
		CPUUsage:    m.CPU.Usage,
		RAMPercent:  m.RAM.Percent,
		DiskPercent: m.Disk.Percent,
		Raw:         datatypes.JSON(copied),
	}).Error
}

func (a *App) handleSiteWS(c *gin.Context) {
	tokenRaw := strings.TrimSpace(c.Query("token"))
	if tokenRaw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	claims, err := auth.ParseUserToken(a.cfg.JWTSecret, tokenRaw)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	site := a.hub.NewSiteClient(claims.UserID, conn)
	defer a.hub.RemoveSite(site)

	_ = site.sendJSON(map[string]any{"type": "connected"})

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		a.handleSiteMessage(site, payload)
	}
}

func (a *App) handleSiteMessage(site *SiteClient, payload []byte) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		a.sendSiteError(site, "invalid json")
		return
	}

	switch envelope.Type {
	case "subscribe_metrics":
		var msg struct {
			ServerID uint `json:"server_id"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil || msg.ServerID == 0 {
			a.sendSiteError(site, "invalid subscribe_metrics payload")
			return
		}
		if _, err := a.requireServerForUser(site.userID, msg.ServerID); err != nil {
			a.sendSiteError(site, "server not found")
			return
		}
		a.hub.SubscribeMetrics(site, msg.ServerID)
		_ = site.sendJSON(map[string]any{"type": "subscribed_metrics", "server_id": msg.ServerID})
	case "unsubscribe_metrics":
		var msg struct {
			ServerID uint `json:"server_id"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil || msg.ServerID == 0 {
			a.sendSiteError(site, "invalid unsubscribe_metrics payload")
			return
		}
		a.hub.UnsubscribeMetrics(site, msg.ServerID)
		_ = site.sendJSON(map[string]any{"type": "unsubscribed_metrics", "server_id": msg.ServerID})
	case "open_terminal":
		var msg struct {
			ServerID uint `json:"server_id"`
			Rows     int  `json:"rows"`
			Cols     int  `json:"cols"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil || msg.ServerID == 0 {
			a.sendSiteError(site, "invalid open_terminal payload")
			return
		}
		if msg.Rows <= 0 {
			msg.Rows = 24
		}
		if msg.Cols <= 0 {
			msg.Cols = 80
		}
		if _, err := a.requireServerForUser(site.userID, msg.ServerID); err != nil {
			a.sendSiteError(site, "server not found")
			return
		}
		sessionID, err := a.hub.OpenTerminal(site, msg.ServerID, msg.Rows, msg.Cols)
		if err != nil {
			a.sendSiteError(site, err.Error())
			return
		}
		_ = site.sendJSON(map[string]any{
			"type":       "terminal_opened",
			"server_id":  msg.ServerID,
			"session_id": sessionID,
		})
	case "terminal_input":
		var msg struct {
			ServerID  uint   `json:"server_id"`
			SessionID string `json:"session_id"`
			Data      string `json:"data"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			a.sendSiteError(site, "invalid terminal_input payload")
			return
		}
		if err := a.hub.TerminalInput(site, msg.ServerID, msg.SessionID, msg.Data); err != nil {
			a.sendSiteError(site, err.Error())
		}
	case "close_terminal":
		var msg struct {
			ServerID uint `json:"server_id"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil || msg.ServerID == 0 {
			a.sendSiteError(site, "invalid close_terminal payload")
			return
		}
		a.hub.CloseTerminal(site, msg.ServerID)
		_ = site.sendJSON(map[string]any{"type": "terminal_closed", "server_id": msg.ServerID})
	case "deploy_logs":
		var msg struct {
			DeployID uint `json:"deploy_id"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil || msg.DeployID == 0 {
			a.sendSiteError(site, "invalid deploy_logs payload")
			return
		}
		var deploy models.Deploy
		if err := a.db.Where("id = ? AND user_id = ?", msg.DeployID, site.userID).First(&deploy).Error; err != nil {
			a.sendSiteError(site, "deploy not found")
			return
		}
		a.hub.SubscribeDeploy(site, msg.DeployID)
		_ = site.sendJSON(map[string]any{"type": "subscribed_deploy_logs", "deploy_id": msg.DeployID})
	case "ping":
		_ = site.sendJSON(map[string]any{"type": "pong"})
	default:
		a.sendSiteError(site, "unknown message type")
	}
}

func (a *App) sendSiteError(site *SiteClient, message string) {
	_ = site.sendJSON(map[string]any{
		"type":  "error",
		"error": message,
	})
}
