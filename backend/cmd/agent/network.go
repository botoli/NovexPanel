package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

func (a *Agent) listOpenPorts() ([]map[string]any, error) {
  stdout, _, _, err := runShellCommand("ss -tulnp")
  if err != nil {
    stdout, _, _, err = runShellCommand("ss -tuln")
    if err != nil {
      return nil, err
    }
  }
  lines := strings.Split(strings.TrimSpace(stdout), "\n")
  items := make([]map[string]any, 0, len(lines))
  for _, line := range lines {
    line = strings.TrimSpace(line)
    if line == "" || strings.HasPrefix(line, "Netid") {
      continue
    }
    fields := strings.Fields(line)
    if len(fields) < 5 {
      continue
    }
    protocol := strings.TrimSpace(fields[0])
    state := strings.TrimSpace(fields[1])
    localField := fields[4]
    addr, port := splitAddressPort(localField)
    procName, procPID := parseProcessInfo(line)
    item := map[string]any{
      "protocol":   protocol,
      "state":      state,
      "local_addr": addr,
      "port":       port,
    }
    if procName != "" {
      item["process"] = fmt.Sprintf("%s (%s)", procName, procPID)
    }
    items = append(items, item)
  }
  return items, nil
}

func (a *Agent) listNetworkConnections(limit int) ([]map[string]any, error) {
  stdout, _, _, err := runShellCommand("ss -tnp state established")
  if err != nil {
    return nil, err
  }
  lines := strings.Split(strings.TrimSpace(stdout), "\n")
  out := make([]map[string]any, 0, len(lines))
  domainCache := make(map[string]string)
  for _, line := range lines {
    line = strings.TrimSpace(line)
    if line == "" || strings.HasPrefix(line, "State") {
      continue
    }
    fields := strings.Fields(line)
    if len(fields) < 5 {
      continue
    }
    state := fields[0]
    local := fields[3]
    remote := fields[4]
    localAddr, localPort := splitAddressPort(local)
    remoteAddr, remotePort := splitAddressPort(remote)
    processName, processPID := parseProcessInfo(line)
    processValue := ""
    if processName != "" {
      processValue = fmt.Sprintf("%s (%s)", processName, processPID)
    }
    domainValue := ""
    if remoteAddr != "" {
      domainValue = reverseLookup(remoteAddr, domainCache)
    }
    out = append(out, map[string]any{
      "protocol":    "tcp",
      "state":       state,
      "local_addr":  localAddr,
      "local_port":  localPort,
      "remote_ip":   remoteAddr,
      "remote_port": remotePort,
      "process":     processValue,
      "domain":      domainValue,
    })
    if limit > 0 && len(out) >= limit {
      break
    }
  }
  return out, nil
}

func reverseLookup(ip string, cache map[string]string) string {
  if ip == "" {
    return ""
  }
  if value, ok := cache[ip]; ok {
    return value
  }
  parsed := net.ParseIP(ip)
  if parsed == nil || parsed.IsPrivate() || parsed.IsLoopback() {
    cache[ip] = ""
    return ""
  }
  ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
  defer cancel()
  names, err := net.DefaultResolver.LookupAddr(ctx, ip)
  if err != nil || len(names) == 0 {
    cache[ip] = ""
    return ""
  }
  host := strings.TrimSuffix(strings.TrimSpace(names[0]), ".")
  cache[ip] = host
  return host
}

func splitAddressPort(raw string) (string, string) {
  raw = strings.TrimSpace(raw)
  if raw == "" {
    return "", ""
  }
  if strings.HasPrefix(raw, "[") {
    if host, port, err := net.SplitHostPort(raw); err == nil {
      host = strings.TrimPrefix(host, "[")
      host = strings.TrimSuffix(host, "]")
      return host, port
    }
  }
  if strings.Count(raw, ":") > 1 && !strings.Contains(raw, "]") {
    // Fallback for IPv6 without brackets.
    idx := strings.LastIndex(raw, ":")
    if idx > 0 {
      return raw[:idx], raw[idx+1:]
    }
  }
  idx := strings.LastIndex(raw, ":")
  if idx < 0 {
    return raw, ""
  }
  return raw[:idx], raw[idx+1:]
}

func parseProcessInfo(line string) (string, string) {
  idx := strings.Index(line, "users:")
  if idx == -1 {
    return "", ""
  }
  segment := line[idx:]
  name := ""
  pid := ""
  start := strings.Index(segment, "\"")
  if start >= 0 {
    rest := segment[start+1:]
    end := strings.Index(rest, "\"")
    if end >= 0 {
      name = rest[:end]
    }
  }
  if pidIdx := strings.Index(segment, "pid="); pidIdx >= 0 {
    rest := segment[pidIdx+4:]
    end := strings.IndexAny(rest, ",)")
    if end >= 0 {
      pid = rest[:end]
    } else {
      pid = rest
    }
  }
  if pid == "" {
    pid = "pid?"
  }
  return name, pid
}
