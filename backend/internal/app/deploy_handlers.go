package app

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	maxRepoURLLength      = 2048
	maxBranchLength       = 128
	maxSubdirectoryLength = 255
	maxOutputDirLength    = 255
	maxBuildCommandLength = 2048
	maxDeployEnvKeys      = 64
	maxDeployEnvKeyLen    = 128
	maxDeployEnvValueLen  = 4096
)

var (
	gitSSHRepoURLPattern = regexp.MustCompile(`^git@[A-Za-z0-9._-]+:[A-Za-z0-9._/-]+(?:\.git)?$`)
	deployBranchPattern  = regexp.MustCompile(`^[A-Za-z0-9._/@\-]+$`)
	pathPattern          = regexp.MustCompile(`^[A-Za-z0-9._/\-]+$`)
	deployEnvKeyPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type CreateDeployRequest struct {
	ServerID           uint              `json:"serverId"`
	ServerIDLegacy     uint              `json:"server_id"`
	RepoURL            string            `json:"repoUrl"`
	RepoURLLegacy      string            `json:"repo_url"`
	Branch             string            `json:"branch"`
	Type               string            `json:"type"`
	TypeLegacy         string            `json:"project_type"`
	TypeCamel          string            `json:"projectType"`
	Subdirectory       string            `json:"subdirectory"`
	SubdirectoryLegacy string            `json:"sub_directory"`
	SubdirectoryCamel  string            `json:"subDirectory"`
	BuildCommand       *string           `json:"buildCommand"`
	BuildCommandLegacy *string           `json:"build_command"`
	OutputDir          *string           `json:"outputDir"`
	OutputDirLegacy    *string           `json:"output_dir"`
	EnvVars            map[string]string `json:"envVars"`
	EnvVarsSnake       map[string]string `json:"env_vars"`
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
	if err := validateRepoURL(repoURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	branch, err := normalizeAndValidateDeployBranch(req.Branch)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projectType := normalizeProjectType(req.Type, req.TypeLegacy, req.TypeCamel)
	if err := validateProjectType(projectType); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subdirectory, err := normalizeAndValidateSubdirectory(req.Subdirectory, req.SubdirectoryLegacy, req.SubdirectoryCamel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	buildCommand := normalizeOptionalString(req.BuildCommand, req.BuildCommandLegacy)
	if err := validateBuildCommand(buildCommand); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	outputDir := normalizeOptionalString(req.OutputDir, req.OutputDirLegacy)
	if err := validateOutputDir(outputDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	envVars, err := mergeAndValidateEnvVars(req.EnvVars, req.EnvVarsSnake)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	envVarsJSON, err := marshalDeployEnvVars(envVars)
	if err != nil {
		log.Printf("marshal deploy env vars failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to process environment variables"})
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

	deploy := models.Deploy{
		UserID:       userID,
		ServerID:     serverID,
		RepoURL:      repoURL,
		Branch:       branch,
		ProjectType:  projectType,
		Subdirectory: subdirectory,
		BuildCommand: buildCommand,
		OutputDir:    outputDir,
		EnvVars:      envVarsJSON,
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

	if err := a.sendDeployCommandToAgent(serverID, deploy.ID, repoURL, branch, projectType, subdirectory, buildCommand, outputDir, envVars); err != nil {
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

	var deployLogs []models.DeployLog
	if err := a.db.Where("deploy_id = ?", deployID).Order("created_at ASC").Find(&deployLogs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load logs"})
		return
	}

	type LogLine struct {
		Line   string `json:"line"`
		Stream string `json:"stream"`
	}

	lines := make([]LogLine, 0)
	for _, log := range deployLogs {
		stream := "stdout"
		if log.IsError {
			stream = "stderr"
		}
		lines = append(lines, LogLine{
			Line:   log.Line,
			Stream: stream,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"lines": lines,
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

	envVars := parseDeployEnvVars(deploy.EnvVars)
	var errorMessage any
	trimmedError := strings.TrimSpace(deploy.ErrorMessage)
	if trimmedError != "" {
		errorMessage = trimmedError
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            deploy.ID,
		"serverId":      deploy.ServerID,
		"repoUrl":       deploy.RepoURL,
		"branch":        deploy.Branch,
		"type":          deploy.ProjectType,
		"subdirectory":  deploy.Subdirectory,
		"buildCommand":  deploy.BuildCommand,
		"outputDir":     deploy.OutputDir,
		"status":        deploy.Status,
		"url":           deploy.URL,
		"port":          deploy.Port,
		"createdAt":     deploy.CreatedAt,
		"updatedAt":     deploy.UpdatedAt,
		"finishedAt":    deploy.FinishedAt,
		"errorMessage":  errorMessage,
		"envVars":       envVars,
		"build_command": deploy.BuildCommand,
		"output_dir":    deploy.OutputDir,
		"finished_at":   deploy.FinishedAt,
		"error_message": errorMessage,
		"env_vars":      envVars,
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

func (a *App) sendDeployCommandToAgent(serverID uint, deployID uint, repoURL, branch, projectType, subdirectory, buildCommand, outputDir string, envVars map[string]string) error {
	normalizedRepoURL := strings.TrimSpace(repoURL)
	if err := validateRepoURL(normalizedRepoURL); err != nil {
		return err
	}

	normalizedBranch, err := normalizeAndValidateDeployBranch(branch)
	if err != nil {
		return err
	}

	normalizedProjectType := normalizeProjectType(projectType, "", "")
	if err := validateProjectType(normalizedProjectType); err != nil {
		return err
	}

	normalizedSubdirectory, err := normalizeAndValidateSubdirectory(subdirectory, "", "")
	if err != nil {
		return err
	}
	normalizedBuild := strings.TrimSpace(buildCommand)
	if err := validateBuildCommand(normalizedBuild); err != nil {
		return err
	}
	normalizedOutput := strings.TrimSpace(outputDir)
	if err := validateOutputDir(normalizedOutput); err != nil {
		return err
	}
	if envVars == nil {
		envVars = map[string]string{}
	}

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
		"envVars":       envVars,
		"env_vars":      envVars,
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

func normalizeAndValidateDeployBranch(raw string) (string, error) {
	branch := normalizeDeployBranch(raw)
	if utf8.RuneCountInString(branch) > maxBranchLength {
		return "", errors.New("branch is too long")
	}
	if hasDangerousInputChars(branch) {
		return "", errors.New("branch contains forbidden characters")
	}
	if strings.Contains(branch, "..") || strings.Contains(branch, "//") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return "", errors.New("invalid branch")
	}
	if strings.Contains(branch, "@{") {
		return "", errors.New("invalid branch")
	}
	if !deployBranchPattern.MatchString(branch) {
		return "", errors.New("invalid branch")
	}
	return branch, nil
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

func validateProjectType(projectType string) error {
	if projectType == "" {
		return nil
	}

	switch projectType {
	case "auto", "go", "node", "python", "static", "vite", "react", "vue", "svelte", "docker":
		return nil
	default:
		return errors.New("invalid project type")
	}
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
	if utf8.RuneCountInString(subdirectory) > maxSubdirectoryLength {
		return "", errors.New("subdirectory is too long")
	}
	if hasDangerousInputChars(subdirectory) {
		return "", errors.New("subdirectory contains forbidden characters")
	}
	if !pathPattern.MatchString(subdirectory) {
		return "", errors.New("subdirectory contains invalid characters")
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

func validateBuildCommand(buildCommand string) error {
	if strings.TrimSpace(buildCommand) == "" {
		return nil
	}
	if utf8.RuneCountInString(buildCommand) > maxBuildCommandLength {
		return errors.New("buildCommand is too long")
	}
	if strings.ContainsRune(buildCommand, '\x00') {
		return errors.New("buildCommand contains forbidden characters")
	}
	return nil
}

func validateOutputDir(outputDir string) error {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil
	}
	if utf8.RuneCountInString(outputDir) > maxOutputDirLength {
		return errors.New("outputDir is too long")
	}
	if hasDangerousInputChars(outputDir) {
		return errors.New("outputDir contains forbidden characters")
	}
	if _, err := normalizeAndValidateSubdirectory(outputDir, "", ""); err != nil {
		return errors.New("invalid outputDir")
	}
	return nil
}

func validateRepoURL(repoURL string) error {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return errors.New("repoUrl is required")
	}
	if utf8.RuneCountInString(repoURL) > maxRepoURLLength {
		return errors.New("repoUrl is too long")
	}
	if hasDangerousInputChars(repoURL) || strings.ContainsAny(repoURL, " \t\n\r") {
		return errors.New("repoUrl contains forbidden characters")
	}

	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		parsed, err := url.Parse(repoURL)
		if err != nil || parsed.Host == "" {
			return errors.New("invalid repoUrl")
		}
		return nil
	}

	if gitSSHRepoURLPattern.MatchString(repoURL) {
		return nil
	}

	return errors.New("invalid repoUrl")
}

func mergeAndValidateEnvVars(primary, secondary map[string]string) (map[string]string, error) {
	out := make(map[string]string)
	for k, v := range secondary {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	for k, v := range primary {
		out[k] = v
	}
	return validateDeployEnvVars(out)
}

func validateDeployEnvVars(m map[string]string) (map[string]string, error) {
	if len(m) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		key := strings.TrimSpace(k)
		if key == "" {
			return nil, errors.New("invalid environment variable key")
		}
		if utf8.RuneCountInString(key) > maxDeployEnvKeyLen {
			return nil, errors.New("environment variable name is too long")
		}
		if !deployEnvKeyPattern.MatchString(key) {
			return nil, errors.New("invalid environment variable name")
		}
		if strings.ContainsRune(val, '\x00') || strings.ContainsRune(val, '\n') || strings.ContainsRune(val, '\r') {
			return nil, errors.New("environment variable value contains forbidden characters")
		}
		if utf8.RuneCountInString(val) > maxDeployEnvValueLen {
			return nil, errors.New("environment variable value is too long")
		}
		if hasDangerousInputChars(val) {
			return nil, errors.New("environment variable value contains forbidden characters")
		}
		out[key] = strings.TrimSpace(val)
	}
	if len(out) > maxDeployEnvKeys {
		return nil, errors.New("too many environment variables")
	}
	return out, nil
}

func marshalDeployEnvVars(envVars map[string]string) (datatypes.JSON, error) {
	if len(envVars) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}
	payload, err := json.Marshal(envVars)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(payload), nil
}

func parseDeployEnvVars(raw datatypes.JSON) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		log.Printf("decode deploy env vars failed: %v", err)
		return map[string]string{}
	}
	return out
}

func hasDangerousInputChars(value string) bool {
	for _, ch := range value {
		if ch < 32 {
			return true
		}
		switch ch {
		case '|', ';', '$', '`', '<', '>':
			return true
		}
	}
	return false
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
		return "agent request failed"
	}
	lower := strings.ToLower(message)
	if strings.Contains(lower, "offline") {
		return "agent offline"
	}
	if strings.Contains(lower, "timeout") {
		return "agent request timeout"
	}
	return "agent request failed"
}
