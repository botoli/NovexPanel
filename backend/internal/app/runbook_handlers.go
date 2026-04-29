package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"text/template"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type runbookUpsertRequest struct {
	Title       string          `json:"title"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	TargetType  string          `json:"target_type"`
	Definition  json.RawMessage `json:"definition"`
	ChangeNote  string          `json:"change_note"`
}

func (a *App) handleCreateRunbook(c *gin.Context) {
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
	var req runbookUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	slug, err := validateRunbookSlug(req.Slug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	targetType, err := validateRunbookTargetType(req.TargetType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	definition, err := validateRunbookDefinition(req.Definition)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rawDefinition, _ := json.Marshal(definition)
	runbook := models.Runbook{
		UserID:        userID,
		ServerID:      serverID,
		Title:         strings.TrimSpace(req.Title),
		Slug:          slug,
		Description:   strings.TrimSpace(req.Description),
		Tags:          normalizeTags(req.Tags),
		TargetType:    targetType,
		LatestVersion: 1,
	}
	if runbook.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if err := a.db.Create(&runbook).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "runbook slug already exists"})
		return
	}
	version := models.RunbookVersion{
		RunbookID:      runbook.ID,
		Version:        1,
		DefinitionJSON: datatypes.JSON(rawDefinition),
		CreatedBy:      userID,
		ChangeNote:     strings.TrimSpace(req.ChangeNote),
	}
	_ = a.db.Create(&version).Error
	a.logRunbookAudit(userID, serverID, &runbook.ID, nil, "create", map[string]any{"slug": runbook.Slug})
	c.JSON(http.StatusCreated, gin.H{"id": runbook.ID})
}

func (a *App) handleListRunbooks(c *gin.Context) {
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
	var runbooks []models.Runbook
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("updated_at desc").Find(&runbooks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list runbooks"})
		return
	}
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	tagFilter := strings.ToLower(strings.TrimSpace(c.Query("tag")))
	out := make([]gin.H, 0, len(runbooks))
	for _, rb := range runbooks {
		if search != "" && !strings.Contains(strings.ToLower(rb.Title+" "+rb.Description+" "+rb.Slug), search) {
			continue
		}
		tags := splitTags(rb.Tags)
		if tagFilter != "" {
			matched := false
			for _, t := range tags {
				if strings.EqualFold(t, tagFilter) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, gin.H{
			"id":              rb.ID,
			"title":           rb.Title,
			"slug":            rb.Slug,
			"description":     rb.Description,
			"tags":            tags,
			"target_type":     rb.TargetType,
			"latest_version":  rb.LatestVersion,
			"last_run_at":     rb.LastRunAt,
			"last_run_status": rb.LastRunStatus,
			"updated_at":      rb.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (a *App) loadRunbookForUser(userID, serverID, runbookID uint) (*models.Runbook, error) {
	var rb models.Runbook
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", runbookID, userID, serverID).First(&rb).Error; err != nil {
		return nil, err
	}
	return &rb, nil
}

func (a *App) handleGetRunbook(c *gin.Context) {
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
	runbookID, err := parseUintParam(c, "runbookId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runbook id"})
		return
	}
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var version models.RunbookVersion
	_ = a.db.Where("runbook_id = ? AND version = ?", rb.ID, rb.LatestVersion).First(&version).Error
	c.JSON(http.StatusOK, gin.H{
		"id":              rb.ID,
		"title":           rb.Title,
		"slug":            rb.Slug,
		"description":     rb.Description,
		"tags":            splitTags(rb.Tags),
		"target_type":     rb.TargetType,
		"latest_version":  rb.LatestVersion,
		"last_run_at":     rb.LastRunAt,
		"last_run_status": rb.LastRunStatus,
		"definition":      json.RawMessage(version.DefinitionJSON),
	})
}

func (a *App) handlePatchRunbook(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var req runbookUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]any{
		"title":       strings.TrimSpace(req.Title),
		"description": strings.TrimSpace(req.Description),
		"tags":        normalizeTags(req.Tags),
	}
	if strings.TrimSpace(req.Title) == "" {
		delete(updates, "title")
	}
	if strings.TrimSpace(req.TargetType) != "" {
		targetType, valErr := validateRunbookTargetType(req.TargetType)
		if valErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": valErr.Error()})
			return
		}
		updates["target_type"] = targetType
	}
	if err := a.db.Model(&models.Runbook{}).Where("id = ?", rb.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update runbook"})
		return
	}
	a.logRunbookAudit(userID, serverID, &rb.ID, nil, "update", updates)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *App) handleDeleteRunbook(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	_ = a.db.Where("runbook_id = ?", rb.ID).Delete(&models.RunbookVersion{}).Error
	_ = a.db.Where("runbook_id = ?", rb.ID).Delete(&models.RunbookExecution{}).Error
	_ = a.db.Delete(&models.Runbook{}, rb.ID).Error
	a.logRunbookAudit(userID, serverID, &rb.ID, nil, "delete", map[string]any{"slug": rb.Slug})
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (a *App) handleListRunbookVersions(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	if _, err := a.loadRunbookForUser(userID, serverID, runbookID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var versions []models.RunbookVersion
	_ = a.db.Where("runbook_id = ?", runbookID).Order("version desc").Find(&versions).Error
	c.JSON(http.StatusOK, versions)
}

func (a *App) handleCreateRunbookVersion(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var req runbookUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	definition, err := validateRunbookDefinition(req.Definition)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rawDefinition, _ := json.Marshal(definition)
	nextVersion := rb.LatestVersion + 1
	version := models.RunbookVersion{
		RunbookID:      rb.ID,
		Version:        nextVersion,
		DefinitionJSON: datatypes.JSON(rawDefinition),
		CreatedBy:      userID,
		ChangeNote:     strings.TrimSpace(req.ChangeNote),
	}
	if err := a.db.Create(&version).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create version"})
		return
	}
	_ = a.db.Model(&models.Runbook{}).Where("id = ?", rb.ID).Update("latest_version", nextVersion).Error
	a.logRunbookAudit(userID, serverID, &rb.ID, nil, "create_version", map[string]any{"version": nextVersion})
	c.JSON(http.StatusCreated, gin.H{"version": nextVersion})
}

func (a *App) handleRunbookRollbackVersion(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	var req struct {
		Version int `json:"version"`
	}
	_ = c.ShouldBindJSON(&req)
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	if req.Version <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}
	var version models.RunbookVersion
	if err := a.db.Where("runbook_id = ? AND version = ?", rb.ID, req.Version).First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	newVersionNum := rb.LatestVersion + 1
	newVersion := models.RunbookVersion{
		RunbookID:       rb.ID,
		Version:         newVersionNum,
		DefinitionJSON:  version.DefinitionJSON,
		CreatedBy:       userID,
		ChangeNote:      "rollback to version",
		IsRollbackPoint: true,
	}
	_ = a.db.Create(&newVersion).Error
	_ = a.db.Model(&models.Runbook{}).Where("id = ?", rb.ID).Update("latest_version", newVersionNum).Error
	a.logRunbookAudit(userID, serverID, &rb.ID, nil, "rollback_version", map[string]any{"from": req.Version, "to": newVersionNum})
	c.JSON(http.StatusOK, gin.H{"version": newVersionNum})
}

func renderRunbookDryRun(rawDefinition datatypes.JSON, vars map[string]string) (map[string]any, error) {
	var definition map[string]any
	if err := json.Unmarshal(rawDefinition, &definition); err != nil {
		return nil, err
	}
	stepsRaw, _ := definition["steps"].([]any)
	preview := make([]map[string]any, 0, len(stepsRaw))
	for idx, item := range stepsRaw {
		step, _ := item.(map[string]any)
		commandTpl := strings.TrimSpace("")
		if raw, ok := step["command"]; ok {
			commandTpl = strings.TrimSpace(fmt.Sprintf("%v", raw))
		}
		rendered := commandTpl
		if commandTpl != "" {
			tpl, err := template.New("cmd").Delims("{{", "}}").Parse(commandTpl)
			if err == nil {
				var buf bytes.Buffer
				if execErr := tpl.Execute(&buf, vars); execErr == nil {
					rendered = buf.String()
				}
			}
		}
		preview = append(preview, map[string]any{
			"step_index": idx,
			"name":       step["name"],
			"type":       step["type"],
			"command":    rendered,
		})
	}
	return map[string]any{"steps": preview, "variables": vars}, nil
}

func (a *App) handleRunbookDryRun(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var req struct {
		Version   int               `json:"version"`
		Variables map[string]string `json:"variables"`
	}
	_ = c.ShouldBindJSON(&req)
	versionNum := req.Version
	if versionNum <= 0 {
		versionNum = rb.LatestVersion
	}
	var version models.RunbookVersion
	if err := a.db.Where("runbook_id = ? AND version = ?", rb.ID, versionNum).First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	preview, err := renderRunbookDryRun(version.DefinitionJSON, req.Variables)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to render dry run"})
		return
	}
	a.logRunbookAudit(userID, serverID, &rb.ID, nil, "dry_run", map[string]any{"version": versionNum})
	c.JSON(http.StatusOK, gin.H{"version": versionNum, "preview": preview})
}

func (a *App) handleExecuteRunbook(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	rb, err := a.loadRunbookForUser(userID, serverID, runbookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var req struct {
		Version   int               `json:"version"`
		Variables map[string]string `json:"variables"`
		DryRun    bool              `json:"dry_run"`
	}
	_ = c.ShouldBindJSON(&req)
	versionNum := req.Version
	if versionNum <= 0 {
		versionNum = rb.LatestVersion
	}
	var version models.RunbookVersion
	if err := a.db.Where("runbook_id = ? AND version = ?", rb.ID, versionNum).First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}

	now := time.Now().UTC()
	execution := models.RunbookExecution{
		RunbookID:        rb.ID,
		RunbookVersionID: version.ID,
		DryRun:           req.DryRun,
		Status:           "pending",
		StartedAt:        &now,
		TriggeredBy:      userID,
		Summary:          "execution accepted",
	}
	if err := a.db.Create(&execution).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create execution"})
		return
	}

	payload := map[string]any{
		"runbook_id":    rb.ID,
		"execution_id":  execution.ID,
		"version":       versionNum,
		"dry_run":       req.DryRun,
		"variables":     req.Variables,
		"target_type":   rb.TargetType,
		"definition":    json.RawMessage(version.DefinitionJSON),
		"runbook_title": rb.Title,
	}
	if req.DryRun {
		_ = a.db.Model(&models.RunbookExecution{}).Where("id = ?", execution.ID).Updates(map[string]any{
			"status":      "success",
			"finished_at": time.Now().UTC(),
			"summary":     "dry run completed",
		}).Error
	} else {
		if _, err := a.hub.RequestAgent(serverID, "runbook_execute", payload, 12*time.Second); err != nil {
			_ = a.db.Model(&models.RunbookExecution{}).Where("id = ?", execution.ID).Updates(map[string]any{
				"status":      "failed",
				"finished_at": time.Now().UTC(),
				"summary":     normalizeAgentDispatchError(err),
			}).Error
			c.JSON(http.StatusBadGateway, gin.H{"error": normalizeAgentDispatchError(err)})
			return
		}
	}
	_ = a.db.Model(&models.Runbook{}).Where("id = ?", rb.ID).Updates(map[string]any{
		"last_run_at":     now,
		"last_run_status": "pending",
	}).Error
	a.logRunbookAudit(userID, serverID, &rb.ID, &execution.ID, "execute", map[string]any{"version": versionNum, "dry_run": req.DryRun})
	c.JSON(http.StatusAccepted, gin.H{"execution_id": execution.ID, "status": execution.Status})
}

func (a *App) handleRollbackRunbookExecution(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	var req struct {
		ExecutionID uint `json:"execution_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ExecutionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution_id is required"})
		return
	}
	var execution models.RunbookExecution
	if err := a.db.Where("id = ?", req.ExecutionID).First(&execution).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}
	var rb models.Runbook
	if err := a.db.Where("id = ? AND server_id = ? AND user_id = ?", execution.RunbookID, serverID, userID).First(&rb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	now := time.Now().UTC()
	newExec := models.RunbookExecution{
		RunbookID:                 rb.ID,
		RunbookVersionID:          execution.RunbookVersionID,
		Status:                    "pending",
		StartedAt:                 &now,
		TriggeredBy:               userID,
		RolledBackFromExecutionID: &execution.ID,
		Summary:                   "rollback requested",
	}
	_ = a.db.Create(&newExec).Error
	_, _ = a.hub.RequestAgent(serverID, "runbook_rollback", map[string]any{
		"runbook_id":       rb.ID,
		"execution_id":     newExec.ID,
		"rollback_from_id": execution.ID,
	}, 12*time.Second)
	a.logRunbookAudit(userID, serverID, &rb.ID, &newExec.ID, "rollback_execution", map[string]any{"from_execution": execution.ID})
	c.JSON(http.StatusAccepted, gin.H{"execution_id": newExec.ID})
}

func (a *App) handleListRunbookExecutions(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	if _, err := a.loadRunbookForUser(userID, serverID, runbookID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var executions []models.RunbookExecution
	_ = a.db.Where("runbook_id = ?", runbookID).Order("id desc").Limit(100).Find(&executions).Error
	c.JSON(http.StatusOK, executions)
}

func (a *App) handleRunbookExecutionLogs(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	executionID, err := parseUintParam(c, "executionId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution id"})
		return
	}
	var execution models.RunbookExecution
	if err := a.db.Where("id = ?", executionID).First(&execution).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}
	var rb models.Runbook
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", execution.RunbookID, userID, serverID).First(&rb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var logs []models.RunbookExecutionLog
	_ = a.db.Where("execution_id = ?", executionID).Order("id asc").Find(&logs).Error
	c.JSON(http.StatusOK, logs)
}

func (a *App) handleRunbookAudit(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	if _, err := a.loadRunbookForUser(userID, serverID, runbookID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var events []models.RunbookAuditEvent
	_ = a.db.Where("runbook_id = ?", runbookID).Order("id desc").Limit(200).Find(&events).Error
	c.JSON(http.StatusOK, events)
}

func (a *App) handleRunbookDiff(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	runbookID, _ := parseUintParam(c, "runbookId")
	if _, err := a.loadRunbookForUser(userID, serverID, runbookID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	from := strings.TrimSpace(c.Query("from"))
	to := strings.TrimSpace(c.Query("to"))
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required"})
		return
	}
	var fromVer, toVer models.RunbookVersion
	if err := a.db.Where("runbook_id = ? AND version = ?", runbookID, from).First(&fromVer).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "from version not found"})
		return
	}
	if err := a.db.Where("runbook_id = ? AND version = ?", runbookID, to).First(&toVer).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "to version not found"})
		return
	}
	diff := computeRunbookDefinitionDiff(fromVer.DefinitionJSON, toVer.DefinitionJSON)
	c.JSON(http.StatusOK, gin.H{"from": from, "to": to, "diff": diff})
}

func computeRunbookDefinitionDiff(fromRaw, toRaw datatypes.JSON) []map[string]any {
	var fromObj map[string]any
	var toObj map[string]any
	_ = json.Unmarshal(fromRaw, &fromObj)
	_ = json.Unmarshal(toRaw, &toObj)
	keysMap := map[string]struct{}{}
	for k := range fromObj {
		keysMap[k] = struct{}{}
	}
	for k := range toObj {
		keysMap[k] = struct{}{}
	}
	keys := make([]string, 0, len(keysMap))
	for k := range keysMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		fromVal, fromOK := fromObj[key]
		toVal, toOK := toObj[key]
		if fromOK && toOK {
			fb, _ := json.Marshal(fromVal)
			tb, _ := json.Marshal(toVal)
			if string(fb) == string(tb) {
				continue
			}
			out = append(out, map[string]any{"field": key, "type": "changed", "from": fromVal, "to": toVal})
			continue
		}
		if fromOK {
			out = append(out, map[string]any{"field": key, "type": "removed", "from": fromVal})
			continue
		}
		if toOK {
			out = append(out, map[string]any{"field": key, "type": "added", "to": toVal})
		}
	}
	return out
}

func (a *App) logRunbookAudit(userID, serverID uint, runbookID, executionID *uint, action string, payload any) {
	raw, _ := json.Marshal(payload)
	event := models.RunbookAuditEvent{
		UserID:      userID,
		ServerID:    serverID,
		RunbookID:   runbookID,
		ExecutionID: executionID,
		Action:      action,
		PayloadJSON: datatypes.JSON(raw),
	}
	_ = a.db.Create(&event).Error
}

func (a *App) appendRunbookExecutionLog(executionID uint, stepIndex int, stepName, status, line, stream string, attempt int) {
	entry := models.RunbookExecutionLog{
		ExecutionID: executionID,
		StepIndex:   stepIndex,
		StepName:    strings.TrimSpace(stepName),
		Status:      strings.TrimSpace(strings.ToLower(status)),
		Line:        strings.TrimSpace(line),
		Stream:      strings.TrimSpace(strings.ToLower(stream)),
		Attempt:     attempt,
		CreatedAt:   time.Now().UTC(),
	}
	if entry.Stream == "" {
		entry.Stream = "stdout"
	}
	if entry.Status == "" {
		entry.Status = "running"
	}
	_ = a.db.Create(&entry).Error
}

func (a *App) applyRunbookResult(executionID uint, status, summary string) {
	if executionID == 0 {
		return
	}
	finalStatus := strings.TrimSpace(strings.ToLower(status))
	switch finalStatus {
	case "success", "failed", "rolled_back":
	default:
		finalStatus = "failed"
	}
	now := time.Now().UTC()
	var execution models.RunbookExecution
	if err := a.db.Where("id = ?", executionID).First(&execution).Error; err != nil {
		return
	}
	_ = a.db.Model(&models.RunbookExecution{}).Where("id = ?", executionID).Updates(map[string]any{
		"status":      finalStatus,
		"finished_at": &now,
		"summary":     summary,
	}).Error
	_ = a.db.Model(&models.Runbook{}).Where("id = ?", execution.RunbookID).Updates(map[string]any{
		"last_run_at":     &now,
		"last_run_status": finalStatus,
	}).Error
}

func (a *App) ensureRunbookStartersForServer(userID, serverID uint) {
	type runbookStarterSeeder interface {
		SeedStarterRunbooksForServer(db *gorm.DB, userID, serverID uint) error
	}
	// storage package helper is invoked via local function below in migrations;
	// runtime auto-seed for newly created servers is intentionally skipped here.
	_ = userID
	_ = serverID
	_ = runbookStarterSeeder(nil)
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
