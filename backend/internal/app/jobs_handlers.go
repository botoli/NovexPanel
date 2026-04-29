package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type createJobRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Cron    string `json:"cron"`
}

func (a *App) handleCreateJob(c *gin.Context) {
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
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(strings.ToLower(req.Type))
	req.Command = strings.TrimSpace(req.Command)
	req.Cron = strings.TrimSpace(req.Cron)
	if req.Name == "" || len(req.Name) > 140 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if req.Type == "" {
		req.Type = "custom"
	}
	if req.Command == "" || len(req.Command) > 2048 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid command"})
		return
	}
	job := models.Job{
		UserID:   userID,
		ServerID: serverID,
		Name:     req.Name,
		Type:     req.Type,
		Command:  req.Command,
		CronExpr: req.Cron,
		Status:   "idle",
		Meta:     datatypes.JSON([]byte("{}")),
	}
	if err := a.db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create job"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": job.ID})
}

func (a *App) handleListServerJobs(c *gin.Context) {
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
	var jobs []models.Job
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list jobs"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func (a *App) handleGetJob(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	jobID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	var job models.Job
	if err := a.db.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (a *App) handleRunJobNow(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	jobID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	var job models.Job
	if err := a.db.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	run := models.JobRun{
		JobID:       job.ID,
		TriggeredBy: "manual",
		Status:      "running",
		StartedAt:   time.Now().UTC(),
	}
	if err := a.db.Create(&run).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to start job run"})
		return
	}
	_ = a.db.Model(&models.Job{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":          "running",
		"last_started_at": run.StartedAt,
		"last_error":      "",
	}).Error

	payload := map[string]any{
		"job_id":   job.ID,
		"run_id":   run.ID,
		"command":  job.Command,
		"job_type": job.Type,
	}
	if _, err := a.hub.RequestAgent(job.ServerID, "run_job", payload, 12*time.Second); err != nil {
		_ = a.failJobRun(run.ID, err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": normalizeAgentDispatchError(err)})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "run_id": run.ID, "status": "running"})
}

func (a *App) handleCancelJob(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	jobID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	var job models.Job
	if err := a.db.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	_, _ = a.hub.RequestAgent(job.ServerID, "cancel_job", map[string]any{"job_id": job.ID}, 10*time.Second)
	_ = a.db.Model(&models.Job{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":        "cancelled",
		"last_ended_at": time.Now().UTC(),
	}).Error
	c.JSON(http.StatusOK, gin.H{"job_id": job.ID, "status": "cancelled"})
}

func (a *App) handleListJobRuns(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	jobID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	var job models.Job
	if err := a.db.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	var runs []models.JobRun
	if err := a.db.Where("job_id = ?", jobID).Order("id desc").Limit(50).Find(&runs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list runs"})
		return
	}
	c.JSON(http.StatusOK, runs)
}

func (a *App) handleJobLogs(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	jobID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	var job models.Job
	if err := a.db.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	var logs []models.JobLog
	if err := a.db.Where("job_id = ?", jobID).Order("created_at asc").Limit(5000).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (a *App) failJobRun(runID uint, errText string) error {
	now := time.Now().UTC()
	return a.db.Transaction(func(tx *gorm.DB) error {
		var run models.JobRun
		if err := tx.Where("id = ?", runID).First(&run).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.JobRun{}).Where("id = ?", runID).Updates(map[string]any{
			"status":      "failed",
			"error":       errText,
			"finished_at": &now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Job{}).Where("id = ?", run.JobID).Updates(map[string]any{
			"status":        "failed",
			"last_error":    errText,
			"last_ended_at": &now,
		}).Error
	})
}

func (a *App) applyJobRunResult(jobID, runID uint, status string, exitCode *int, errText string, rawLogs []string) {
	if jobID == 0 || runID == 0 {
		return
	}
	now := time.Now().UTC()
	if status == "" {
		status = "failed"
	}
	updates := map[string]any{"status": status, "finished_at": &now}
	if exitCode != nil {
		updates["exit_code"] = *exitCode
	}
	if strings.TrimSpace(errText) != "" {
		updates["error"] = strings.TrimSpace(errText)
	}
	_ = a.db.Model(&models.JobRun{}).Where("id = ? AND job_id = ?", runID, jobID).Updates(updates).Error

	jobUpdates := map[string]any{
		"status":        status,
		"last_ended_at": &now,
		"last_error":    strings.TrimSpace(errText),
	}
	if exitCode != nil {
		jobUpdates["last_exit_code"] = *exitCode
	}
	_ = a.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(jobUpdates).Error

	for _, line := range rawLogs {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		logLine := models.JobLog{JobID: jobID, JobRunID: &runID, Line: trimmed, Stream: "stdout", CreatedAt: now}
		_ = a.db.Create(&logLine).Error
		a.hub.BroadcastJobLog(jobID, runID, trimmed, "stdout", now)
	}
	a.hub.BroadcastJobComplete(jobID, runID, status, errText, exitCode)
}

func parseIntFromJSONRaw(raw json.RawMessage) *int {
	if len(raw) == 0 {
		return nil
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return &v
}

func parseStringList(raw json.RawMessage) []string {
	var out []string
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}
