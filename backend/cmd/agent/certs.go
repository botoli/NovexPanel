package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a *Agent) issueCertificate(payload certIssuePayload) (map[string]any, error) {
  domains := normalizeCertDomains(payload.Domains)
  if len(domains) == 0 {
    return nil, errors.New("domains are required")
  }
  challenge := strings.TrimSpace(strings.ToLower(payload.Challenge))
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
    return nil, errors.New("wildcard requires dns challenge")
  }

  certName := sanitizeCertName(primaryDomain(domains))
  if certName == "" {
    certName = "novex-cert"
  }

  switch challenge {
  case "http":
    return issueViaCertbot(domains, certName, payload.Email, payload.Force)
  case "dns":
    if strings.TrimSpace(payload.DNSProvider) == "" {
      return nil, errors.New("dns_provider is required")
    }
    return issueViaAcmeSh(domains, certName, payload)
  default:
    return nil, errors.New("unsupported challenge")
  }
}

func issueViaCertbot(domains []string, certName, email string, force bool) (map[string]any, error) {
  args := []string{"certonly", "--standalone", "--non-interactive", "--agree-tos", "--preferred-challenges", "http"}
  if strings.TrimSpace(email) == "" {
    args = append(args, "--register-unsafely-without-email")
  } else {
    args = append(args, "--email", email)
  }
  if force {
    args = append(args, "--force-renewal")
  } else {
    args = append(args, "--keep-until-expiring")
  }
  args = append(args, "--cert-name", certName)
  for _, domain := range domains {
    args = append(args, "-d", domain)
  }

  cmd := "sudo certbot " + strings.Join(args, " ")
  stdout, stderr, exitCode, err := runShellCommand(cmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }

  base := filepath.Join("/etc/letsencrypt/live", certName)
  certPem, err := os.ReadFile(filepath.Join(base, "fullchain.pem"))
  if err != nil {
    return nil, err
  }
  keyPem, err := os.ReadFile(filepath.Join(base, "privkey.pem"))
  if err != nil {
    return nil, err
  }
  chainPem, _ := os.ReadFile(filepath.Join(base, "chain.pem"))

  return map[string]any{
    "cert_pem":  string(certPem),
    "key_pem":   string(keyPem),
    "chain_pem": string(chainPem),
    "cert_name": certName,
  }, nil
}

func issueViaAcmeSh(domains []string, certName string, payload certIssuePayload) (map[string]any, error) {
  acme := findAcmeSh()
  if acme == "" {
    return nil, errors.New("acme.sh not found")
  }
  envPrefix := buildEnvPrefix(payload.DNSCredentials)

  issueArgs := []string{acme, "--issue", "--dns", payload.DNSProvider}
  if payload.Force {
    issueArgs = append(issueArgs, "--force")
  }
  for _, d := range domains {
    issueArgs = append(issueArgs, "-d", d)
  }
  issueCmd := strings.TrimSpace(envPrefix + " " + strings.Join(issueArgs, " "))
  stdout, stderr, exitCode, err := runShellCommand(issueCmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }

  certDir := filepath.Join("/etc/novex/certs", certName)
  if err := os.MkdirAll(certDir, 0o700); err != nil {
    return nil, err
  }
  keyPath := filepath.Join(certDir, "privkey.pem")
  fullPath := filepath.Join(certDir, "fullchain.pem")
  installArgs := []string{acme, "--install-cert", "-d", domains[0], "--key-file", keyPath, "--fullchain-file", fullPath, "--reloadcmd", "true"}
  installCmd := strings.TrimSpace(envPrefix + " " + strings.Join(installArgs, " "))
  stdout, stderr, exitCode, err = runShellCommand(installCmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }

  certPem, err := os.ReadFile(fullPath)
  if err != nil {
    return nil, err
  }
  keyPem, err := os.ReadFile(keyPath)
  if err != nil {
    return nil, err
  }
  chainPem, _ := os.ReadFile(filepath.Join(certDir, "chain.pem"))

  return map[string]any{
    "cert_pem":  string(certPem),
    "key_pem":   string(keyPem),
    "chain_pem": string(chainPem),
    "cert_name": certName,
  }, nil
}

func (a *Agent) installCertificate(payload certInstallPayload) (map[string]any, error) {
  certName := sanitizeCertName(payload.CertName)
  if certName == "" {
    return nil, errors.New("cert_name is required")
  }
  base := filepath.Join("/etc/novex/certs", certName)
  if err := os.MkdirAll(base, 0o700); err != nil {
    return nil, err
  }
  if err := os.WriteFile(filepath.Join(base, "fullchain.pem"), []byte(payload.CertPEM), 0o600); err != nil {
    return nil, err
  }
  if err := os.WriteFile(filepath.Join(base, "privkey.pem"), []byte(payload.KeyPEM), 0o600); err != nil {
    return nil, err
  }
  if strings.TrimSpace(payload.ChainPEM) != "" {
    _ = os.WriteFile(filepath.Join(base, "chain.pem"), []byte(payload.ChainPEM), 0o600)
  }
  return map[string]any{"ok": true, "path": base}, nil
}

func normalizeCertDomains(domains []string) []string {
  out := make([]string, 0, len(domains))
  seen := make(map[string]struct{}, len(domains))
  for _, d := range domains {
    d = strings.TrimSpace(strings.ToLower(d))
    d = strings.TrimSuffix(d, ".")
    if d == "" {
      continue
    }
    if _, ok := seen[d]; ok {
      continue
    }
    seen[d] = struct{}{}
    out = append(out, d)
  }
  return out
}

func primaryDomain(domains []string) string {
  if len(domains) == 0 {
    return ""
  }
  for _, d := range domains {
    if strings.HasPrefix(d, "*.") {
      continue
    }
    return strings.TrimPrefix(d, "*.")
  }
  return strings.TrimPrefix(domains[0], "*.")
}

func sanitizeCertName(domain string) string {
  name := strings.TrimSpace(strings.ToLower(domain))
  name = strings.TrimPrefix(name, "*.")
  name = strings.ReplaceAll(name, "*", "")
  name = strings.ReplaceAll(name, ".", "-")
  if name == "" {
    return ""
  }
  return name
}

func findAcmeSh() string {
  if path, err := exec.LookPath("acme.sh"); err == nil {
    return path
  }
  home, err := os.UserHomeDir()
  if err != nil {
    return ""
  }
  candidate := filepath.Join(home, ".acme.sh", "acme.sh")
  if _, err := os.Stat(candidate); err == nil {
    return candidate
  }
  return ""
}

func buildEnvPrefix(env map[string]string) string {
  if len(env) == 0 {
    return ""
  }
  parts := make([]string, 0, len(env))
  for key, value := range env {
    if strings.TrimSpace(key) == "" {
      continue
    }
    parts = append(parts, fmt.Sprintf("%s=%s", key, shellQuote(value)))
  }
  return strings.Join(parts, " ")
}

func shellQuote(value string) string {
  if value == "" {
    return "''"
  }
  if !strings.ContainsAny(value, " '"+"\""+"$") {
    return value
  }
  return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
