package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fileApplyRequest struct {
	Provider     string `json:"provider"`
	Path         string `json:"path"`
	Content      string `json:"content"`
	ExpectedHash string `json:"expected_hash"`
	LockToken    string `json:"lock_token"`
}

func fileChecksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func normalizeFileProvider(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "generic":
		return "generic"
	case "nginx":
		return "nginx"
	case "docker-compose", "docker_compose", "compose":
		return "docker-compose"
	case "systemd":
		return "systemd"
	case "env":
		return "env"
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "toml":
		return "toml"
	case "conf":
		return "conf"
	default:
		return ""
	}
}

func (a *App) lockKey(serverID uint, path string) string {
	return fmt.Sprintf("%d:%s", serverID, strings.TrimSpace(strings.ToLower(path)))
}

func (a *App) handleFileTree(c *gin.Context) {
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
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		path = "/etc"
	}
	raw, err := a.hub.RequestAgent(serverID, "file_list", map[string]any{"path": path}, 20*time.Second)
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

func (a *App) handleFileContent(c *gin.Context) {
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
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "file_read", map[string]any{"path": path}, 20*time.Second)
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

func (a *App) handleFileValidate(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req struct {
		Provider string `json:"provider"`
		Path     string `json:"path"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	provider := normalizeFileProvider(req.Provider)
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "file_validate", map[string]any{
		"provider": provider,
		"path":     req.Path,
		"content":  req.Content,
	}, 25*time.Second)
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

func (a *App) handleFileLock(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	key := a.lockKey(serverID, req.Path)
	a.fileLockMu.Lock()
	defer a.fileLockMu.Unlock()
	if existing, ok := a.fileLocks[key]; ok && existing.ExpiresAt.After(time.Now()) && existing.UserID != userID {
		c.JSON(http.StatusConflict, gin.H{"error": "file is locked by another user"})
		return
	}
	token := uuid.NewString()
	a.fileLocks[key] = fileLockState{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	c.JSON(http.StatusOK, gin.H{"lock_token": token, "expires_at": a.fileLocks[key].ExpiresAt})
}

func (a *App) handleFileUnlock(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	var req struct {
		Path      string `json:"path"`
		LockToken string `json:"lock_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	key := a.lockKey(serverID, req.Path)
	a.fileLockMu.Lock()
	defer a.fileLockMu.Unlock()
	if lock, ok := a.fileLocks[key]; ok && lock.UserID == userID && lock.Token == req.LockToken {
		delete(a.fileLocks, key)
	}
	c.JSON(http.StatusOK, gin.H{"status": "unlocked"})
}

func (a *App) handleFileApply(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req fileApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	provider := normalizeFileProvider(req.Provider)
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	key := a.lockKey(serverID, req.Path)
	a.fileLockMu.Lock()
	lock, hasLock := a.fileLocks[key]
	a.fileLockMu.Unlock()
	if !hasLock || lock.UserID != userID || lock.Token != strings.TrimSpace(req.LockToken) || lock.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusConflict, gin.H{"error": "missing or expired lock"})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "file_apply", map[string]any{
		"provider":      provider,
		"path":          req.Path,
		"content":       req.Content,
		"expected_hash": req.ExpectedHash,
	}, 45*time.Second)
	success := err == nil
	message := ""
	if err != nil {
		message = publicAgentError(err)
	}
	checksum := fileChecksum(req.Content)
	_ = a.db.Create(&models.FileOpAuditLog{
		UserID:   userID,
		ServerID: serverID,
		Path:     req.Path,
		Provider: provider,
		Action:   "apply",
		Success:  success,
		Message:  message,
		Checksum: checksum,
	}).Error
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": message})
		return
	}
	var decoded map[string]any
	if unmarshalErr := json.Unmarshal(raw, &decoded); unmarshalErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	validationRaw, _ := json.Marshal(decoded["validation"])
	_ = a.db.Create(&models.FileOpVersion{
		UserID:     userID,
		ServerID:   serverID,
		Path:       req.Path,
		Checksum:   checksum,
		Content:    req.Content,
		Provider:   provider,
		Validation: datatypes.JSON(validationRaw),
		CreatedBy:  userID,
	}).Error
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleFileRollback(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req struct {
		Path      string `json:"path"`
		VersionID uint   `json:"version_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.VersionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version_id is required"})
		return
	}
	var version models.FileOpVersion
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", req.VersionID, userID, serverID).First(&version).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	raw, err := a.hub.RequestAgent(serverID, "file_rollback", map[string]any{
		"path":    version.Path,
		"content": version.Content,
	}, 30*time.Second)
	success := err == nil
	msg := ""
	if err != nil {
		msg = publicAgentError(err)
	}
	_ = a.db.Create(&models.FileOpAuditLog{
		UserID:   userID,
		ServerID: serverID,
		Path:     version.Path,
		Provider: version.Provider,
		Action:   "rollback",
		Success:  success,
		Message:  msg,
		Checksum: version.Checksum,
	}).Error
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": msg})
		return
	}
	decoded, decErr := decodeRawJSON(raw)
	if decErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleFileHistory(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	var rows []models.FileOpVersion
	if err := a.db.Where("user_id = ? AND server_id = ? AND path = ?", userID, serverID, path).Order("id desc").Limit(100).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load history"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *App) handleFileAudit(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var rows []models.FileOpAuditLog
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(200).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load audit"})
		return
	}
	c.JSON(http.StatusOK, rows)
}
