package app

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"sort"
	"strings"
	"time"

	"novexpanel/backend/internal/models"
)

type tlsBundle struct {
  CertPEM  string `json:"cert_pem"`
  KeyPEM   string `json:"key_pem"`
  ChainPEM string `json:"chain_pem"`
}

type tlsIssueConfig struct {
  Challenge     string            `json:"challenge"`
  Email         string            `json:"email"`
  DNSProvider   string            `json:"dns_provider"`
  DNSCredentials map[string]string `json:"dns_credentials"`
  Force         bool              `json:"force"`
}

func normalizeDomain(raw string) (string, error) {
  value := strings.TrimSpace(strings.ToLower(raw))
  if value == "" {
    return "", errors.New("domain is required")
  }
  value = strings.TrimSuffix(value, ".")
  wildcard := strings.HasPrefix(value, "*.")
  if wildcard {
    value = strings.TrimPrefix(value, "*.")
  }
  if len(value) > 253 {
    return "", errors.New("domain is too long")
  }
  labels := strings.Split(value, ".")
  if len(labels) < 2 {
    return "", errors.New("domain must have at least one dot")
  }
  for _, label := range labels {
    if label == "" || len(label) > 63 {
      return "", errors.New("invalid domain label")
    }
    if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
      return "", errors.New("invalid domain label")
    }
    for _, r := range label {
      if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
        return "", errors.New("invalid domain label")
      }
    }
  }
  if wildcard {
    return "*." + value, nil
  }
  return value, nil
}

func normalizeDomainList(domains []string) ([]string, error) {
  if len(domains) == 0 {
    return nil, errors.New("domains are required")
  }
  seen := make(map[string]struct{}, len(domains))
  out := make([]string, 0, len(domains))
  for _, raw := range domains {
    normalized, err := normalizeDomain(raw)
    if err != nil {
      return nil, err
    }
    if _, ok := seen[normalized]; ok {
      continue
    }
    seen[normalized] = struct{}{}
    out = append(out, normalized)
  }
  sort.Strings(out)
  return out, nil
}

func normalizeProtocol(raw string) (string, error) {
  switch strings.TrimSpace(strings.ToLower(raw)) {
  case "", "http":
    return "http", nil
  case "https", "tls":
    return "https", nil
  default:
    return "", errors.New("invalid protocol")
  }
}

func encryptTLSBundle(secret string, bundle tlsBundle) (enc, dek, nonce string, err error) {
  raw, err := json.Marshal(bundle)
  if err != nil {
    return "", "", "", err
  }
  masterKey := deriveMasterKey(secret)
  return encryptSecretEnvelope(masterKey, string(raw))
}

func decryptTLSBundle(secret string, cert models.TLSCertificate) (tlsBundle, error) {
  masterKey := deriveMasterKey(secret)
  plain, err := decryptSecretEnvelope(masterKey, cert.BundleEnc, cert.BundleDEK, cert.BundleNonce)
  if err != nil {
    return tlsBundle{}, err
  }
  var bundle tlsBundle
  if err := json.Unmarshal([]byte(plain), &bundle); err != nil {
    return tlsBundle{}, err
  }
  return bundle, nil
}

func encryptDNSCredentials(secret string, creds map[string]string) (enc, dek, nonce string, err error) {
  if len(creds) == 0 {
    return "", "", "", nil
  }
  raw, err := json.Marshal(creds)
  if err != nil {
    return "", "", "", err
  }
  masterKey := deriveMasterKey(secret)
  return encryptSecretEnvelope(masterKey, string(raw))
}

func decryptDNSCredentials(secret string, cert models.TLSCertificate) (map[string]string, error) {
  if strings.TrimSpace(cert.DNSCredentialsEnc) == "" {
    return map[string]string{}, nil
  }
  masterKey := deriveMasterKey(secret)
  plain, err := decryptSecretEnvelope(masterKey, cert.DNSCredentialsEnc, cert.DNSCredentialsDEK, cert.DNSCredentialsNonce)
  if err != nil {
    return nil, err
  }
  var creds map[string]string
  if err := json.Unmarshal([]byte(plain), &creds); err != nil {
    return nil, err
  }
  return creds, nil
}

func parseCertificateMetadata(certPEM string) (issuer, serial string, notBefore, notAfter time.Time, fingerprint string, err error) {
  block, _ := pem.Decode([]byte(certPEM))
  if block == nil {
    return "", "", time.Time{}, time.Time{}, "", errors.New("invalid certificate")
  }
  parsed, err := x509.ParseCertificate(block.Bytes)
  if err != nil {
    return "", "", time.Time{}, time.Time{}, "", err
  }
  issuer = parsed.Issuer.CommonName
  serial = parsed.SerialNumber.String()
  notBefore = parsed.NotBefore
  notAfter = parsed.NotAfter
  sum := sha256.Sum256(parsed.Raw)
  fingerprint = hex.EncodeToString(sum[:])
  return issuer, serial, notBefore, notAfter, fingerprint, nil
}

func scheduleRenewal(notAfter time.Time) time.Time {
  if notAfter.IsZero() {
    return time.Now().Add(24 * time.Hour)
  }
  target := notAfter.Add(-30 * 24 * time.Hour)
  if target.Before(time.Now().Add(2 * time.Hour)) {
    return time.Now().Add(2 * time.Hour)
  }
  return target
}

func primaryDomain(domains []string) string {
  if len(domains) == 0 {
    return ""
  }
  for _, d := range domains {
    if strings.HasPrefix(d, "*.") {
      continue
    }
    return d
  }
  return strings.TrimPrefix(domains[0], "*.")
}
