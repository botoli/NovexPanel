package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type runCommandRequest struct {
	Command string `json:"command"`
}

type deployRequest struct {
	Source      string `json:"source"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	ProjectType string `json:"project_type"`
	ZipData     string `json:"zip_data"`
}

type metricsHistoryPoint struct {
	Timestamp time.Time
	CPU       float64
	RAM       float64
	Disk      float64
}

type aggregatedMetricsRow struct {
	BucketUnix int64   `gorm:"column:bucket_unix"`
	CPU        float64 `gorm:"column:cpu"`
	RAM        float64 `gorm:"column:ram"`
	Disk       float64 `gorm:"column:disk"`
}

func (a *App) handleListServers(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var servers []models.Server
	if err := a.db.Where("user_id = ?", userID).Order("id desc").Find(&servers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list servers"})
		return
	}

	response := make([]gin.H, 0, len(servers))
	for _, server := range servers {
		item := gin.H{
			"id":           server.ID,
			"name":         server.Name,
			"ip":           server.IP,
			"online":       server.Online,
			"last_metrics": server.LastMetrics,
		}
		response = append(response, item)
	}

	c.JSON(http.StatusOK, response)
}

func (a *App) handleServerMetricsHistory(c *gin.Context) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	interval, err := parseMetricsInterval(c.Query("interval"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval, allowed values: 1s, 1m, 5m, 1h"})
		return
	}

	since := time.Now().Add(-time.Duration(a.cfg.MetricsHistoryDays) * 24 * time.Hour)
	points, err := a.loadMetricsHistory(serverID, since, interval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load metrics"})
		return
	}

	response := make([]gin.H, 0, len(points))
	for _, point := range points {
		response = append(response, gin.H{
			"timestamp": point.Timestamp,
			"cpu":       point.CPU,
			"ram":       point.RAM,
			"disk":      point.Disk,
		})
	}

	c.JSON(http.StatusOK, response)
}

func parseMetricsInterval(raw string) (string, error) {
	interval := strings.TrimSpace(strings.ToLower(raw))
	if interval == "" {
		return "1s", nil
	}

	switch interval {
	case "1s", "1m", "5m", "1h":
		return interval, nil
	default:
		return "", fmt.Errorf("unsupported interval %q", interval)
	}
}

func (a *App) loadMetricsHistory(serverID uint, since time.Time, interval string) ([]metricsHistoryPoint, error) {
	if interval == "1s" {
		var points []models.MetricPoint
		if err := a.db.Where("server_id = ? AND timestamp >= ?", serverID, since).Order("timestamp asc").Find(&points).Error; err != nil {
			return nil, err
		}

		response := make([]metricsHistoryPoint, 0, len(points))
		for _, point := range points {
			response = append(response, metricsHistoryPoint{
				Timestamp: point.Timestamp,
				CPU:       point.CPUUsage,
				RAM:       point.RAMPercent,
				Disk:      point.DiskPercent,
			})
		}
		return response, nil
	}

	bucketExpr, err := metricBucketExpression(a.db.Dialector.Name(), interval)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
SELECT %s AS bucket_unix,
       AVG(cpu_usage) AS cpu,
       AVG(ram_percent) AS ram,
       AVG(disk_percent) AS disk
FROM metric_points
WHERE server_id = ? AND timestamp >= ?
GROUP BY 1
ORDER BY 1 ASC`, bucketExpr)

	rows := make([]aggregatedMetricsRow, 0)
	if err := a.db.Raw(query, serverID, since).Scan(&rows).Error; err != nil {
		return nil, err
	}

	response := make([]metricsHistoryPoint, 0, len(rows))
	for _, row := range rows {
		response = append(response, metricsHistoryPoint{
			Timestamp: time.Unix(row.BucketUnix, 0).UTC(),
			CPU:       row.CPU,
			RAM:       row.RAM,
			Disk:      row.Disk,
		})
	}

	return response, nil
}

func metricBucketExpression(dialect, interval string) (string, error) {
	switch dialect {
	case "postgres":
		switch interval {
		case "1m":
			return "CAST(FLOOR(EXTRACT(EPOCH FROM DATE_TRUNC('minute', timestamp))) AS BIGINT)", nil
		case "5m":
			return "CAST(FLOOR(EXTRACT(EPOCH FROM timestamp) / 300) * 300 AS BIGINT)", nil
		case "1h":
			return "CAST(FLOOR(EXTRACT(EPOCH FROM DATE_TRUNC('hour', timestamp))) AS BIGINT)", nil
		}
	case "sqlite":
		switch interval {
		case "1m":
			return "CAST((CAST(strftime('%s', timestamp) AS INTEGER) / 60) * 60 AS INTEGER)", nil
		case "5m":
			return "CAST((CAST(strftime('%s', timestamp) AS INTEGER) / 300) * 300 AS INTEGER)", nil
		case "1h":
			return "CAST((CAST(strftime('%s', timestamp) AS INTEGER) / 3600) * 3600 AS INTEGER)", nil
		}
	}

	return "", fmt.Errorf("unsupported interval %q for db dialect %q", interval, dialect)
}

func (a *App) handleServerProcesses(c *gin.Context) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	raw, err := a.hub.RequestAgent(serverID, "get_processes", nil, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServerCommand(c *gin.Context) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	var req runCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Command = strings.TrimSpace(req.Command)
	if req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}

	raw, err := a.hub.RequestAgent(serverID, "run_command", map[string]any{"command": req.Command}, 60*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleKillServerProcess(c *gin.Context) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	pidRaw := c.Param("pid")
	var pid uint64
	if parsed, parseErr := parsePositiveInt(pidRaw); parseErr == nil {
		pid = parsed
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pid"})
		return
	}

	raw, err := a.hub.RequestAgent(serverID, "kill_process", map[string]any{"pid": pid}, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleServerDeploy(c *gin.Context) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	var req deployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Source = strings.TrimSpace(strings.ToLower(req.Source))
	req.ProjectType = strings.TrimSpace(strings.ToLower(req.ProjectType))
	if req.ProjectType == "" {
		req.ProjectType = "auto"
	}

	if req.Source != "github" && req.Source != "zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source must be github or zip"})
		return
	}
	if req.Source == "github" && strings.TrimSpace(req.RepoURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_url is required for github source"})
		return
	}
	if req.Source == "zip" && strings.TrimSpace(req.ZipData) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zip_data is required for zip source"})
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}

	deploy := models.Deploy{
		UserID:      userID,
		ServerID:    serverID,
		Source:      req.Source,
		Status:      "running",
		ProjectType: req.ProjectType,
		RepoURL:     req.RepoURL,
		StartedAt:   time.Now(),
	}
	if err := a.db.Create(&deploy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create deploy"})
		return
	}

	payload := map[string]any{
		"deploy_id":    deploy.ID,
		"source":       req.Source,
		"repo_url":     req.RepoURL,
		"branch":       req.Branch,
		"project_type": req.ProjectType,
		"zip_data":     req.ZipData,
	}

	if _, err := a.hub.RequestAgent(serverID, "deploy", payload, 20*time.Second); err != nil {
		now := time.Now()
		a.db.Model(&deploy).Updates(map[string]any{
			"status":        "failed",
			"error_message": err.Error(),
			"finished_at":   &now,
		})
		c.JSON(http.StatusBadGateway, gin.H{
			"error":     err.Error(),
			"deploy_id": deploy.ID,
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"deploy_id": deploy.ID,
		"status":    "running",
	})
}

func (a *App) handleDeployLogs(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deployID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deploy id"})
		return
	}

	var deploy models.Deploy
	if err := a.db.Where("id = ? AND user_id = ?", deployID, userID).First(&deploy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deploy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load deploy"})
		return
	}

	var logs []models.DeployLog
	if err := a.db.Where("deploy_id = ?", deployID).Order("id asc").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load deploy logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deploy_id": deploy.ID,
		"status":    deploy.Status,
		"url":       deploy.ResultURL,
		"logs":      logs,
	})
}

func decodeRawJSON(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return gin.H{}, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parsePositiveInt(raw string) (uint64, error) {
	return parseUintFromString(raw)
}
