package app

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"novexpanel/backend/internal/auth"
	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (a *App) handleAgentWS(c *gin.Context) {
	tokenRaw := strings.TrimSpace(c.Query("token"))
	if tokenRaw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	log.Printf("agent ws connect attempt: remote=%s name=%q", c.Request.RemoteAddr, strings.TrimSpace(c.Query("name")))

	tokenHash := auth.HashAgentToken(tokenRaw)
	tokenHashPrefix := tokenHash
	if len(tokenHashPrefix) > 12 {
		tokenHashPrefix = tokenHashPrefix[:12]
	}
	var token models.AgentToken
	if err := a.db.Where("token_hash = ? AND revoked = ?", tokenHash, false).First(&token).Error; err != nil {
		log.Printf("agent ws token auth failed: remote=%s token_hash_prefix=%s error=%v", c.Request.RemoteAddr, tokenHashPrefix, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	serverName := strings.TrimSpace(c.Query("name"))
	log.Printf("agent ws token auth ok: token_id=%d user_id=%d token_hash_prefix=%s name=%q", token.ID, token.UserID, tokenHashPrefix, serverName)

	now := time.Now()
	if token.ExpiresAt != nil && token.ExpiresAt.Before(now) {
		log.Printf("agent ws token expired: token_id=%d", token.ID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	upgrader := a.newWSUpgrader(false)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("agent ws upgrade failed: remote=%s error=%v", c.Request.RemoteAddr, err)
		return
	}
	defer conn.Close()
	configureWSReadSettings(conn, a.wsReadLimit(512*1024))

	ip := c.ClientIP()
	if ip == "" {
		ip = parseRemoteIP(c.Request.RemoteAddr)
	}

	var server models.Server
	err = a.db.Where("token_id = ?", token.ID).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if serverName == "" {
				serverName = "Server " + strconv.FormatUint(uint64(token.ID), 10)
			}
			server = models.Server{
				UserID:      token.UserID,
				TokenID:     token.ID,
				Name:        serverName,
				IP:          ip,
				Online:      true,
				ConnectedAt: &now,
			}
			if err := a.db.Create(&server).Error; err != nil {
				log.Printf("agent ws create server failed: token_id=%d error=%v", token.ID, err)
				return
			}
			log.Printf("agent ws created server: server_id=%d token_id=%d user_id=%d", server.ID, server.TokenID, server.UserID)
		} else {
			log.Printf("agent ws load server failed: token_id=%d error=%v", token.ID, err)
			return
		}
	} else {
		updates := map[string]any{
			"ip":              ip,
			"online":          true,
			"connected_at":    &now,
			"disconnected_at": nil,
		}
		if serverName != "" {
			updates["name"] = serverName
		}
		if err := a.db.Model(&server).Updates(updates).Error; err != nil {
			log.Printf("agent ws update server failed: server_id=%d error=%v", server.ID, err)
			return
		}
		log.Printf("agent ws using existing server: server_id=%d token_id=%d user_id=%d", server.ID, server.TokenID, server.UserID)
	}

	_ = a.db.Model(&token).Update("last_used_at", &now).Error

	client := a.hub.RegisterAgent(server.ID, token.UserID, conn)
	log.Printf("agent ws registered in hub: server_id=%d user_id=%d active_server_ids=%v", server.ID, token.UserID, a.hub.ActiveAgentServerIDs())
	stopKeepalive := make(chan struct{})
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopKeepalive:
				return
			case <-ticker.C:
				if err := client.sendPing(); err != nil {
					log.Printf("agent ws ping failed: server_id=%d error=%v", server.ID, err)
					_ = conn.Close()
					return
				}
			}
		}
	}()
	defer func() {
		close(stopKeepalive)
		log.Printf("agent ws disconnecting: server_id=%d user_id=%d", server.ID, token.UserID)
		a.hub.UnregisterAgent(server.ID, client)
		disconnectedAt := time.Now()
		_ = a.db.Model(&models.Server{}).Where("id = ?", server.ID).Updates(map[string]any{
			"online":          false,
			"disconnected_at": &disconnectedAt,
		}).Error
		log.Printf("agent ws disconnected: server_id=%d", server.ID)
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			log.Printf("agent ws read loop stopped: server_id=%d error=%v", server.ID, err)
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
			DeployID      uint   `json:"deploy_id"`
			DeployIDCamel uint   `json:"deployId"`
			Line          string `json:"line"`
			IsError       bool   `json:"is_error"`
			IsErrorCamel  bool   `json:"isError"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if msg.DeployID == 0 {
			msg.DeployID = msg.DeployIDCamel
		}
		if msg.DeployID == 0 || msg.Line == "" {
			return
		}
		isError := msg.IsError || msg.IsErrorCamel
		_ = a.db.Create(&models.DeployLog{DeployID: msg.DeployID, Line: msg.Line, IsError: isError}).Error

		var deploy models.Deploy
		if err := a.db.Where("id = ?", msg.DeployID).First(&deploy).Error; err == nil {
			updates := map[string]any{
				"status":     "building",
				"deploy_log": appendDeployLogLine(deploy.DeployLog, msg.Line),
			}
			if updateErr := a.db.Model(&models.Deploy{}).Where("id = ?", msg.DeployID).Updates(updates).Error; updateErr != nil {
				log.Printf("update deploy log failed (deploy_id=%d): %v", msg.DeployID, updateErr)
			}
		}

		a.hub.BroadcastDeployLog(msg.DeployID, msg.Line, isError)
	case "deploy_result":
		var msg struct {
			DeployID      uint   `json:"deployId"`
			DeployIDSnake uint   `json:"deploy_id"`
			Status        string `json:"status"`
			URL           string `json:"url"`
			Port          int    `json:"port"`
			Log           string `json:"log"`
			Error         string `json:"error"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if msg.DeployID == 0 {
			msg.DeployID = msg.DeployIDSnake
		}
		a.applyDeployResult(msg.DeployID, msg.Status, msg.URL, msg.Port, msg.Log, msg.Error)
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
		status := "error"
		if msg.Success {
			status = "success"
		}
		a.applyDeployResult(msg.DeployID, status, msg.URL, 0, "", msg.Error)
	}
}

func (a *App) applyDeployResult(deployID uint, agentStatus, url string, port int, deployLog, errText string) {
	if deployID == 0 {
		return
	}

	status := "failed"
	if strings.EqualFold(strings.TrimSpace(agentStatus), "success") {
		status = "running"
	}

	var deploy models.Deploy
	if err := a.db.Where("id = ?", deployID).First(&deploy).Error; err != nil {
		log.Printf("deploy result ignored, deploy not found (deploy_id=%d): %v", deployID, err)
		return
	}

	currentLog := deploy.DeployLog
	if strings.TrimSpace(deployLog) != "" {
		currentLog = deployLog
		if !strings.HasSuffix(currentLog, "\n") {
			currentLog += "\n"
		}
	}
	if strings.TrimSpace(errText) != "" {
		currentLog = appendDeployLogLine(currentLog, errText)
	}

	now := time.Now()
	updates := map[string]any{
		"status":        status,
		"url":           url,
		"result_url":    url,
		"error_message": errText,
		"deploy_log":    currentLog,
		"finished_at":   &now,
	}
	if port > 0 {
		updates["port"] = port
	}

	if err := a.db.Model(&models.Deploy{}).Where("id = ?", deployID).Updates(updates).Error; err != nil {
		log.Printf("update deploy result failed (deploy_id=%d): %v", deployID, err)
		return
	}

	success := status == "running"
	a.hub.BroadcastDeployComplete(deployID, success, url, errText)
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
			Percent    float64 `json:"percent"`
			ReadSpeed  float64 `json:"read_speed"`
			WriteSpeed float64 `json:"write_speed"`
		} `json:"disk"`
		Network struct {
			RXSpeed float64 `json:"rx_speed"`
			TXSpeed float64 `json:"tx_speed"`
		} `json:"network"`
	}
	_ = json.Unmarshal(copied, &m)

	now := time.Now()
	lastMetrics := datatypes.JSON(copied)
	_ = a.db.Model(&models.Server{}).Where("id = ?", serverID).Update("last_metrics", lastMetrics).Error
	_ = a.db.Create(&models.MetricPoint{
		ServerID:       serverID,
		Timestamp:      now,
		CPUUsage:       m.CPU.Usage,
		RAMPercent:     m.RAM.Percent,
		DiskPercent:    m.Disk.Percent,
		DiskReadBytes:  m.Disk.ReadSpeed,
		DiskWriteBytes: m.Disk.WriteSpeed,
		NetworkRXBytes: m.Network.RXSpeed,
		NetworkTXBytes: m.Network.TXSpeed,
		Raw:            datatypes.JSON(copied),
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

	upgrader := a.newWSUpgrader(true)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	configureWSReadSettings(conn, a.wsReadLimit(128*1024))

	site := a.hub.NewSiteClient(claims.UserID, conn)
	stopKeepalive := make(chan struct{})
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopKeepalive:
				return
			case <-ticker.C:
				if err := site.sendPing(); err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	defer a.hub.RemoveSite(site)
	defer close(stopKeepalive)

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
	case "terminal_resize":
		var msg struct {
			ServerID  uint   `json:"server_id"`
			SessionID string `json:"session_id"`
			Rows      int    `json:"rows"`
			Cols      int    `json:"cols"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			a.sendSiteError(site, "invalid terminal_resize payload")
			return
		}
		if msg.Rows <= 0 || msg.Cols <= 0 {
			a.sendSiteError(site, "invalid terminal_resize dimensions")
			return
		}
		if msg.Rows > 1000 || msg.Cols > 1000 {
			a.sendSiteError(site, "terminal_resize dimensions are too large")
			return
		}
		if err := a.hub.TerminalResize(site, msg.ServerID, msg.SessionID, msg.Rows, msg.Cols); err != nil {
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
