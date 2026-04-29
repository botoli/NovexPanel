package app

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type secretUpsertRequest struct {
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Value        string     `json:"value"`
	ServiceScope string     `json:"service_scope"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

func (a *App) loadUserRole(userID uint) string {
	var user models.User
	if err := a.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return "viewer"
	}
	role := strings.TrimSpace(strings.ToLower(user.Role))
	if role == "" {
		return "developer"
	}
	return role
}

func (a *App) requireSecretWriteRole(c *gin.Context, userID uint) bool {
	role := a.loadUserRole(userID)
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "rbac: viewer cannot modify secrets"})
		return false
	}
	return true
}

func normalizeSecretType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "env", "env_var", "env-vars":
		return "env_var"
	case "api", "api_key", "api-key":
		return "api_key"
	case "ssh", "ssh_key", "ssh-key":
		return "ssh_key"
	case "cert", "certificate":
		return "certificate"
	case "token":
		return "token"
	default:
		return ""
	}
}

func (a *App) logSecretAudit(userID, serverID uint, secretID *uint, action, target, message string) {
	_ = a.db.Create(&models.SecretVaultAuditLog{
		UserID:    userID,
		ServerID:  serverID,
		SecretID:  secretID,
		Action:    action,
		Target:    target,
		Message:   strings.TrimSpace(message),
		CreatedAt: time.Now().UTC(),
	}).Error
}

func (a *App) handleListSecrets(c *gin.Context) {
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
	serviceScope := strings.TrimSpace(c.Query("service_scope"))
	var items []models.SecretVaultItem
	query := a.db.Where("user_id = ? AND server_id = ?", userID, serverID)
	if serviceScope != "" {
		query = query.Where("service_scope = ?", serviceScope)
	}
	if err := query.Order("updated_at desc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list secrets"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (a *App) handleCreateSecret(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if !a.requireSecretWriteRole(c, userID) {
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req secretUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	secretType := normalizeSecretType(req.Type)
	if secretType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid secret type"})
		return
	}
	if strings.TrimSpace(req.Value) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value is required"})
		return
	}
	masterKey := deriveMasterKey(a.cfg.TokenEncSecret)
	encValue, encDEK, nonce, err := encryptSecretEnvelope(masterKey, req.Value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to encrypt secret"})
		return
	}
	item := models.SecretVaultItem{
		UserID:         userID,
		ServerID:       serverID,
		ServiceScope:   strings.TrimSpace(req.ServiceScope),
		Name:           name,
		Type:           secretType,
		EncryptedValue: encValue,
		EncryptedDEK:   encDEK,
		Nonce:          nonce,
		KeyVersion:     1,
		MaskedValue:    maskSecret(req.Value),
		ExpiresAt:      req.ExpiresAt,
	}
	if err := a.db.Create(&item).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "secret already exists"})
		return
	}
	a.logSecretAudit(userID, serverID, &item.ID, "create", "secrets", item.Name)
	c.JSON(http.StatusCreated, gin.H{"id": item.ID})
}

func (a *App) handleUpdateSecret(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	secretID, err := parseUintParam(c, "secretId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid secret id"})
		return
	}
	if !a.requireSecretWriteRole(c, userID) {
		return
	}
	var item models.SecretVaultItem
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", secretID, userID, serverID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
		return
	}
	var req secretUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]any{"service_scope": strings.TrimSpace(req.ServiceScope), "expires_at": req.ExpiresAt}
	if strings.TrimSpace(req.Value) != "" {
		masterKey := deriveMasterKey(a.cfg.TokenEncSecret)
		encValue, encDEK, nonce, encErr := encryptSecretEnvelope(masterKey, req.Value)
		if encErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to encrypt secret"})
			return
		}
		now := time.Now().UTC()
		updates["encrypted_value"] = encValue
		updates["encrypted_dek"] = encDEK
		updates["nonce"] = nonce
		updates["masked_value"] = maskSecret(req.Value)
		updates["last_rotated_at"] = &now
	}
	if err := a.db.Model(&models.SecretVaultItem{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update secret"})
		return
	}
	a.logSecretAudit(userID, serverID, &item.ID, "update", "secrets", item.Name)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *App) handleRotateSecret(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	secretID, err := parseUintParam(c, "secretId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid secret id"})
		return
	}
	if !a.requireSecretWriteRole(c, userID) {
		return
	}
	var item models.SecretVaultItem
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", secretID, userID, serverID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
		return
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Value) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value is required"})
		return
	}
	masterKey := deriveMasterKey(a.cfg.TokenEncSecret)
	encValue, encDEK, nonce, encErr := encryptSecretEnvelope(masterKey, req.Value)
	if encErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to encrypt secret"})
		return
	}
	now := time.Now().UTC()
	_ = a.db.Model(&models.SecretVaultItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"encrypted_value": encValue,
		"encrypted_dek":   encDEK,
		"nonce":           nonce,
		"masked_value":    maskSecret(req.Value),
		"last_rotated_at": &now,
		"key_version":     item.KeyVersion + 1,
		"revoked_at":      nil,
	}).Error
	a.logSecretAudit(userID, serverID, &item.ID, "rotate", "rotation", item.Name)
	c.JSON(http.StatusOK, gin.H{"status": "rotated"})
}

func (a *App) handleRevokeSecret(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	secretID, err := parseUintParam(c, "secretId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid secret id"})
		return
	}
	if !a.requireSecretWriteRole(c, userID) {
		return
	}
	var item models.SecretVaultItem
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", secretID, userID, serverID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
		return
	}
	now := time.Now().UTC()
	_ = a.db.Model(&models.SecretVaultItem{}).Where("id = ?", item.ID).Update("revoked_at", &now).Error
	a.logSecretAudit(userID, serverID, &item.ID, "revoke", "revocation", item.Name)
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func (a *App) handleRevealSecret(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	secretID, err := parseUintParam(c, "secretId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid secret id"})
		return
	}
	if !a.requireSecretWriteRole(c, userID) {
		return
	}
	var item models.SecretVaultItem
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", secretID, userID, serverID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
		return
	}
	if item.RevokedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "secret is revoked"})
		return
	}
	masterKey := deriveMasterKey(a.cfg.TokenEncSecret)
	plain, decErr := decryptSecretEnvelope(masterKey, item.EncryptedValue, item.EncryptedDEK, item.Nonce)
	if decErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to decrypt secret"})
		return
	}
	now := time.Now().UTC()
	_ = a.db.Model(&models.SecretVaultItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"usage_count":  item.UsageCount + 1,
		"last_used_at": &now,
	}).Error
	a.logSecretAudit(userID, serverID, &item.ID, "reveal_partial", "reveal", item.Name)
	// Never return full value; partial reveal only.
	partial := ""
	if len(plain) <= 6 {
		partial = "***"
	} else {
		partial = plain[:3] + strings.Repeat("*", len(plain)-6) + plain[len(plain)-3:]
	}
	c.JSON(http.StatusOK, gin.H{"value": partial})
}

func (a *App) resolveSecretValue(userID, serverID uint, name, serviceScope string) (string, *models.SecretVaultItem, error) {
	var item models.SecretVaultItem
	query := a.db.Where("user_id = ? AND server_id = ? AND name = ?", userID, serverID, strings.TrimSpace(name))
	scope := strings.TrimSpace(serviceScope)
	if scope != "" {
		query = query.Where("service_scope = ? OR service_scope = ''", scope)
	}
	if err := query.Order("id desc").First(&item).Error; err != nil {
		return "", nil, err
	}
	if item.RevokedAt != nil {
		return "", nil, errors.New("secret revoked")
	}
	if item.ExpiresAt != nil && item.ExpiresAt.Before(time.Now().UTC()) {
		return "", nil, errors.New("secret expired")
	}
	masterKey := deriveMasterKey(a.cfg.TokenEncSecret)
	plain, err := decryptSecretEnvelope(masterKey, item.EncryptedValue, item.EncryptedDEK, item.Nonce)
	if err != nil {
		return "", nil, err
	}
	return plain, &item, nil
}

func (a *App) handleInjectSecrets(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if !a.requireSecretWriteRole(c, userID) {
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req struct {
		Target       string   `json:"target"`
		ServiceScope string   `json:"service_scope"`
		Names        []string `json:"names"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
		return
	}
	out := make(map[string]string, len(req.Names))
	for _, name := range req.Names {
		plain, item, err := a.resolveSecretValue(userID, serverID, name, req.ServiceScope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to resolve secret: " + strings.TrimSpace(name)})
			return
		}
		out[name] = plain
		now := time.Now().UTC()
		_ = a.db.Model(&models.SecretVaultItem{}).Where("id = ?", item.ID).Updates(map[string]any{
			"usage_count":  item.UsageCount + 1,
			"last_used_at": &now,
		}).Error
		a.logSecretAudit(userID, serverID, &item.ID, "inject", req.Target, item.Name)
	}
	c.JSON(http.StatusOK, gin.H{"injected": out})
}

func (a *App) handleSecretsAudit(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, _ := parseUintParam(c, "id")
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var logs []models.SecretVaultAuditLog
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(200).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load secret audit logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (a *App) rotateExpiredSecretsTick() {
	now := time.Now().UTC()
	var items []models.SecretVaultItem
	_ = a.db.Where("expires_at IS NOT NULL AND expires_at <= ? AND revoked_at IS NULL", now).Find(&items).Error
	for _, item := range items {
		_ = a.db.Model(&models.SecretVaultItem{}).Where("id = ?", item.ID).Update("revoked_at", &now).Error
		msg := "expired and auto-revoked by scheduler"
		a.logSecretAudit(item.UserID, item.ServerID, &item.ID, "expire_revoke", "scheduler", msg)
	}
}
