package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
}

type githubUserResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

func (a *App) handleGitHubOAuthStart(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if strings.TrimSpace(a.cfg.GitHubClientID) == "" || strings.TrimSpace(a.cfg.GitHubRedirectURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "github oauth is not configured"})
		return
	}

	state, err := a.buildOAuthState(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to generate oauth state"})
		return
	}

	q := url.Values{}
	q.Set("client_id", a.cfg.GitHubClientID)
	q.Set("redirect_uri", a.cfg.GitHubRedirectURL)
	q.Set("scope", "read:user repo")
	q.Set("state", state)

	c.JSON(http.StatusOK, gin.H{
		"url": "https://github.com/login/oauth/authorize?" + q.Encode(),
	})
}

func (a *App) handleGitHubOAuthCallback(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code/state"})
		return
	}

	userID, err := a.verifyOAuthState(state)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid oauth state"})
		return
	}

	token, scope, err := a.exchangeGitHubCode(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ghUser, err := a.fetchGitHubUser(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "unable to load github profile"})
		return
	}

	encToken, err := a.encryptSecret(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to persist token"})
		return
	}

	now := time.Now().UTC()
	var existing models.GitHubConnection
	err = a.db.Where("user_id = ?", userID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save connection"})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = models.GitHubConnection{
			UserID:         userID,
			GitHubUserID:   ghUser.ID,
			Login:          ghUser.Login,
			AvatarURL:      ghUser.AvatarURL,
			AccessTokenEnc: encToken,
			Scope:          scope,
			ConnectedAt:    now,
			TokenUpdatedAt: now,
		}
		if createErr := a.db.Create(&existing).Error; createErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save connection"})
			return
		}
	} else {
		update := map[string]any{
			"github_user_id":   ghUser.ID,
			"login":            ghUser.Login,
			"avatar_url":       ghUser.AvatarURL,
			"access_token_enc": encToken,
			"scope":            scope,
			"token_updated_at": now,
			"last_synced_at":   now,
		}
		if updateErr := a.db.Model(&models.GitHubConnection{}).Where("id = ?", existing.ID).Updates(update).Error; updateErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update connection"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "connected"})
}

func (a *App) handleGetGitHubConnection(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var connection models.GitHubConnection
	if err := a.db.Where("user_id = ?", userID).First(&connection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"connected": false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load connection"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":      true,
		"id":             connection.ID,
		"login":          connection.Login,
		"avatar_url":     connection.AvatarURL,
		"scope":          connection.Scope,
		"connected_at":   connection.ConnectedAt,
		"last_synced_at": connection.LastSyncedAt,
	})
}

func (a *App) handleListGitHubRepos(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var connection models.GitHubConnection
	if err := a.db.Where("user_id = ?", userID).First(&connection).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "github is not connected"})
		return
	}
	token, err := a.decryptSecret(connection.AccessTokenEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to decrypt token"})
		return
	}

	req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "novexpanel")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "github request failed"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "github rejected repository listing"})
		return
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid github response"})
		return
	}
	out := make([]gin.H, 0, len(raw))
	for _, item := range raw {
		out = append(out, gin.H{
			"id":             item["id"],
			"name":           item["name"],
			"full_name":      item["full_name"],
			"private":        item["private"],
			"default_branch": item["default_branch"],
			"html_url":       item["html_url"],
			"clone_url":      item["clone_url"],
		})
	}
	_ = a.db.Model(&models.GitHubConnection{}).Where("id = ?", connection.ID).Update("last_synced_at", time.Now().UTC()).Error
	c.JSON(http.StatusOK, out)
}

func (a *App) handleDisconnectGitHub(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := a.db.Where("user_id = ?", userID).Delete(&models.GitHubConnection{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to disconnect"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) exchangeGitHubCode(ctx context.Context, code string) (string, string, error) {
	if strings.TrimSpace(a.cfg.GitHubClientSecret) == "" {
		return "", "", errors.New("github oauth secret is not configured")
	}
	values := url.Values{}
	values.Set("client_id", a.cfg.GitHubClientID)
	values.Set("client_secret", a.cfg.GitHubClientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", a.cfg.GitHubRedirectURL)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(values.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "novexpanel")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", errors.New("github oauth exchange failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", errors.New("github oauth exchange rejected")
	}
	var payload githubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", errors.New("invalid github oauth response")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", "", errors.New("empty github access token")
	}
	return payload.AccessToken, payload.Scope, nil
}

func (a *App) fetchGitHubUser(ctx context.Context, token string) (*githubUserResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "novexpanel")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("github user request failed")
	}
	var user githubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (a *App) buildOAuthState(userID uint) (string, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := fmt.Sprintf("%d:%s", userID, ts)
	mac := hmac.New(sha256.New, []byte(a.cfg.TokenEncSecret))
	_, _ = mac.Write([]byte(body))
	signature := hex.EncodeToString(mac.Sum(nil))
	raw := body + ":" + signature
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func (a *App) verifyOAuthState(state string) (uint, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return 0, errors.New("invalid state")
	}
	body := parts[0] + ":" + parts[1]
	mac := hmac.New(sha256.New, []byte(a.cfg.TokenEncSecret))
	_, _ = mac.Write([]byte(body))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return 0, errors.New("state signature mismatch")
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	if time.Since(time.Unix(ts, 0)) > 10*time.Minute {
		return 0, errors.New("state expired")
	}
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid user id")
	}
	return uint(id), nil
}

func (a *App) encryptSecret(plain string) (string, error) {
	key := sha256.Sum256([]byte(a.cfg.TokenEncSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	packed := append(nonce, ciphertext...)
	return base64.RawStdEncoding.EncodeToString(packed), nil
}

func (a *App) decryptSecret(enc string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(a.cfg.TokenEncSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted payload")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
