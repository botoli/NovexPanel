package app

import (
	"errors"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateDeployRequest struct {
	ServerID           uint    `json:"serverId"`
	ServerIDLegacy     uint    `json:"server_id"`
	RepoURL            string  `json:"repoUrl"`
	RepoURLLegacy      string  `json:"repo_url"`
	Branch             string  `json:"branch"`
	Type               string  `json:"type"`
	TypeLegacy         string  `json:"project_type"`
	TypeCamel          string  `json:"projectType"`
	Subdirectory       string  `json:"subdirectory"`
	SubdirectoryLegacy string  `json:"sub_directory"`
	SubdirectoryCamel  string  `json:"subDirectory"`
	BuildCommand       *string `json:"buildCommand"`
	BuildCommandLegacy *string `json:"build_command"`
	OutputDir          *string `json:"outputDir"`
	OutputDirLegacy    *string `json:"output_dir"`
}

func (a *App) handleCreateDeploy(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	serverID := req.ServerID
	if serverID == 0 {
		serverID = req.ServerIDLegacy
	}
	if serverID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serverId is required"})
		return
	}

	repoURL := strings.TrimSpace(req.RepoURL)
	if repoURL == "" {
		repoURL = strings.TrimSpace(req.RepoURLLegacy)
	}
	if repoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repoUrl is required"})
		return
	}

	branch := normalizeDeployBranch(req.Branch)
	projectType := normalizeProjectType(req.Type, req.TypeLegacy, req.TypeCamel)
	subdirectory, err := normalizeAndValidateSubdirectory(req.Subdirectory, req.SubdirectoryLegacy, req.SubdirectoryCamel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	buildCommand := normalizeOptionalString(req.BuildCommand, req.BuildCommandLegacy)
	outputDir := normalizeOptionalString(req.OutputDir, req.OutputDirLegacy)

	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	deploy := models.Deploy{
		UserID:       userID,
		ServerID:     serverID,
		RepoURL:      repoURL,
		Branch:       branch,
		ProjectType:  projectType,
		Subdirectory: subdirectory,
		BuildCommand: buildCommand,
		OutputDir:    outputDir,
		Source:       "github",
		Status:       "pending",
		StartedAt:    time.Now(),
	}

	if err := a.db.Create(&deploy).Error; err != nil {
		log.Printf("create deploy failed (server_id=%d): %v", serverID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create deploy"})
		return
	}

	log.Printf("deploy created (deploy_id=%d server_id=%d user_id=%d)", deploy.ID, deploy.ServerID, userID)

	if err := a.sendDeployCommandToAgent(serverID, deploy.ID, repoURL, branch, projectType, subdirectory, buildCommand, outputDir); err != nil {
		var failedDeploy models.Deploy
		if loadErr := a.db.First(&failedDeploy, deploy.ID).Error; loadErr == nil {
			deploy.Status = failedDeploy.Status
		}
		c.JSON(http.StatusAccepted, gin.H{
			"deployId": deploy.ID,
			"status":   deploy.Status,
			"error":    err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"deployId": deploy.ID,
		"status":   deploy.Status,
	})
}

func (a *App) handleListDeploys(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	serverIDRaw := strings.TrimSpace(c.Query("serverId"))
	if serverIDRaw == "" {
		serverIDRaw = strings.TrimSpace(c.Query("server_id"))
	}
	if serverIDRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serverId query param is required"})
		return
	}

	serverIDParsed, err := parseUintFromString(serverIDRaw)
	if err != nil || serverIDParsed == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid serverId"})
		return
	}
	serverID := uint(serverIDParsed)

	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	query := a.db.Where("server_id = ?", serverID)

	subdirectoryFilterRaw := strings.TrimSpace(c.Query("subdirectory"))
	if subdirectoryFilterRaw != "" {
		subdirectoryFilter, valErr := normalizeAndValidateSubdirectory(subdirectoryFilterRaw, "", "")
		if valErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": valErr.Error()})
			return
		}
		query = query.Where("subdirectory = ?", subdirectoryFilter)
	}

	var deploys []models.Deploy
	if err := query.Order("id desc").Find(&deploys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load deploys"})
		return
	}

	response := make([]gin.H, 0, len(deploys))
	for _, deploy := range deploys {
		response = append(response, gin.H{
			"id":               deploy.ID,
			"serverId":         deploy.ServerID,
			"repoUrl":          deploy.RepoURL,
			"branch":           deploy.Branch,
			"type":             deploy.ProjectType,
			"subdirectory":     deploy.Subdirectory,
			"buildCommand":     deploy.BuildCommand,
			"outputDir":        deploy.OutputDir,
			"status":           deploy.Status,
			"url":              deploy.URL,
			"port":             deploy.Port,
			"deployLogPreview": truncateDeployLog(deploy.DeployLog, 800),
			"createdAt":        deploy.CreatedAt,
			"updatedAt":        deploy.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

func (a *App) handleDeployLog(c *gin.Context) {
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
	if err := a.db.Where("id = ?", deployID).First(&deploy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deploy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load deploy"})
		return
	}

	if _, err := a.requireServerForUser(userID, deploy.ServerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployId":     deploy.ID,
		"status":       deploy.Status,
		"url":          deploy.URL,
		"port":         deploy.Port,
		"type":         deploy.ProjectType,
		"subdirectory": deploy.Subdirectory,
		"deployLog":    deploy.DeployLog,
	})
}

func (a *App) handleGetDeploy(c *gin.Context) {
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
	if err := a.db.Where("id = ?", deployID).First(&deploy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deploy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load deploy"})
		return
	}

	if _, err := a.requireServerForUser(userID, deploy.ServerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           deploy.ID,
		"serverId":     deploy.ServerID,
		"repoUrl":      deploy.RepoURL,
		"branch":       deploy.Branch,
		"type":         deploy.ProjectType,
		"subdirectory": deploy.Subdirectory,
		"buildCommand": deploy.BuildCommand,
		"outputDir":    deploy.OutputDir,
		"status":       deploy.Status,
		"url":          deploy.URL,
		"port":         deploy.Port,
		"createdAt":    deploy.CreatedAt,
		"updatedAt":    deploy.UpdatedAt,
	})
}

func (a *App) handleDeleteDeploy(c *gin.Context) {
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
	if err := a.db.Where("id = ?", deployID).First(&deploy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deploy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load deploy"})
		return
	}

	if _, err := a.requireServerForUser(userID, deploy.ServerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load server"})
		return
	}

	if err := a.sendStopDeployCommandToAgent(deploy.ServerID, deploy.ID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("deploy_id = ?", deploy.ID).Delete(&models.DeployLog{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.Deploy{}, deploy.ID).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete deploy"})
		return
	}

	log.Printf("deploy deleted (deploy_id=%d server_id=%d user_id=%d)", deploy.ID, deploy.ServerID, userID)
	c.JSON(http.StatusOK, gin.H{
		"deployId": deploy.ID,
		"status":   "deleted",
	})
}

func (a *App) sendDeployCommandToAgent(serverID uint, deployID uint, repoURL, branch, projectType, subdirectory, buildCommand, outputDir string) error {
	normalizedRepoURL := strings.TrimSpace(repoURL)
	normalizedBranch := normalizeDeployBranch(branch)
	normalizedProjectType := normalizeProjectType(projectType, "", "")
	normalizedSubdirectory, err := normalizeAndValidateSubdirectory(subdirectory, "", "")
	if err != nil {
		return err
	}
	normalizedBuild := strings.TrimSpace(buildCommand)
	normalizedOutput := strings.TrimSpace(outputDir)

	payload := map[string]any{
		"deploy_id":    deployID,
		"deployId":     deployID,
		"source":       "github",
		"repo_url":     normalizedRepoURL,
		"repoUrl":      normalizedRepoURL,
		"branch":       normalizedBranch,
		"project_type": normalizedProjectType,
		"projectType":  normalizedProjectType,
		"type":         normalizedProjectType,
		// Agent contract:
		// 1) clone repository into a temp directory,
		// 2) if subdirectory is not empty, use repoDir/subdirectory as working directory,
		// 3) run detect/build/start inside that directory.
		"subdirectory":  normalizedSubdirectory,
		"sub_directory": normalizedSubdirectory,
		"build_command": normalizedBuild,
		"buildCommand":  normalizedBuild,
		"output_dir":    normalizedOutput,
		"outputDir":     normalizedOutput,
	}

	log.Printf("dispatch deploy command: deploy_id=%d server_id=%d command=deploy branch=%q repo=%q type=%q subdirectory=%q", deployID, serverID, normalizedBranch, normalizedRepoURL, normalizedProjectType, normalizedSubdirectory)
	if _, err := a.hub.RequestAgent(serverID, "deploy", payload, 10*time.Second); err != nil {
		errText := normalizeAgentDispatchError(err)
		log.Printf("send deploy command failed (deploy_id=%d server_id=%d): %s", deployID, serverID, errText)

		update := map[string]any{
			"status":        "failed",
			"error_message": errText,
			"deploy_log":    appendDeployLogLine("", errText),
		}
		if dbErr := a.db.Model(&models.Deploy{}).Where("id = ?", deployID).Updates(update).Error; dbErr != nil {
			log.Printf("mark deploy failed update error (deploy_id=%d): %v", deployID, dbErr)
		}
		return errors.New(errText)
	}

	log.Printf("deploy command acknowledged (deploy_id=%d server_id=%d)", deployID, serverID)
	return nil
}

func (a *App) sendStopDeployCommandToAgent(serverID uint, deployID uint) error {
	payload := map[string]any{
		"deploy_id": deployID,
		"deployId":  deployID,
	}
	log.Printf("dispatch stop_deploy command: deploy_id=%d server_id=%d", deployID, serverID)
	if _, err := a.hub.RequestAgent(serverID, "stop_deploy", payload, 10*time.Second); err != nil {
		errText := normalizeAgentDispatchError(err)
		log.Printf("send stop_deploy command failed (deploy_id=%d server_id=%d): %s", deployID, serverID, errText)
		return errors.New(errText)
	}

	log.Printf("stop_deploy command acknowledged (deploy_id=%d server_id=%d)", deployID, serverID)
	return nil
}

func normalizeDeployBranch(raw string) string {
	branch := strings.TrimSpace(raw)
	if branch == "" {
		return "main"
	}
	return branch
}

func normalizeProjectType(primary, fallback, alt string) string {
	projectType := strings.ToLower(strings.TrimSpace(primary))
	if projectType == "" {
		projectType = strings.ToLower(strings.TrimSpace(fallback))
	}
	if projectType == "" {
		projectType = strings.ToLower(strings.TrimSpace(alt))
	}
	return projectType
}

func normalizeAndValidateSubdirectory(primary, fallback, alt string) (string, error) {
	subdirectory := strings.TrimSpace(primary)
	if subdirectory == "" {
		subdirectory = strings.TrimSpace(fallback)
	}
	if subdirectory == "" {
		subdirectory = strings.TrimSpace(alt)
	}
	if subdirectory == "" {
		return "", nil
	}

	if strings.Contains(subdirectory, "\\") {
		return "", errors.New("subdirectory must use '/' and cannot contain '\\'")
	}

	if path.IsAbs(subdirectory) {
		return "", errors.New("subdirectory must be a relative path")
	}

	cleaned := path.Clean(subdirectory)
	if cleaned == "." {
		return "", nil
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("subdirectory cannot escape repository root")
	}

	if strings.HasPrefix(cleaned, "/") {
		return "", errors.New("subdirectory must be a relative path")
	}

	return cleaned, nil
}

func normalizeOptionalString(primary, fallback *string) string {
	if primary != nil {
		return strings.TrimSpace(*primary)
	}
	if fallback != nil {
		return strings.TrimSpace(*fallback)
	}
	return ""
}

func truncateDeployLog(raw string, maxLen int) string {
	if maxLen <= 0 || len(raw) <= maxLen {
		return raw
	}
	if maxLen <= 3 {
		return raw[:maxLen]
	}
	return raw[:maxLen-3] + "..."
}

func appendDeployLogLine(existing, line string) string {
	cleanLine := strings.TrimRight(strings.TrimSpace(line), "\n")
	if cleanLine == "" {
		return existing
	}
	if existing == "" {
		return cleanLine + "\n"
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + cleanLine + "\n"
}

func normalizeAgentDispatchError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "agent offline"
	}
	if strings.Contains(strings.ToLower(message), "offline") {
		return "agent offline"
	}
	return message
}
