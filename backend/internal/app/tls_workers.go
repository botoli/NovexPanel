package app

import (
	"context"
	"strings"
	"time"

	"novexpanel/backend/internal/models"
)

func (a *App) startTLSRenewalWorker(ctx context.Context) {
  go func() {
    ticker := time.NewTicker(6 * time.Hour)
    defer ticker.Stop()
    for {
      select {
      case <-ctx.Done():
        return
      case <-ticker.C:
        a.runTLSRenewalTick()
      }
    }
  }()
}

func (a *App) runTLSRenewalTick() {
  now := time.Now().UTC()
  var certs []models.TLSCertificate
  _ = a.db.Where("auto_renew = ? AND status = ? AND (next_renew_at IS NULL OR next_renew_at <= ?)", true, "active", now).Find(&certs).Error

  for _, cert := range certs {
    a.renewTLSCertificate(cert)
  }
}

func (a *App) renewTLSCertificate(cert models.TLSCertificate) {
  domains := splitDomainsString(cert.Domains)
  if len(domains) == 0 {
    return
  }
  creds, err := decryptDNSCredentials(a.cfg.TokenEncSecret, cert)
  if err != nil {
    a.logRenewal(cert.ID, "failed", "dns credentials decrypt failed", cert.RenewalFailures+1)
    return
  }

  _, issueErr := a.issueTLSCertificate(cert.UserID, cert.ServerID, domains, tlsIssueConfig{
    Challenge:      cert.ChallengeType,
    DNSProvider:    cert.DNSProvider,
    DNSCredentials: creds,
    Force:          true,
  }, &cert)

  if issueErr == nil {
    return
  }

  failures := cert.RenewalFailures + 1
  next := time.Now().UTC().Add(renewalBackoff(failures))
  _ = a.db.Model(&models.TLSCertificate{}).Where("id = ?", cert.ID).Updates(map[string]any{
    "renewal_failures": failures,
    "next_renew_at":    timePtr(next),
    "status":           "error",
  }).Error
  a.logRenewal(cert.ID, "failed", issueErr.Error(), failures)
}

func renewalBackoff(failures int) time.Duration {
  if failures <= 0 {
    return 2 * time.Hour
  }
  if failures == 1 {
    return 2 * time.Hour
  }
  if failures == 2 {
    return 6 * time.Hour
  }
  if failures == 3 {
    return 12 * time.Hour
  }
  return 24 * time.Hour
}

func splitDomainsString(raw string) []string {
  parts := strings.Split(strings.TrimSpace(raw), ",")
  out := make([]string, 0, len(parts))
  for _, p := range parts {
    p = strings.TrimSpace(p)
    if p != "" {
      out = append(out, p)
    }
  }
  return out
}
