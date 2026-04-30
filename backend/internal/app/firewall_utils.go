package app

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"gorm.io/datatypes"
)

type firewallRuleInput struct {
  Provider    string `json:"provider"`
  Direction   string `json:"direction"`
  Action      string `json:"action"`
  Protocol    string `json:"protocol"`
  Port        string `json:"port"`
  Source      string `json:"source"`
  Destination string `json:"destination"`
  Comment     string `json:"comment"`
  Enabled     *bool  `json:"enabled"`
}

type firewallRulePayload struct {
  Direction   string `json:"direction"`
  Action      string `json:"action"`
  Protocol    string `json:"protocol"`
  Port        string `json:"port"`
  Source      string `json:"source"`
  Destination string `json:"destination"`
  Comment     string `json:"comment"`
  Enabled     bool   `json:"enabled"`
}

type firewallAuditSnapshot struct {
  Provider     string              `json:"provider"`
  System       json.RawMessage     `json:"system"`
  ManagedRules []models.FirewallRule `json:"managed_rules"`
}

func normalizeFirewallProvider(raw string) (string, error) {
  switch strings.TrimSpace(strings.ToLower(raw)) {
  case "", "ufw":
    return "ufw", nil
  case "iptables", "ipt":
    return "iptables", nil
  default:
    return "", errors.New("unsupported firewall provider")
  }
}

func normalizeFirewallDirection(raw string) (string, error) {
  switch strings.TrimSpace(strings.ToLower(raw)) {
  case "", "in", "ingress", "incoming":
    return "in", nil
  case "out", "egress", "outgoing":
    return "out", nil
  default:
    return "", errors.New("invalid direction")
  }
}

func normalizeFirewallAction(raw string) (string, error) {
  switch strings.TrimSpace(strings.ToLower(raw)) {
  case "allow", "accept":
    return "allow", nil
  case "deny", "drop":
    return "deny", nil
  case "reject":
    return "reject", nil
  case "limit", "rate":
    return "limit", nil
  default:
    return "", errors.New("invalid action")
  }
}

func normalizeFirewallProtocol(raw string) (string, error) {
  switch strings.TrimSpace(strings.ToLower(raw)) {
  case "", "any", "all":
    return "any", nil
  case "tcp":
    return "tcp", nil
  case "udp":
    return "udp", nil
  default:
    return "", errors.New("invalid protocol")
  }
}

func normalizeFirewallPort(raw string) (string, error) {
  port := strings.TrimSpace(raw)
  if port == "" {
    return "", nil
  }
  if strings.Contains(port, ",") {
    return "", errors.New("port list is not supported")
  }
  if strings.Contains(port, "-") {
    parts := strings.Split(port, "-")
    if len(parts) != 2 {
      return "", errors.New("invalid port range")
    }
    start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
    if err != nil || start <= 0 || start > 65535 {
      return "", errors.New("invalid port range")
    }
    end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
    if err != nil || end <= 0 || end > 65535 || end < start {
      return "", errors.New("invalid port range")
    }
    return strconv.Itoa(start) + "-" + strconv.Itoa(end), nil
  }
  value, err := strconv.Atoi(port)
  if err != nil || value <= 0 || value > 65535 {
    return "", errors.New("invalid port")
  }
  return strconv.Itoa(value), nil
}

func normalizeFirewallLocation(raw string) (string, error) {
  value := strings.TrimSpace(strings.ToLower(raw))
  if value == "" || value == "any" || value == "*" {
    return "any", nil
  }
  if strings.Contains(value, "/") {
    if _, _, err := net.ParseCIDR(value); err != nil {
      return "", errors.New("invalid cidr")
    }
    return value, nil
  }
  if ip := net.ParseIP(value); ip != nil {
    return value, nil
  }
  return "", errors.New("invalid address")
}

func validateFirewallRuleInput(input firewallRuleInput) (models.FirewallRule, error) {
  provider, err := normalizeFirewallProvider(input.Provider)
  if err != nil {
    return models.FirewallRule{}, err
  }
  direction, err := normalizeFirewallDirection(input.Direction)
  if err != nil {
    return models.FirewallRule{}, err
  }
  action, err := normalizeFirewallAction(input.Action)
  if err != nil {
    return models.FirewallRule{}, err
  }
  protocol, err := normalizeFirewallProtocol(input.Protocol)
  if err != nil {
    return models.FirewallRule{}, err
  }
  port, err := normalizeFirewallPort(input.Port)
  if err != nil {
    return models.FirewallRule{}, err
  }
  source, err := normalizeFirewallLocation(input.Source)
  if err != nil {
    return models.FirewallRule{}, err
  }
  destination, err := normalizeFirewallLocation(input.Destination)
  if err != nil {
    return models.FirewallRule{}, err
  }
  comment := strings.TrimSpace(input.Comment)
  if len(comment) > 255 {
    return models.FirewallRule{}, errors.New("comment is too long")
  }
  enabled := true
  if input.Enabled != nil {
    enabled = *input.Enabled
  }

  return models.FirewallRule{
    Provider:    provider,
    Direction:   direction,
    Action:      action,
    Protocol:    protocol,
    Port:        port,
    Source:      source,
    Destination: destination,
    Comment:     comment,
    Enabled:     enabled,
  }, nil
}

func firewallRuleKey(rule models.FirewallRule) string {
  return strings.Join([]string{
    rule.Provider,
    rule.Direction,
    rule.Action,
    rule.Protocol,
    rule.Port,
    rule.Source,
    rule.Destination,
  }, "|")
}

func buildFirewallRulePayload(rule models.FirewallRule) firewallRulePayload {
  return firewallRulePayload{
    Direction:   rule.Direction,
    Action:      rule.Action,
    Protocol:    rule.Protocol,
    Port:        rule.Port,
    Source:      rule.Source,
    Destination: rule.Destination,
    Comment:     rule.Comment,
    Enabled:     rule.Enabled,
  }
}

func marshalFirewallSnapshot(provider string, systemSnapshot json.RawMessage, managed []models.FirewallRule) datatypes.JSON {
  snapshot := firewallAuditSnapshot{
    Provider:     provider,
    System:       systemSnapshot,
    ManagedRules: managed,
  }
  raw, _ := json.Marshal(snapshot)
  if len(raw) == 0 {
    raw = []byte("{}")
  }
  return datatypes.JSON(raw)
}

func (a *App) logFirewallAudit(userID, serverID uint, ruleID *uint, provider, action string, success bool, message string, snapshot datatypes.JSON) {
  entry := models.FirewallAuditLog{
    UserID:    userID,
    ServerID:  serverID,
    RuleID:    ruleID,
    Provider:  provider,
    Action:    strings.TrimSpace(action),
    Success:   success,
    Message:   strings.TrimSpace(message),
    Snapshot:  snapshot,
    CreatedAt: time.Now().UTC(),
  }
  _ = a.db.Create(&entry).Error
}
