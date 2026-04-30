package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"novexpanel/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type firewallQuickRuleRequest struct {
	Provider  string `json:"provider"`
	Action    string `json:"action"`
	Protocol  string `json:"protocol"`
	Port      string `json:"port"`
	Source    string `json:"source"`
	Comment   string `json:"comment"`
	Direction string `json:"direction"`
}

type firewallApplyRequest struct {
	Provider  string               `json:"provider"`
	Operation string               `json:"operation"`
	Rule      firewallRulePayload  `json:"rule"`
	Previous  *firewallRulePayload `json:"previous_rule"`
}

func (a *App) handleNetworkOpenPorts(c *gin.Context) {
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

	raw, err := a.hub.RequestAgent(serverID, "list_open_ports", nil, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	payload, ok := decoded.(map[string]any)
	if !ok {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	rawPorts, _ := payload["ports"].([]any)
	normalized := make([]gin.H, 0, len(rawPorts))
	for _, item := range rawPorts {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		normalized = append(normalized, gin.H{
			"port":    valueToString(m["port"]),
			"proto":   valueToString(m["protocol"]),
			"process": valueToString(m["process"]),
			"state":   valueToString(m["state"]),
			"address": valueToString(m["local_addr"]),
		})
	}
	c.JSON(http.StatusOK, gin.H{"ports": normalized})
}

func (a *App) handleNetworkConnections(c *gin.Context) {
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

	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, parseErr := parsePositiveInt(raw); parseErr == nil && parsed > 0 {
			limit = int(parsed)
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	raw, err := a.hub.RequestAgent(serverID, "network_connections", map[string]any{"limit": limit}, 25*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}
	c.JSON(http.StatusOK, decoded)
}

func (a *App) handleNetworkTraffic(c *gin.Context) {
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

	interval, err := parseMetricsInterval(c.Query("interval"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval"})
		return
	}
	rangeDuration, err := parseMetricsRange(c.Query("range"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range"})
		return
	}

	now := time.Now()
	since := now.Add(-rangeDuration)
	points, err := a.loadMetricsHistory(serverID, since, now, interval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load traffic"})
		return
	}

	spikes, summary := detectTrafficSpikes(points)
	formatted := make([]gin.H, 0, len(points))
	for _, point := range points {
		formatted = append(formatted, gin.H{
			"timestamp": point.Timestamp,
			"rx":        point.NetworkRX,
			"tx":        point.NetworkTX,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"points":  formatted,
		"spikes":  spikes,
		"summary": summary,
	})
}

func detectTrafficSpikes(points []metricsHistoryPoint) ([]gin.H, gin.H) {
	if len(points) == 0 {
		return []gin.H{}, gin.H{"avg_rx": 0, "avg_tx": 0, "max_rx": 0, "max_tx": 0}
	}
	values := make([]float64, 0, len(points))
	maxRX := 0.0
	maxTX := 0.0
	sumRX := 0.0
	sumTX := 0.0
	for _, p := range points {
		values = append(values, p.NetworkRX)
		sumRX += p.NetworkRX
		sumTX += p.NetworkTX
		if p.NetworkRX > maxRX {
			maxRX = p.NetworkRX
		}
		if p.NetworkTX > maxTX {
			maxTX = p.NetworkTX
		}
	}
	mean := sumRX / float64(len(points))
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(points))
	std := math.Sqrt(variance)
	threshold := math.Max(mean*2, mean+3*std)

	spikes := make([]gin.H, 0)
	for _, p := range points {
		if p.NetworkRX >= threshold && p.NetworkRX > 0 {
			spikes = append(spikes, gin.H{
				"timestamp": p.Timestamp,
				"rx":        p.NetworkRX,
				"tx":        p.NetworkTX,
			})
		}
	}

	summary := gin.H{
		"avg_rx": mean,
		"avg_tx": sumTX / float64(len(points)),
		"max_rx": maxRX,
		"max_tx": maxTX,
	}
	return spikes, summary
}

func (a *App) handleNetworkAlerts(c *gin.Context) {
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

	interval, _ := parseMetricsInterval(c.Query("interval"))
	if interval == "" {
		interval = "1m"
	}
	rangeDuration, _ := parseMetricsRange(c.Query("range"))
	if rangeDuration == 0 {
		rangeDuration = 1 * time.Hour
	}

	now := time.Now()
	since := now.Add(-rangeDuration)
	points, err := a.loadMetricsHistory(serverID, since, now, interval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load metrics"})
		return
	}
	spikes, _ := detectTrafficSpikes(points)

	raw, err := a.hub.RequestAgent(serverID, "network_connections", map[string]any{"limit": 400}, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}

	connections := parseConnectionList(decoded)
	suspicious := detectSuspiciousConnections(connections)

	c.JSON(http.StatusOK, gin.H{
		"spikes":      spikes,
		"suspicious":  suspicious,
		"connections": connections,
	})
}

type networkConn struct {
	RemoteIP   string `json:"remote_ip"`
	RemotePort string `json:"remote_port"`
	Domain     string `json:"domain"`
	Process    string `json:"process"`
	LocalAddr  string `json:"local_addr"`
	LocalPort  string `json:"local_port"`
	State      string `json:"state"`
	Protocol   string `json:"protocol"`
}

func parseConnectionList(decoded any) []networkConn {
	list := make([]networkConn, 0)
	payload, ok := decoded.(map[string]any)
	if !ok {
		return list
	}
	rawItems, ok := payload["connections"].([]any)
	if !ok {
		return list
	}
	for _, item := range rawItems {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		conn := networkConn{
			RemoteIP:   strings.TrimSpace(valueToString(m["remote_ip"])),
			RemotePort: strings.TrimSpace(valueToString(m["remote_port"])),
			Domain:     strings.TrimSpace(valueToString(m["domain"])),
			Process:    strings.TrimSpace(valueToString(m["process"])),
			LocalAddr:  strings.TrimSpace(valueToString(m["local_addr"])),
			LocalPort:  strings.TrimSpace(valueToString(m["local_port"])),
			State:      strings.TrimSpace(valueToString(m["state"])),
			Protocol:   strings.TrimSpace(valueToString(m["protocol"])),
		}
		if conn.RemoteIP == "" {
			continue
		}
		list = append(list, conn)
	}
	return list
}

func valueToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func detectSuspiciousConnections(conns []networkConn) []gin.H {
	counts := make(map[string]int)
	for _, conn := range conns {
		counts[conn.RemoteIP]++
	}
	type kv struct {
		IP    string
		Count int
	}
	items := make([]kv, 0, len(counts))
	for ip, count := range counts {
		if count >= 40 {
			items = append(items, kv{IP: ip, Count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Count > items[j].Count })
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{"ip": item.IP, "connections": item.Count})
	}
	return out
}

func (a *App) handleFirewallList(c *gin.Context) {
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

	provider, err := normalizeFirewallProvider(c.Query("provider"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var managed []models.FirewallRule
	if err := a.db.Where("user_id = ? AND server_id = ? AND provider = ?", userID, serverID, provider).Order("id desc").Find(&managed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load rules"})
		return
	}

	raw, err := a.hub.RequestAgent(serverID, "firewall_list", map[string]any{"provider": provider}, 20*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	decoded, err := decodeRawJSON(raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":        provider,
		"managed_rules":   managed,
		"system_snapshot": decoded,
	})
}

func (a *App) handleFirewallCreateRule(c *gin.Context) {
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

	var req firewallRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	normalized, err := validateFirewallRuleInput(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	normalized.UserID = userID
	normalized.ServerID = serverID

	var existing models.FirewallRule
	if err := a.db.Where("user_id = ? AND server_id = ? AND provider = ? AND direction = ? AND protocol = ? AND port = ? AND source = ? AND destination = ?",
		userID, serverID, normalized.Provider, normalized.Direction, normalized.Protocol, normalized.Port, normalized.Source, normalized.Destination).
		First(&existing).Error; err == nil {
		if existing.Action == normalized.Action {
			c.JSON(http.StatusConflict, gin.H{"error": "rule already exists"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "conflicting rule exists"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to validate rule"})
		return
	}

	snapshot, snapErr := a.buildFirewallSnapshot(userID, serverID, normalized.Provider)
	if snapErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": snapErr.Error()})
		return
	}

	if err := a.db.Create(&normalized).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create rule"})
		return
	}

	applyErr := error(nil)
	if normalized.Enabled {
		applyErr = a.applyFirewallChange(serverID, normalized.Provider, "add", buildFirewallRulePayload(normalized), nil)
	}

	if applyErr != nil {
		_ = a.db.Delete(&models.FirewallRule{}, normalized.ID).Error
		a.logFirewallAudit(userID, serverID, &normalized.ID, normalized.Provider, "create", false, applyErr.Error(), snapshot)
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(applyErr)})
		return
	}

	if normalized.Action == "deny" || normalized.Action == "reject" {
		if normalized.Source != "any" {
			_ = a.db.Create(&models.FirewallBlockEvent{
				UserID:    userID,
				ServerID:  serverID,
				RuleID:    &normalized.ID,
				Provider:  normalized.Provider,
				IP:        normalized.Source,
				Action:    "block",
				CreatedAt: time.Now().UTC(),
			}).Error
		}
	}

	a.logFirewallAudit(userID, serverID, &normalized.ID, normalized.Provider, "create", true, "", snapshot)
	c.JSON(http.StatusCreated, normalized)
}

func (a *App) handleFirewallUpdateRule(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	ruleID, err := parseUintParam(c, "ruleId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	var current models.FirewallRule
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", ruleID, userID, serverID).First(&current).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	var req firewallRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	req.Provider = current.Provider
	normalized, err := validateFirewallRuleInput(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing models.FirewallRule
	if err := a.db.Where("user_id = ? AND server_id = ? AND provider = ? AND direction = ? AND protocol = ? AND port = ? AND source = ? AND destination = ? AND id <> ?",
		userID, serverID, normalized.Provider, normalized.Direction, normalized.Protocol, normalized.Port, normalized.Source, normalized.Destination, current.ID).
		First(&existing).Error; err == nil {
		if existing.Action == normalized.Action {
			c.JSON(http.StatusConflict, gin.H{"error": "rule already exists"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "conflicting rule exists"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to validate rule"})
		return
	}

	normalized.ID = current.ID
	normalized.UserID = userID
	normalized.ServerID = serverID

	snapshot, snapErr := a.buildFirewallSnapshot(userID, serverID, current.Provider)
	if snapErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": snapErr.Error()})
		return
	}

	operation := "update"
	if current.Enabled && !normalized.Enabled {
		operation = "delete"
	}
	if !current.Enabled && normalized.Enabled {
		operation = "add"
	}
	if !current.Enabled && !normalized.Enabled {
		operation = "noop"
	}

	if operation != "noop" {
		prevPayload := buildFirewallRulePayload(current)
		payload := buildFirewallRulePayload(normalized)
		if operation == "delete" {
			payload = prevPayload
		}
		var prev *firewallRulePayload
		if operation == "update" {
			prev = &prevPayload
		}
		if err := a.applyFirewallChange(serverID, current.Provider, operation, payload, prev); err != nil {
			a.logFirewallAudit(userID, serverID, &current.ID, current.Provider, "update", false, err.Error(), snapshot)
			c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
			return
		}
	}

	if err := a.db.Model(&models.FirewallRule{}).Where("id = ?", current.ID).Updates(map[string]any{
		"direction":   normalized.Direction,
		"action":      normalized.Action,
		"protocol":    normalized.Protocol,
		"port":        normalized.Port,
		"source":      normalized.Source,
		"destination": normalized.Destination,
		"comment":     normalized.Comment,
		"enabled":     normalized.Enabled,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update rule"})
		return
	}

	if current.Action != normalized.Action || current.Source != normalized.Source {
		if current.Action == "deny" || current.Action == "reject" {
			if current.Source != "any" {
				_ = a.db.Create(&models.FirewallBlockEvent{
					UserID:    userID,
					ServerID:  serverID,
					RuleID:    &current.ID,
					Provider:  current.Provider,
					IP:        current.Source,
					Action:    "unblock",
					CreatedAt: time.Now().UTC(),
				}).Error
			}
		}
		if normalized.Action == "deny" || normalized.Action == "reject" {
			if normalized.Source != "any" {
				_ = a.db.Create(&models.FirewallBlockEvent{
					UserID:    userID,
					ServerID:  serverID,
					RuleID:    &current.ID,
					Provider:  current.Provider,
					IP:        normalized.Source,
					Action:    "block",
					CreatedAt: time.Now().UTC(),
				}).Error
			}
		}
	}

	a.logFirewallAudit(userID, serverID, &current.ID, current.Provider, "update", true, "", snapshot)
	c.JSON(http.StatusOK, normalized)
}

func (a *App) handleFirewallDeleteRule(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	serverID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	ruleID, err := parseUintParam(c, "ruleId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	if _, err := a.requireServerForUser(userID, serverID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	var rule models.FirewallRule
	if err := a.db.Where("id = ? AND user_id = ? AND server_id = ?", ruleID, userID, serverID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	snapshot, snapErr := a.buildFirewallSnapshot(userID, serverID, rule.Provider)
	if snapErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": snapErr.Error()})
		return
	}

	if rule.Enabled {
		if err := a.applyFirewallChange(serverID, rule.Provider, "delete", buildFirewallRulePayload(rule), nil); err != nil {
			a.logFirewallAudit(userID, serverID, &rule.ID, rule.Provider, "delete", false, err.Error(), snapshot)
			c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
			return
		}
	}

	_ = a.db.Delete(&models.FirewallRule{}, rule.ID).Error

	if rule.Action == "deny" || rule.Action == "reject" {
		if rule.Source != "any" {
			_ = a.db.Create(&models.FirewallBlockEvent{
				UserID:    userID,
				ServerID:  serverID,
				RuleID:    &rule.ID,
				Provider:  rule.Provider,
				IP:        rule.Source,
				Action:    "unblock",
				CreatedAt: time.Now().UTC(),
			}).Error
		}
	}

	a.logFirewallAudit(userID, serverID, &rule.ID, rule.Provider, "delete", true, "", snapshot)
	c.Status(http.StatusNoContent)
}

func (a *App) handleFirewallQuickRule(c *gin.Context) {
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

	var req firewallQuickRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	input := firewallRuleInput{
		Provider:    req.Provider,
		Direction:   req.Direction,
		Action:      req.Action,
		Protocol:    req.Protocol,
		Port:        req.Port,
		Source:      req.Source,
		Destination: "any",
		Comment:     req.Comment,
	}
	if strings.TrimSpace(input.Direction) == "" {
		input.Direction = "in"
	}

	normalized, err := validateFirewallRuleInput(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	normalized.UserID = userID
	normalized.ServerID = serverID

	snapshot, snapErr := a.buildFirewallSnapshot(userID, serverID, normalized.Provider)
	if snapErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": snapErr.Error()})
		return
	}

	if err := a.db.Create(&normalized).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create rule"})
		return
	}

	if err := a.applyFirewallChange(serverID, normalized.Provider, "add", buildFirewallRulePayload(normalized), nil); err != nil {
		_ = a.db.Delete(&models.FirewallRule{}, normalized.ID).Error
		a.logFirewallAudit(userID, serverID, &normalized.ID, normalized.Provider, "quick_rule", false, err.Error(), snapshot)
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}

	a.logFirewallAudit(userID, serverID, &normalized.ID, normalized.Provider, "quick_rule", true, "", snapshot)
	c.JSON(http.StatusCreated, normalized)
}

func (a *App) handleFirewallRollback(c *gin.Context) {
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

	provider, err := normalizeFirewallProvider(c.Query("provider"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var audit models.FirewallAuditLog
	if err := a.db.Where("user_id = ? AND server_id = ? AND provider = ? AND success = ?", userID, serverID, provider, true).
		Order("id desc").First(&audit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no rollback snapshot found"})
		return
	}

	var snapshot firewallAuditSnapshot
	if err := json.Unmarshal(audit.Snapshot, &snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid snapshot"})
		return
	}

	raw, err := a.hub.RequestAgent(serverID, "firewall_rollback", map[string]any{
		"provider": provider,
		"snapshot": json.RawMessage(snapshot.System),
	}, 35*time.Second)
	if err != nil {
		a.logFirewallAudit(userID, serverID, nil, provider, "rollback", false, err.Error(), datatypes.JSON(audit.Snapshot))
		c.JSON(http.StatusBadGateway, gin.H{"error": publicAgentError(err)})
		return
	}
	if _, err := decodeRawJSON(raw); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from agent"})
		return
	}

	tx := a.db.Begin()
	_ = tx.Where("user_id = ? AND server_id = ? AND provider = ?", userID, serverID, provider).Delete(&models.FirewallRule{}).Error
	for _, rule := range snapshot.ManagedRules {
		rule.ID = 0
		rule.UserID = userID
		rule.ServerID = serverID
		rule.CreatedAt = time.Now().UTC()
		rule.UpdatedAt = time.Now().UTC()
		_ = tx.Create(&rule).Error
	}
	_ = tx.Commit().Error

	a.logFirewallAudit(userID, serverID, nil, provider, "rollback", true, "", datatypes.JSON(audit.Snapshot))
	c.JSON(http.StatusOK, gin.H{"status": "rolled_back"})
}

func (a *App) handleFirewallAudit(c *gin.Context) {
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

	provider := strings.TrimSpace(c.Query("provider"))
	query := a.db.Where("user_id = ? AND server_id = ?", userID, serverID)
	if provider != "" {
		if normalized, err := normalizeFirewallProvider(provider); err == nil {
			query = query.Where("provider = ?", normalized)
		}
	}

	var logs []models.FirewallAuditLog
	if err := query.Order("id desc").Limit(limit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load audit logs"})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (a *App) handleFirewallBlockedHistory(c *gin.Context) {
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

	var events []models.FirewallBlockEvent
	if err := a.db.Where("user_id = ? AND server_id = ?", userID, serverID).Order("id desc").Limit(limit).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load blocked history"})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (a *App) buildFirewallSnapshot(userID, serverID uint, provider string) (datatypes.JSON, error) {
	raw, err := a.hub.RequestAgent(serverID, "firewall_snapshot", map[string]any{"provider": provider}, 20*time.Second)
	if err != nil {
		return nil, errors.New(publicAgentError(err))
	}
	var managed []models.FirewallRule
	if err := a.db.Where("user_id = ? AND server_id = ? AND provider = ?", userID, serverID, provider).Find(&managed).Error; err != nil {
		return nil, errors.New("unable to load snapshot")
	}
	return marshalFirewallSnapshot(provider, raw, managed), nil
}

func (a *App) applyFirewallChange(serverID uint, provider, operation string, rule firewallRulePayload, prev *firewallRulePayload) error {
	payload := firewallApplyRequest{
		Provider:  provider,
		Operation: operation,
		Rule:      rule,
		Previous:  prev,
	}
	raw, err := a.hub.RequestAgent(serverID, "firewall_apply", payload, 30*time.Second)
	if err != nil {
		return err
	}
	_, decodeErr := decodeRawJSON(raw)
	return decodeErr
}
