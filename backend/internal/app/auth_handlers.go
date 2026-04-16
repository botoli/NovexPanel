package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/auth"
	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createAgentTokenRequest struct {
	Name *string `json:"name"`
}

type updateAgentTokenNameRequest struct {
	Name *string `json:"name"`
}

func (a *App) handleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6 characters"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to hash password"})
		return
	}

	user := models.User{Email: req.Email, PasswordHash: hash}
	if err := a.db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": user.ID})
}

func (a *App) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	var user models.User
	if err := a.db.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := auth.ComparePassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := auth.CreateUserToken(a.cfg.JWTSecret, user.ID, a.cfg.JWTExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"user_id":    user.ID,
		"expires_in": int64(a.cfg.JWTExpiry.Seconds()),
	})
}

func (a *App) handleCreateAgentToken(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createAgentTokenRequest
	if err := decodeJSONBodyAllowEmpty(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	tokenName := normalizeOptionalName(req.Name)

	rawToken, tokenHash, tokenPrefix, err := auth.GenerateAgentToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to generate token"})
		return
	}

	expiresAt := time.Now().Add(a.cfg.AgentTokenExpiry)
	model := models.AgentToken{
		UserID:      userID,
		Name:        tokenName,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		ExpiresAt:   &expiresAt,
	}

	if err := a.db.Create(&model).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           model.ID,
		"agent_token":  rawToken,
		"token_prefix": model.TokenPrefix,
		"expires_at":   model.ExpiresAt,
		"name":         model.Name,
	})
}

func (a *App) handleListAgentTokens(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var tokens []models.AgentToken
	if err := a.db.Where("user_id = ?", userID).Order("id desc").Find(&tokens).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list tokens"})
		return
	}

	tokenIDs := make([]uint, 0, len(tokens))
	for _, token := range tokens {
		tokenIDs = append(tokenIDs, token.ID)
	}

	serversByToken := make(map[uint]models.Server)
	if len(tokenIDs) > 0 {
		var servers []models.Server
		if err := a.db.Where("token_id IN ?", tokenIDs).Find(&servers).Error; err == nil {
			for _, server := range servers {
				serversByToken[server.TokenID] = server
			}
		}
	}

	response := make([]gin.H, 0, len(tokens))
	for _, token := range tokens {
		item := gin.H{
			"id":           token.ID,
			"name":         token.Name,
			"token_prefix": token.TokenPrefix,
			"revoked":      token.Revoked,
			"last_used_at": token.LastUsedAt,
			"expires_at":   token.ExpiresAt,
			"created_at":   token.CreatedAt,
			"server":       nil,
		}
		if server, ok := serversByToken[token.ID]; ok {
			item["server"] = gin.H{
				"id":     server.ID,
				"name":   server.Name,
				"ip":     server.IP,
				"online": server.Online,
			}
		}
		response = append(response, item)
	}

	c.JSON(http.StatusOK, response)
}

func (a *App) handleUpdateAgentTokenName(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tokenID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}

	var req updateAgentTokenNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	name := normalizeOptionalName(req.Name)

	var token models.AgentToken
	if err := a.db.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	if err := a.db.Model(&token).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update token name"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (a *App) handleRevokeAgentToken(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tokenID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}

	var token models.AgentToken
	if err := a.db.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	if !token.Revoked {
		if err := a.db.Model(&token).Update("revoked", true).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to revoke token"})
			return
		}
	}

	var server models.Server
	if err := a.db.Where("token_id = ?", token.ID).First(&server).Error; err == nil {
		a.hub.KickServer(server.ID)
	}

	c.Status(http.StatusNoContent)
}

func decodeJSONBodyAllowEmpty(c *gin.Context, dst any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single json object")
	}

	return nil
}

func normalizeOptionalName(name *string) *string {
	if name == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
