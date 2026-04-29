package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"novexpanel/backend/internal/auth"
	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type updateMeRequest struct {
	Email       *string `json:"email"`
	NewPassword *string `json:"new_password"`
}

type createMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type createAPITokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn int    `json:"expires_in_days"`
}

func (a *App) handleUpdateMe(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]any{}
	if req.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*req.Email))
		if email == "" || len(email) > 254 || !strings.Contains(email, "@") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
			return
		}
		updates["email"] = email
	}
	if req.NewPassword != nil {
		if len(*req.NewPassword) < 8 || len(*req.NewPassword) > 1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password"})
			return
		}
		hash, err := auth.HashPassword(*req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to hash password"})
			return
		}
		updates["password_hash"] = hash
	}
	if len(updates) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	if err := a.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "unable to update profile"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) handleListMembers(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	var members []models.ProjectAccess
	if err := a.db.Where("user_id = ?", userID).Order("id desc").Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load members"})
		return
	}
	c.JSON(http.StatusOK, members)
}

func (a *App) handleCreateMember(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	var req createMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	role := strings.TrimSpace(strings.ToLower(req.Role))
	if email == "" || !strings.Contains(email, "@") || len(email) > 254 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
		return
	}
	switch role {
	case "viewer", "developer", "admin":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	member := models.ProjectAccess{UserID: userID, Email: email, Role: role, InvitedByID: userID}
	if err := a.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to add member"})
		return
	}
	c.JSON(http.StatusCreated, member)
}

func (a *App) handleDeleteMember(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	memberID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member id"})
		return
	}
	if err := a.db.Where("id = ? AND user_id = ?", memberID, userID).Delete(&models.ProjectAccess{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete member"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) handleCreateAPIToken(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	var req createAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token name"})
		return
	}
	expiresInDays := req.ExpiresIn
	if expiresInDays <= 0 {
		expiresInDays = 90
	}
	if expiresInDays > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expires_in_days must be <= 365"})
		return
	}
	plain, tokenHash, tokenPrefix, err := generateAPIToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create token"})
		return
	}
	exp := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)
	token := models.APIToken{
		UserID: userID, Name: name, TokenHash: tokenHash, TokenPrefix: tokenPrefix, ExpiresAt: &exp,
	}
	if err := a.db.Create(&token).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": token.ID, "token": plain, "token_prefix": token.TokenPrefix, "expires_at": token.ExpiresAt})
}

func (a *App) handleListAPITokens(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	var tokens []models.APIToken
	if err := a.db.Where("user_id = ?", userID).Order("id desc").Find(&tokens).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list api tokens"})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

func (a *App) handleRevokeAPIToken(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	tokenID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}
	var token models.APIToken
	if err := a.db.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load token"})
		return
	}
	if err := a.db.Model(&models.APIToken{}).Where("id = ?", token.ID).Update("revoked", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to revoke token"})
		return
	}
	c.Status(http.StatusNoContent)
}

func generateAPIToken() (plain, hash, prefix string, err error) {
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", err
	}
	plain = "nvx_ci_" + hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(digest[:])
	prefix = plain[:14]
	return plain, hash, prefix, nil
}
