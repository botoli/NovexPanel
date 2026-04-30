package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type domainCreateRequest struct {
  Domain             string            `json:"domain"`
  Service            string            `json:"service"`
  Port               int               `json:"port"`
  Protocol           string            `json:"protocol"`
  AutoRenew          *bool             `json:"auto_renew"`
  IssueTLS           bool              `json:"issue_tls"`
  CertificateDomains []string          `json:"certificate_domains"`
  Challenge          string            `json:"challenge"`
  Email              string            `json:"email"`
  DNSProvider        string            `json:"dns_provider"`
  DNSCredentials     map[string]string `json:"dns_credentials"`
}

type domainUpdateRequest struct {
  Service    *string `json:"service"`
  Port       *int    `json:"port"`
  Protocol   *string `json:"protocol"`
  AutoRenew  *bool   `json:"auto_renew"`
}

type exposeAppRequest struct {
  Domain             string            `json:"domain"`
  Service            string            `json:"service"`
  Port               int               `json:"port"`
  TLS                bool              `json:"tls"`
  AutoFirewall       bool              `json:"auto_firewall"`
  FirewallProvider   string            `json:"firewall_provider"`
  Challenge          string            `json:"challenge"`
  Email              string            `json:"email"`
  DNSProvider        string            `json:"dns_provider"`
  DNSCredentials     map[string]string `json:"dns_credentials"`
  CertificateDomains []string          `json:"certificate_domains"`
}

func (a *App) handleListDomains(c *gin.Context) {
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

  var bindings []models.DomainBinding
  if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Find(&bindings).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load domains"})
    return
  }

  certMap := make(map[uint]models.TLSCertificate)
  certIDs := make([]uint, 0)
  for _, b := range bindings {
    if b.CertificateID != nil {
      certIDs = append(certIDs, *b.CertificateID)
    }
  }
  if len(certIDs) > 0 {
    var certs []models.TLSCertificate
    _ = a.db.Where("id IN ?", certIDs).Find(&certs).Error
    for _, cert := range certs {
      certMap[cert.ID] = cert
    }
  }

  response := make([]gin.H, 0, len(bindings))
  for _, binding := range bindings {
    item := gin.H{
      "id":                binding.ID,
      "domain":            binding.Domain,
      "service":           binding.Service,
      "port":              binding.Port,
      "protocol":          binding.Protocol,
      "auto_renew":        binding.AutoRenew,
      "status":            binding.Status,
      "validation_errors": binding.ValidationErrors,
      "created_at":        binding.CreatedAt,
      "updated_at":        binding.UpdatedAt,
    }
    if binding.CertificateID != nil {
      if cert, ok := certMap[*binding.CertificateID]; ok {
        item["certificate"] = gin.H{
          "id":              cert.ID,
          "issuer":          cert.Issuer,
          "not_after":       cert.NotAfter,
          "status":          cert.Status,
          "auto_renew":      cert.AutoRenew,
          "renewal_failures": cert.RenewalFailures,
        }
      }
    }
    response = append(response, item)
  }

  c.JSON(http.StatusOK, response)
}

func (a *App) handleGetDomain(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }

  response := gin.H{
    "id":                binding.ID,
    "domain":            binding.Domain,
    "service":           binding.Service,
    "port":              binding.Port,
    "protocol":          binding.Protocol,
    "auto_renew":        binding.AutoRenew,
    "status":            binding.Status,
    "validation_errors": binding.ValidationErrors,
    "created_at":        binding.CreatedAt,
    "updated_at":        binding.UpdatedAt,
  }

  if binding.CertificateID != nil {
    var cert models.TLSCertificate
    if err := a.db.Where("id = ?", *binding.CertificateID).First(&cert).Error; err == nil {
      response["certificate"] = gin.H{
        "id":              cert.ID,
        "issuer":          cert.Issuer,
        "serial":          cert.Serial,
        "not_before":      cert.NotBefore,
        "not_after":       cert.NotAfter,
        "fingerprint":     cert.Fingerprint,
        "status":          cert.Status,
        "auto_renew":      cert.AutoRenew,
        "renewal_failures": cert.RenewalFailures,
        "last_renewed_at": cert.LastRenewedAt,
        "next_renew_at":   cert.NextRenewAt,
      }
    }
  }

  c.JSON(http.StatusOK, response)
}

func (a *App) handleCreateDomain(c *gin.Context) {
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

  var req domainCreateRequest
  if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
    return
  }

  domain, err := normalizeDomain(req.Domain)
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
  }
  protocol, err := normalizeProtocol(req.Protocol)
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
  }
  if req.Port <= 0 || req.Port > 65535 {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
    return
  }
  autoRenew := true
  if req.AutoRenew != nil {
    autoRenew = *req.AutoRenew
  }

  if protocol == "https" {
    req.IssueTLS = true
  }

  var existing models.DomainBinding
  if err := a.db.Where("user_id = ? AND server_id = ? AND domain = ?", userID, serverID, domain).First(&existing).Error; err == nil {
    c.JSON(http.StatusConflict, gin.H{"error": "domain already exists"})
    return
  } else if !errors.Is(err, gorm.ErrRecordNotFound) {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to validate domain"})
    return
  }

  binding := models.DomainBinding{
    UserID:    userID,
    ServerID:  serverID,
    Domain:    domain,
    Service:   strings.TrimSpace(req.Service),
    Port:      req.Port,
    Protocol:  protocol,
    AutoRenew: autoRenew,
    Status:    "pending",
  }

  if err := a.db.Create(&binding).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create domain"})
    return
  }

  if req.IssueTLS {
    certDomains := req.CertificateDomains
    if len(certDomains) == 0 {
      certDomains = []string{domain}
    }
    certDomains, err = normalizeDomainList(certDomains)
    if err != nil {
      _ = a.db.Delete(&models.DomainBinding{}, binding.ID).Error
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }
    cert, err := a.issueTLSCertificate(userID, serverID, certDomains, tlsIssueConfig{
      Challenge:      strings.TrimSpace(req.Challenge),
      Email:          strings.TrimSpace(req.Email),
      DNSProvider:    strings.TrimSpace(req.DNSProvider),
      DNSCredentials: req.DNSCredentials,
      Force:          true,
    }, nil)
    if err != nil {
      binding.Status = "error"
      binding.ValidationErrors = err.Error()
      _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
        "status":            binding.Status,
        "validation_errors": binding.ValidationErrors,
      }).Error
      c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
      return
    }
    _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
      "certificate_id": cert.ID,
      "status":         "active",
    }).Error
    binding.CertificateID = &cert.ID
    binding.Status = "active"
  } else {
    _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Update("status", "active").Error
    binding.Status = "active"
  }

  a.logDomainAudit(userID, serverID, domain, "create", true, "", nil)
  c.JSON(http.StatusCreated, binding)
}

func (a *App) handleUpdateDomain(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }

  var req domainUpdateRequest
  if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
    return
  }

  updates := map[string]any{}
  if req.Service != nil {
    updates["service"] = strings.TrimSpace(*req.Service)
  }
  if req.Port != nil {
    if *req.Port <= 0 || *req.Port > 65535 {
      c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
      return
    }
    updates["port"] = *req.Port
  }
  if req.Protocol != nil {
    protocol, err := normalizeProtocol(*req.Protocol)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }
    updates["protocol"] = protocol
  }
  if req.AutoRenew != nil {
    updates["auto_renew"] = *req.AutoRenew
  }

  if len(updates) == 0 {
    c.Status(http.StatusNoContent)
    return
  }

  if err := a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(updates).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update domain"})
    return
  }

  a.logDomainAudit(userID, serverID, binding.Domain, "update", true, "", updates)
  c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *App) handleDeleteDomain(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }

  _ = a.db.Delete(&models.DomainBinding{}, binding.ID).Error
  a.logDomainAudit(userID, serverID, binding.Domain, "delete", true, "", nil)
  c.Status(http.StatusNoContent)
}

func (a *App) handleIssueDomainCertificate(c *gin.Context) {
  a.handleIssueOrRenewDomainCertificate(c, false)
}

func (a *App) handleRenewDomainCertificate(c *gin.Context) {
  a.handleIssueOrRenewDomainCertificate(c, true)
}

func (a *App) handleIssueOrRenewDomainCertificate(c *gin.Context, force bool) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }

  var req domainCreateRequest
  _ = c.ShouldBindJSON(&req)

  certDomains := req.CertificateDomains
  if len(certDomains) == 0 {
    if binding.Domain != "" {
      certDomains = []string{binding.Domain}
    }
  }
  certDomains, err = normalizeDomainList(certDomains)
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
  }

  var existingCert *models.TLSCertificate
  if binding.CertificateID != nil {
    var loaded models.TLSCertificate
    if err := a.db.Where("id = ?", *binding.CertificateID).First(&loaded).Error; err == nil {
      existingCert = &loaded
    }
  }

  cert, err := a.issueTLSCertificate(userID, serverID, certDomains, tlsIssueConfig{
    Challenge:      strings.TrimSpace(req.Challenge),
    Email:          strings.TrimSpace(req.Email),
    DNSProvider:    strings.TrimSpace(req.DNSProvider),
    DNSCredentials: req.DNSCredentials,
    Force:          force,
  }, existingCert)
  if err != nil {
    binding.Status = "error"
    binding.ValidationErrors = err.Error()
    _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
      "status":            binding.Status,
      "validation_errors": binding.ValidationErrors,
    }).Error
    a.logDomainAudit(userID, serverID, binding.Domain, "issue", false, err.Error(), nil)
    c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
    return
  }

  _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
    "certificate_id": cert.ID,
    "status":         "active",
    "validation_errors": "",
  }).Error

  binding.CertificateID = &cert.ID
  binding.Status = "active"
  binding.ValidationErrors = ""

  a.logDomainAudit(userID, serverID, binding.Domain, "issue", true, "", nil)
  c.JSON(http.StatusOK, gin.H{"certificate_id": cert.ID, "status": "issued"})
}

func (a *App) handleRollbackDomainCertificate(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }
  if binding.CertificateID == nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "no certificate to rollback"})
    return
  }

  var history models.TLSCertificateHistory
  if err := a.db.Where("certificate_id = ?", *binding.CertificateID).Order("id desc").First(&history).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "no certificate history"})
    return
  }

  var cert models.TLSCertificate
  if err := a.db.Where("id = ?", *binding.CertificateID).First(&cert).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
    return
  }

  bundle := tlsBundle{}
  masterKey := deriveMasterKey(a.cfg.TokenEncSecret)
  plain, err := decryptSecretEnvelope(masterKey, history.BundleEnc, history.BundleDEK, history.BundleNonce)
  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to decrypt history"})
    return
  }
  if err := json.Unmarshal([]byte(plain), &bundle); err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid history bundle"})
    return
  }

  if err := a.installCertificateOnAgent(serverID, primaryDomain(strings.Split(cert.Domains, ",")), bundle); err != nil {
    c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
    return
  }

  _ = a.db.Model(&models.TLSCertificate{}).Where("id = ?", cert.ID).Updates(map[string]any{
    "bundle_enc":   history.BundleEnc,
    "bundle_dek":   history.BundleDEK,
    "bundle_nonce": history.BundleNonce,
    "issuer":       history.Issuer,
    "serial":       history.Serial,
    "not_after":    history.NotAfter,
    "fingerprint":  history.Fingerprint,
    "status":       "active",
  }).Error

  a.logDomainAudit(userID, serverID, binding.Domain, "rollback", true, "", nil)
  c.JSON(http.StatusOK, gin.H{"status": "rolled_back"})
}

func (a *App) handleDomainDNSDiagnostics(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  server, err := a.requireServerForUser(userID, serverID)
  if err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }

  diagnostics, err := resolveDNSDiagnostics(binding.Domain, server.IP)
  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
  }

  c.JSON(http.StatusOK, diagnostics)
}

func (a *App) handleDomainCertHistory(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }
  if binding.CertificateID == nil {
    c.JSON(http.StatusOK, []models.TLSCertificateHistory{})
    return
  }

  var history []models.TLSCertificateHistory
  if err := a.db.Where("certificate_id = ?", *binding.CertificateID).Order("id desc").Find(&history).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load history"})
    return
  }
  c.JSON(http.StatusOK, history)
}

func (a *App) handleDomainRenewalLogs(c *gin.Context) {
  userID, _ := userIDFromContext(c)
  serverID, err := parseUintParam(c, "id")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
    return
  }
  domainID, err := parseUintParam(c, "domainId")
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
    return
  }
  if _, err := a.requireServerForUser(userID, serverID); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
    return
  }

  var binding models.DomainBinding
  if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", domainID, userID, serverID).First(&binding).Error; err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
    return
  }
  if binding.CertificateID == nil {
    c.JSON(http.StatusOK, []models.TLSRenewalLog{})
    return
  }

  var logs []models.TLSRenewalLog
  if err := a.db.Where("certificate_id = ?", *binding.CertificateID).Order("id desc").Limit(200).Find(&logs).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load renewal logs"})
    return
  }
  c.JSON(http.StatusOK, logs)
}

func (a *App) handleDomainAudit(c *gin.Context) {
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

  limit := 100
  if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
    if parsed, parseErr := parsePositiveInt(raw); parseErr == nil && parsed > 0 {
      limit = int(parsed)
    }
  }
  if limit > 500 {
    limit = 500
  }

  var logs []models.DomainAuditLog
  if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(limit).Find(&logs).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load audit logs"})
    return
  }
  c.JSON(http.StatusOK, logs)
}

func (a *App) handleExposeApp(c *gin.Context) {
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

  var req exposeAppRequest
  if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
    return
  }

  domain, err := normalizeDomain(req.Domain)
  if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
  }
  if req.Port <= 0 || req.Port > 65535 {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
    return
  }
  protocol := "http"
  if req.TLS {
    protocol = "https"
  }

  var binding models.DomainBinding
  if err := a.db.Where("user_id = ? AND server_id = ? AND domain = ?", userID, serverID, domain).First(&binding).Error; err != nil {
    if !errors.Is(err, gorm.ErrRecordNotFound) {
      c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load domain"})
      return
    }
    binding = models.DomainBinding{
      UserID:    userID,
      ServerID:  serverID,
      Domain:    domain,
      Service:   strings.TrimSpace(req.Service),
      Port:      req.Port,
      Protocol:  protocol,
      AutoRenew: true,
      Status:    "pending",
    }
    if err := a.db.Create(&binding).Error; err != nil {
      c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create domain"})
      return
    }
  }

  if req.TLS {
    certDomains := req.CertificateDomains
    if len(certDomains) == 0 {
      certDomains = []string{domain}
    }
    certDomains, err = normalizeDomainList(certDomains)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    var existingCert *models.TLSCertificate
    if binding.CertificateID != nil {
      var loaded models.TLSCertificate
      if err := a.db.Where("id = ?", *binding.CertificateID).First(&loaded).Error; err == nil {
        existingCert = &loaded
      }
    }

    cert, err := a.issueTLSCertificate(userID, serverID, certDomains, tlsIssueConfig{
      Challenge:      strings.TrimSpace(req.Challenge),
      Email:          strings.TrimSpace(req.Email),
      DNSProvider:    strings.TrimSpace(req.DNSProvider),
      DNSCredentials: req.DNSCredentials,
      Force:          true,
    }, existingCert)
    if err != nil {
      _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
        "status":            "error",
        "validation_errors": err.Error(),
      }).Error
      c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
      return
    }
    _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
      "certificate_id": cert.ID,
      "status":         "active",
      "validation_errors": "",
    }).Error
    binding.CertificateID = &cert.ID
  }

  if req.AutoFirewall {
    provider := req.FirewallProvider
    if provider == "" {
      provider = "ufw"
    }
    input := firewallRuleInput{
      Provider:    provider,
      Direction:   "in",
      Action:      "allow",
      Protocol:    "tcp",
      Port:        strconv.Itoa(req.Port),
      Source:      "any",
      Destination: "any",
      Comment:     "Expose app",
    }
    normalized, err := validateFirewallRuleInput(input)
    if err == nil {
      normalized.UserID = userID
      normalized.ServerID = serverID
      snapshot, snapErr := a.buildFirewallSnapshot(userID, serverID, normalized.Provider)
      if snapErr == nil {
        _ = a.db.Create(&normalized).Error
        _ = a.applyFirewallChange(serverID, normalized.Provider, "add", buildFirewallRulePayload(normalized), nil)
        a.logFirewallAudit(userID, serverID, &normalized.ID, normalized.Provider, "expose_rule", true, "", snapshot)
      }
    }
  }

  confPath := "/etc/nginx/conf.d/novex-" + strings.ReplaceAll(domain, ".", "-") + ".conf"
  conf := buildNginxConfig(domain, req.Port, req.TLS)
  if err := a.applyNginxConfig(serverID, confPath, conf); err != nil {
    c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
    return
  }

  _ = a.db.Model(&models.DomainBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
    "service":  strings.TrimSpace(req.Service),
    "port":     req.Port,
    "protocol": protocol,
    "status":   "active",
  }).Error

  a.logDomainAudit(userID, serverID, domain, "expose", true, "", nil)
  c.JSON(http.StatusOK, gin.H{"status": "exposed", "domain_id": binding.ID})
}

func buildNginxConfig(domain string, port int, tls bool) string {
  host := strings.TrimPrefix(domain, "*.")
  base := []string{
    "server {",
    "  listen 80;",
    "  server_name " + host + ";",
  }
  if tls {
    base = append(base, "  return 301 https://$host$request_uri;", "}")
    secure := []string{
      "server {",
      "  listen 443 ssl http2;",
      "  server_name " + host + ";",
      "  ssl_certificate /etc/novex/certs/" + host + "/fullchain.pem;",
      "  ssl_certificate_key /etc/novex/certs/" + host + "/privkey.pem;",
      "  location / {",
      "    proxy_pass http://127.0.0.1:" + toString(port) + ";",
      "    proxy_set_header Host $host;",
      "    proxy_set_header X-Real-IP $remote_addr;",
      "    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
      "    proxy_set_header X-Forwarded-Proto $scheme;",
      "  }",
      "}",
    }
    return strings.Join(append(base, secure...), "\n") + "\n"
  }
  base = append(base,
    "  location / {",
    "    proxy_pass http://127.0.0.1:"+toString(port)+";",
    "    proxy_set_header Host $host;",
    "    proxy_set_header X-Real-IP $remote_addr;",
    "    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
    "    proxy_set_header X-Forwarded-Proto $scheme;",
    "  }",
    "}",
  )
  return strings.Join(base, "\n") + "\n"
}

func (a *App) applyNginxConfig(serverID uint, path, content string) error {
  raw, err := a.hub.RequestAgent(serverID, "file_apply", map[string]any{
    "provider":      "nginx",
    "path":          path,
    "content":       content,
    "expected_hash": "",
  }, 45*time.Second)
  if err != nil {
    return errors.New(publicAgentError(err))
  }
  decoded, err := decodeRawJSON(raw)
  if err != nil {
    return errors.New("invalid response from agent")
  }
  payload, ok := decoded.(map[string]any)
  if ok {
    if okValue, ok := payload["ok"].(bool); ok && !okValue {
      if message, ok := payload["error"].(string); ok && message != "" {
        return errors.New(message)
      }
      return errors.New("nginx apply failed")
    }
  }
  return nil
}

func (a *App) issueTLSCertificate(userID, serverID uint, domains []string, cfg tlsIssueConfig, existing *models.TLSCertificate) (*models.TLSCertificate, error) {
  domains, err := normalizeDomainList(domains)
  if err != nil {
    return nil, err
  }

  challenge := strings.TrimSpace(cfg.Challenge)
  if challenge == "" {
    challenge = "http"
  }
  hasWildcard := false
  for _, d := range domains {
    if strings.HasPrefix(d, "*.") {
      hasWildcard = true
      break
    }
  }
  if hasWildcard && challenge != "dns" {
    return nil, errors.New("wildcard domains require dns challenge")
  }
  if challenge == "dns" && strings.TrimSpace(cfg.DNSProvider) == "" {
    return nil, errors.New("dns_provider is required for dns challenge")
  }

  payload := map[string]any{
    "domains":         domains,
    "email":           cfg.Email,
    "challenge":       challenge,
    "dns_provider":    cfg.DNSProvider,
    "dns_credentials": cfg.DNSCredentials,
    "force":           cfg.Force,
  }

  raw, err := a.hub.RequestAgent(serverID, "cert_issue", payload, 140*time.Second)
  if err != nil {
    return nil, errors.New(publicAgentError(err))
  }

  var resp struct {
    CertPEM  string `json:"cert_pem"`
    KeyPEM   string `json:"key_pem"`
    ChainPEM string `json:"chain_pem"`
    CertName string `json:"cert_name"`
  }
  if err := json.Unmarshal(raw, &resp); err != nil {
    return nil, errors.New("invalid response from agent")
  }
  if strings.TrimSpace(resp.CertPEM) == "" || strings.TrimSpace(resp.KeyPEM) == "" {
    return nil, errors.New("certificate issuance failed")
  }

  issuer, serial, notBefore, notAfter, fingerprint, err := parseCertificateMetadata(resp.CertPEM)
  if err != nil {
    return nil, err
  }

  enc, dek, nonce, err := encryptTLSBundle(a.cfg.TokenEncSecret, tlsBundle{
    CertPEM:  resp.CertPEM,
    KeyPEM:   resp.KeyPEM,
    ChainPEM: resp.ChainPEM,
  })
  if err != nil {
    return nil, err
  }

  dnsEnc, dnsDek, dnsNonce, err := encryptDNSCredentials(a.cfg.TokenEncSecret, cfg.DNSCredentials)
  if err != nil {
    return nil, err
  }

  bundle := tlsBundle{CertPEM: resp.CertPEM, KeyPEM: resp.KeyPEM, ChainPEM: resp.ChainPEM}
  if err := a.installCertificateOnAgent(serverID, primaryDomain(domains), bundle); err != nil {
    return nil, err
  }

  if existing == nil {
    cert := models.TLSCertificate{
      UserID:              userID,
      ServerID:            serverID,
      Domains:             strings.Join(domains, ","),
      Issuer:              issuer,
      Serial:              serial,
      NotBefore:           notBefore,
      NotAfter:            notAfter,
      Fingerprint:         fingerprint,
      BundleEnc:           enc,
      BundleDEK:           dek,
      BundleNonce:         nonce,
      ChallengeType:       challenge,
      DNSProvider:         strings.TrimSpace(cfg.DNSProvider),
      DNSCredentialsEnc:   dnsEnc,
      DNSCredentialsDEK:   dnsDek,
      DNSCredentialsNonce: dnsNonce,
      AutoRenew:           true,
      RenewalFailures:     0,
      Status:              "active",
    }
    if err := a.storeTLSCertificate(&cert); err != nil {
      return nil, err
    }
    a.logRenewal(cert.ID, "success", "issued", 1)
    return &cert, nil
  }

  history := models.TLSCertificateHistory{
    CertificateID: existing.ID,
    BundleEnc:     existing.BundleEnc,
    BundleDEK:     existing.BundleDEK,
    BundleNonce:   existing.BundleNonce,
    Issuer:        existing.Issuer,
    Serial:        existing.Serial,
    NotAfter:      existing.NotAfter,
    Fingerprint:   existing.Fingerprint,
    CreatedAt:     time.Now().UTC(),
  }
  _ = a.db.Create(&history).Error

  updates := map[string]any{
    "domains":               strings.Join(domains, ","),
    "issuer":                issuer,
    "serial":                serial,
    "not_before":            notBefore,
    "not_after":             notAfter,
    "fingerprint":           fingerprint,
    "bundle_enc":            enc,
    "bundle_dek":            dek,
    "bundle_nonce":          nonce,
    "dns_provider":          strings.TrimSpace(cfg.DNSProvider),
    "dns_credentials_enc":   dnsEnc,
    "dns_credentials_dek":   dnsDek,
    "dns_credentials_nonce": dnsNonce,
    "challenge_type":         challenge,
    "renewal_failures":      0,
    "last_renewed_at":        time.Now().UTC(),
    "next_renew_at":          timePtr(scheduleRenewal(notAfter)),
    "status":                "active",
  }
  if err := a.db.Model(&models.TLSCertificate{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
    return nil, errors.New("unable to update certificate")
  }
  a.logRenewal(existing.ID, "success", "renewed", 1)
  existing.Domains = strings.Join(domains, ",")
  existing.Issuer = issuer
  existing.Serial = serial
  existing.NotBefore = notBefore
  existing.NotAfter = notAfter
  existing.Fingerprint = fingerprint
  existing.BundleEnc = enc
  existing.BundleDEK = dek
  existing.BundleNonce = nonce
  existing.DNSProvider = strings.TrimSpace(cfg.DNSProvider)
  existing.DNSCredentialsEnc = dnsEnc
  existing.DNSCredentialsDEK = dnsDek
  existing.DNSCredentialsNonce = dnsNonce
  existing.ChallengeType = challenge
  existing.RenewalFailures = 0
  existing.Status = "active"
  return existing, nil
}

func (a *App) storeTLSCertificate(cert *models.TLSCertificate) error {
  cert.NextRenewAt = timePtr(scheduleRenewal(cert.NotAfter))
  if err := a.db.Create(cert).Error; err != nil {
    return errors.New("unable to store certificate")
  }
  return nil
}

func (a *App) installCertificateOnAgent(serverID uint, certName string, bundle tlsBundle) error {
  raw, err := a.hub.RequestAgent(serverID, "cert_install", map[string]any{
    "cert_name": certName,
    "cert_pem":  bundle.CertPEM,
    "key_pem":   bundle.KeyPEM,
    "chain_pem": bundle.ChainPEM,
  }, 30*time.Second)
  if err != nil {
    return errors.New(publicAgentError(err))
  }
  if _, err := decodeRawJSON(raw); err != nil {
    return errors.New("invalid response from agent")
  }
  return nil
}

func (a *App) logDomainAudit(userID, serverID uint, domain, action string, success bool, message string, payload any) {
  raw := datatypes.JSON([]byte("{}"))
  if payload != nil {
    if encoded, err := json.Marshal(payload); err == nil {
      raw = datatypes.JSON(encoded)
    }
  }
  _ = a.db.Create(&models.DomainAuditLog{
    UserID:      userID,
    ServerID:    serverID,
    Domain:      domain,
    Action:      strings.TrimSpace(action),
    Success:     success,
    Message:     strings.TrimSpace(message),
    PayloadJSON: raw,
    CreatedAt:   time.Now().UTC(),
  }).Error
}

func (a *App) logRenewal(certID uint, status, message string, attempt int) {
  _ = a.db.Create(&models.TLSRenewalLog{
    CertificateID: certID,
    Status:        strings.TrimSpace(status),
    Message:       strings.TrimSpace(message),
    Attempt:       attempt,
    CreatedAt:     time.Now().UTC(),
  }).Error
}

func timePtr(t time.Time) *time.Time {
  return &t
}
