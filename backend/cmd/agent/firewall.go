package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ufwSnapshot struct {
  Provider   string         `json:"provider"`
  Enabled    bool           `json:"enabled"`
  DefaultIn  string         `json:"default_in"`
  DefaultOut string         `json:"default_out"`
  Rules      []firewallRule `json:"rules"`
}

type iptablesSnapshot struct {
  Provider string `json:"provider"`
  Raw      string `json:"raw"`
  Raw6     string `json:"raw6"`
}

func (a *Agent) firewallList(providerRaw string) (map[string]any, error) {
  provider, err := normalizeFirewallProvider(providerRaw)
  if err != nil {
    return nil, err
  }
  switch provider {
  case "ufw":
    enabled, defIn, defOut, err := ufwStatus()
    if err != nil {
      return nil, err
    }
    rules, err := ufwRules()
    if err != nil {
      return nil, err
    }
    return map[string]any{
      "provider":    provider,
      "enabled":     enabled,
      "default_in":  defIn,
      "default_out": defOut,
      "rules":       rules,
    }, nil
  case "iptables":
    rules, defIn, defOut, err := iptablesRules()
    if err != nil {
      return nil, err
    }
    return map[string]any{
      "provider":    provider,
      "default_in":  defIn,
      "default_out": defOut,
      "rules":       rules,
    }, nil
  default:
    return nil, fmt.Errorf("unsupported provider")
  }
}

func (a *Agent) firewallSnapshot(providerRaw string) (map[string]any, error) {
  provider, err := normalizeFirewallProvider(providerRaw)
  if err != nil {
    return nil, err
  }
  switch provider {
  case "ufw":
    enabled, defIn, defOut, err := ufwStatus()
    if err != nil {
      return nil, err
    }
    rules, err := ufwRules()
    if err != nil {
      return nil, err
    }
    return map[string]any{
      "provider":    provider,
      "enabled":     enabled,
      "default_in":  defIn,
      "default_out": defOut,
      "rules":       rules,
    }, nil
  case "iptables":
    raw, _, _, err := runShellCommand("sudo iptables-save")
    if err != nil {
      return nil, err
    }
    raw6, _, _, _ := runShellCommand("sudo ip6tables-save")
    return map[string]any{
      "provider": provider,
      "raw":      raw,
      "raw6":     raw6,
    }, nil
  default:
    return nil, fmt.Errorf("unsupported provider")
  }
}

func (a *Agent) firewallApply(providerRaw, operation string, rule firewallRule, prev *firewallRule) (map[string]any, error) {
  provider, err := normalizeFirewallProvider(providerRaw)
  if err != nil {
    return nil, err
  }
  operation = strings.TrimSpace(strings.ToLower(operation))
  if operation == "" {
    operation = "add"
  }

  switch provider {
  case "ufw":
    if operation == "add" {
      return applyUfwRule(rule)
    }
    if operation == "delete" {
      return deleteUfwRule(rule)
    }
    if operation == "update" {
      if prev != nil {
        _, _ = deleteUfwRule(*prev)
      }
      return applyUfwRule(rule)
    }
    return map[string]any{"ok": true}, nil
  case "iptables":
    if operation == "add" {
      return applyIptablesRule(rule)
    }
    if operation == "delete" {
      return deleteIptablesRule(rule)
    }
    if operation == "update" {
      if prev != nil {
        _, _ = deleteIptablesRule(*prev)
      }
      return applyIptablesRule(rule)
    }
    return map[string]any{"ok": true}, nil
  default:
    return nil, fmt.Errorf("unsupported provider")
  }
}

func (a *Agent) firewallRollback(providerRaw string, snapshotRaw json.RawMessage) (map[string]any, error) {
  provider, err := normalizeFirewallProvider(providerRaw)
  if err != nil {
    return nil, err
  }
  switch provider {
  case "ufw":
    var snap ufwSnapshot
    if err := json.Unmarshal(snapshotRaw, &snap); err != nil {
      return nil, fmt.Errorf("invalid snapshot")
    }
    if err := restoreUfwSnapshot(snap); err != nil {
      return nil, err
    }
    return map[string]any{"ok": true}, nil
  case "iptables":
    var snap iptablesSnapshot
    if err := json.Unmarshal(snapshotRaw, &snap); err != nil {
      return nil, fmt.Errorf("invalid snapshot")
    }
    if err := restoreIptablesSnapshot(snap); err != nil {
      return nil, err
    }
    return map[string]any{"ok": true}, nil
  default:
    return nil, fmt.Errorf("unsupported provider")
  }
}

func normalizeFirewallProvider(raw string) (string, error) {
  switch strings.TrimSpace(strings.ToLower(raw)) {
  case "", "ufw":
    return "ufw", nil
  case "iptables", "ipt":
    return "iptables", nil
  default:
    return "", fmt.Errorf("unsupported provider")
  }
}

func normalizeFirewallRule(rule firewallRule) firewallRule {
  rule.Direction = strings.TrimSpace(strings.ToLower(rule.Direction))
  if rule.Direction == "" {
    rule.Direction = "in"
  }
  rule.Action = strings.TrimSpace(strings.ToLower(rule.Action))
  rule.Protocol = strings.TrimSpace(strings.ToLower(rule.Protocol))
  if rule.Protocol == "" {
    rule.Protocol = "any"
  }
  rule.Port = strings.TrimSpace(rule.Port)
  rule.Source = strings.TrimSpace(rule.Source)
  if rule.Source == "" {
    rule.Source = "any"
  }
  rule.Destination = strings.TrimSpace(rule.Destination)
  if rule.Destination == "" {
    rule.Destination = "any"
  }
  return rule
}

func ufwStatus() (bool, string, string, error) {
  stdout, _, _, err := runShellCommand("sudo ufw status verbose")
  if err != nil {
    return false, "", "", err
  }
  enabled := strings.Contains(strings.ToLower(stdout), "status: active")
  defIn := "deny"
  defOut := "allow"
  lines := strings.Split(stdout, "\n")
  for _, line := range lines {
    lower := strings.ToLower(strings.TrimSpace(line))
    if strings.HasPrefix(lower, "default:") {
      if strings.Contains(lower, "incoming") {
        if strings.Contains(lower, "allow") {
          defIn = "allow"
        } else if strings.Contains(lower, "reject") {
          defIn = "reject"
        } else {
          defIn = "deny"
        }
      }
      if strings.Contains(lower, "outgoing") {
        if strings.Contains(lower, "allow") {
          defOut = "allow"
        } else if strings.Contains(lower, "reject") {
          defOut = "reject"
        } else {
          defOut = "deny"
        }
      }
    }
  }
  return enabled, defIn, defOut, nil
}

func ufwRules() ([]firewallRule, error) {
  stdout, _, _, err := runShellCommand("sudo ufw status")
  if err != nil {
    return nil, err
  }
  return parseUfwRules(stdout), nil
}

func parseUfwRules(output string) []firewallRule {
  lines := strings.Split(strings.TrimSpace(output), "\n")
  out := make([]firewallRule, 0)
  for _, line := range lines {
    line = strings.TrimSpace(line)
    if line == "" || strings.HasPrefix(strings.ToLower(line), "status") {
      continue
    }
    if strings.HasPrefix(strings.ToLower(line), "to ") {
      continue
    }
    fields := strings.Fields(line)
    if len(fields) < 3 {
      continue
    }
    actionIdx := -1
    for idx, field := range fields {
      upper := strings.ToUpper(field)
      if upper == "ALLOW" || upper == "DENY" || upper == "REJECT" || upper == "LIMIT" {
        actionIdx = idx
        break
      }
    }
    if actionIdx == -1 {
      continue
    }
    toField := strings.Join(fields[:actionIdx], " ")
    action := strings.ToLower(fields[actionIdx])
    direction := "in"
    fromStart := actionIdx + 1
    if fromStart < len(fields) && (strings.EqualFold(fields[fromStart], "IN") || strings.EqualFold(fields[fromStart], "OUT")) {
      direction = strings.ToLower(fields[fromStart])
      fromStart++
    }
    fromField := strings.Join(fields[fromStart:], " ")
    fromField = strings.ReplaceAll(fromField, "(v6)", "")
    fromField = strings.TrimSpace(fromField)
    if strings.EqualFold(fromField, "anywhere") || fromField == "" {
      fromField = "any"
    }

    port := ""
    protocol := "any"
    toField = strings.ReplaceAll(toField, "(v6)", "")
    if strings.Contains(toField, "/") {
      parts := strings.Split(toField, "/")
      if len(parts) == 2 {
        port = strings.TrimSpace(parts[0])
        protocol = strings.TrimSpace(parts[1])
      }
    } else if !strings.EqualFold(toField, "anywhere") {
      port = strings.TrimSpace(toField)
    }

    out = append(out, firewallRule{
      Direction:   direction,
      Action:      action,
      Protocol:    protocol,
      Port:        port,
      Source:      fromField,
      Destination: "any",
      Enabled:     true,
    })
  }
  return out
}

func applyUfwRule(rule firewallRule) (map[string]any, error) {
  rule = normalizeFirewallRule(rule)
  cmd := buildUfwCommand(rule)
  stdout, stderr, exitCode, err := runShellCommand(cmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }
  return map[string]any{"ok": true, "output": strings.TrimSpace(stdout)}, nil
}

func deleteUfwRule(rule firewallRule) (map[string]any, error) {
  rule = normalizeFirewallRule(rule)
  cmd := buildUfwCommand(rule)
  cmd = strings.Replace(cmd, "ufw --force ", "ufw --force delete ", 1)
  stdout, stderr, exitCode, err := runShellCommand(cmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }
  return map[string]any{"ok": true, "output": strings.TrimSpace(stdout)}, nil
}

func buildUfwCommand(rule firewallRule) string {
  base := "sudo ufw --force " + rule.Action
  if rule.Direction == "out" {
    base += " out"
  } else {
    base += " in"
  }
  if rule.Source == "" || rule.Source == "any" {
    base += " from any"
  } else {
    base += " from " + rule.Source
  }
  if rule.Destination == "" || rule.Destination == "any" {
    base += " to any"
  } else {
    base += " to " + rule.Destination
  }
  if rule.Port != "" {
    base += " port " + rule.Port
  }
  if rule.Protocol != "" && rule.Protocol != "any" {
    base += " proto " + rule.Protocol
  }
  return base
}

func iptablesRules() ([]firewallRule, string, string, error) {
  stdout, _, _, err := runShellCommand("sudo iptables -S")
  if err != nil {
    return nil, "", "", err
  }
  rules, defIn, defOut := parseIptablesRules(stdout)
  return rules, defIn, defOut, nil
}

func parseIptablesRules(output string) ([]firewallRule, string, string) {
  lines := strings.Split(strings.TrimSpace(output), "\n")
  out := make([]firewallRule, 0)
  defIn := "accept"
  defOut := "accept"

  for _, line := range lines {
    line = strings.TrimSpace(line)
    if line == "" {
      continue
    }
    fields := strings.Fields(line)
    if len(fields) < 2 {
      continue
    }
    if fields[0] == "-P" && len(fields) >= 3 {
      chain := fields[1]
      policy := strings.ToLower(fields[2])
      if chain == "INPUT" {
        defIn = policy
      }
      if chain == "OUTPUT" {
        defOut = policy
      }
      continue
    }
    if fields[0] != "-A" || len(fields) < 3 {
      continue
    }
    chain := fields[1]
    direction := ""
    switch chain {
    case "INPUT":
      direction = "in"
    case "OUTPUT":
      direction = "out"
    default:
      continue
    }

    rule := firewallRule{Direction: direction, Protocol: "any", Source: "any", Destination: "any", Enabled: true}
    hasLimit := false

    for idx := 2; idx < len(fields); idx++ {
      switch fields[idx] {
      case "-p":
        if idx+1 < len(fields) {
          rule.Protocol = strings.ToLower(fields[idx+1])
          idx++
        }
      case "-s":
        if idx+1 < len(fields) {
          rule.Source = fields[idx+1]
          idx++
        }
      case "-d":
        if idx+1 < len(fields) {
          rule.Destination = fields[idx+1]
          idx++
        }
      case "--dport":
        if idx+1 < len(fields) {
          rule.Port = strings.ReplaceAll(fields[idx+1], ":", "-")
          idx++
        }
      case "-j":
        if idx+1 < len(fields) {
          jump := strings.ToUpper(fields[idx+1])
          switch jump {
          case "ACCEPT":
            rule.Action = "allow"
          case "DROP":
            rule.Action = "deny"
          case "REJECT":
            rule.Action = "reject"
          default:
            rule.Action = strings.ToLower(jump)
          }
          idx++
        }
      case "-m":
        if idx+1 < len(fields) && fields[idx+1] == "limit" {
          hasLimit = true
          idx++
        }
      }
    }
    if rule.Action == "" {
      rule.Action = "allow"
    }
    if hasLimit && rule.Action == "allow" {
      rule.Action = "limit"
    }
    out = append(out, rule)
  }

  return out, defIn, defOut
}

func applyIptablesRule(rule firewallRule) (map[string]any, error) {
  rule = normalizeFirewallRule(rule)
  cmd, err := buildIptablesCommand("-A", rule)
  if err != nil {
    return nil, err
  }
  stdout, stderr, exitCode, err := runShellCommand(cmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }
  return map[string]any{"ok": true, "output": strings.TrimSpace(stdout)}, nil
}

func deleteIptablesRule(rule firewallRule) (map[string]any, error) {
  rule = normalizeFirewallRule(rule)
  cmd, err := buildIptablesCommand("-D", rule)
  if err != nil {
    return nil, err
  }
  stdout, stderr, exitCode, err := runShellCommand(cmd)
  if err != nil || exitCode != 0 {
    return nil, errors.New(strings.TrimSpace(stderr + "\n" + stdout))
  }
  return map[string]any{"ok": true, "output": strings.TrimSpace(stdout)}, nil
}

func buildIptablesCommand(action string, rule firewallRule) (string, error) {
  chain := "INPUT"
  if rule.Direction == "out" {
    chain = "OUTPUT"
  }
  proto := rule.Protocol
  if proto == "" || proto == "any" {
    proto = ""
  }
  port := strings.TrimSpace(rule.Port)
  if port != "" && proto == "" {
    proto = "tcp"
  }
  target := "ACCEPT"
  switch rule.Action {
  case "allow":
    target = "ACCEPT"
  case "deny":
    target = "DROP"
  case "reject":
    target = "REJECT"
  case "limit":
    target = "ACCEPT"
  default:
    target = "ACCEPT"
  }

  parts := []string{"sudo", "iptables", action, chain}
  if proto != "" {
    parts = append(parts, "-p", proto)
  }
  if rule.Source != "" && rule.Source != "any" {
    parts = append(parts, "-s", rule.Source)
  }
  if rule.Destination != "" && rule.Destination != "any" {
    parts = append(parts, "-d", rule.Destination)
  }
  if port != "" {
    port = strings.ReplaceAll(port, "-", ":")
    parts = append(parts, "--dport", port)
  }
  if rule.Action == "limit" {
    parts = append(parts, "-m", "limit", "--limit", "25/min", "--limit-burst", "100")
  }
  parts = append(parts, "-j", target)
  return strings.Join(parts, " "), nil
}

func restoreUfwSnapshot(snapshot ufwSnapshot) error {
  _, stderr, exitCode, err := runShellCommand("sudo ufw --force reset")
  if err != nil || exitCode != 0 {
    return fmt.Errorf(strings.TrimSpace(stderr))
  }
  if snapshot.DefaultIn != "" {
    _, stderr, exitCode, err = runShellCommand("sudo ufw default " + snapshot.DefaultIn + " incoming")
    if err != nil || exitCode != 0 {
      return fmt.Errorf(strings.TrimSpace(stderr))
    }
  }
  if snapshot.DefaultOut != "" {
    _, stderr, exitCode, err = runShellCommand("sudo ufw default " + snapshot.DefaultOut + " outgoing")
    if err != nil || exitCode != 0 {
      return fmt.Errorf(strings.TrimSpace(stderr))
    }
  }
  for _, rule := range snapshot.Rules {
    if _, err := applyUfwRule(rule); err != nil {
      return err
    }
  }
  if snapshot.Enabled {
    _, _, _, _ = runShellCommand("sudo ufw --force enable")
  } else {
    _, _, _, _ = runShellCommand("sudo ufw disable")
  }
  return nil
}

func restoreIptablesSnapshot(snapshot iptablesSnapshot) error {
  raw := strings.TrimSpace(snapshot.Raw)
  if raw == "" {
    return fmt.Errorf("empty iptables snapshot")
  }
  if err := runRestoreCommand("sudo iptables-restore", raw); err != nil {
    return err
  }
  if strings.TrimSpace(snapshot.Raw6) != "" {
    _ = runRestoreCommand("sudo ip6tables-restore", snapshot.Raw6)
  }
  return nil
}

func runRestoreCommand(cmd, payload string) error {
  tmpDir := os.TempDir()
  file := filepath.Join(tmpDir, fmt.Sprintf("novex-iptables-%d.rules", time.Now().UnixNano()))
  if err := os.WriteFile(file, []byte(payload), 0o600); err != nil {
    return err
  }
  defer os.Remove(file)
  stdout, stderr, exitCode, err := runShellCommand(cmd + " < " + file)
  if err != nil || exitCode != 0 {
    return fmt.Errorf(strings.TrimSpace(stderr + "\n" + stdout))
  }
  return nil
}
