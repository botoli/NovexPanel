package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type Hub struct {
	db *gorm.DB

	mu                sync.RWMutex
	agents            map[uint]*AgentClient
	metricSubscribers map[uint]map[*SiteClient]struct{}
	deploySubscribers map[uint]map[*SiteClient]struct{}
	terminals         map[string]*TerminalSession
}

type AgentClient struct {
	serverID uint
	userID   uint
	conn     *websocket.Conn

	sendMu    sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan commandResult
}

type SiteClient struct {
	userID uint
	conn   *websocket.Conn

	sendMu          sync.Mutex
	metricSubs      map[uint]struct{}
	deploySubs      map[uint]struct{}
	activeTerminals map[uint]string
}

type TerminalSession struct {
	ServerID uint
	Site     *SiteClient
}

type commandResult struct {
	Success bool
	Data    json.RawMessage
	Err     string
}

func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		db:                db,
		agents:            make(map[uint]*AgentClient),
		metricSubscribers: make(map[uint]map[*SiteClient]struct{}),
		deploySubscribers: make(map[uint]map[*SiteClient]struct{}),
		terminals:         make(map[string]*TerminalSession),
	}
}

func (h *Hub) NewSiteClient(userID uint, conn *websocket.Conn) *SiteClient {
	return &SiteClient{
		userID:          userID,
		conn:            conn,
		metricSubs:      make(map[uint]struct{}),
		deploySubs:      make(map[uint]struct{}),
		activeTerminals: make(map[uint]string),
	}
}

func (a *AgentClient) sendJSON(v any) error {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()

	if err := a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return a.conn.WriteJSON(v)
}

func (s *SiteClient) sendJSON(v any) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if err := s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return s.conn.WriteJSON(v)
}

func (h *Hub) RegisterAgent(serverID, userID uint, conn *websocket.Conn) *AgentClient {
	client := &AgentClient{
		serverID: serverID,
		userID:   userID,
		conn:     conn,
		pending:  make(map[string]chan commandResult),
	}

	var old *AgentClient
	h.mu.Lock()
	old = h.agents[serverID]
	h.agents[serverID] = client
	h.mu.Unlock()

	if old != nil {
		old.failPending(errors.New("agent replaced by new connection"))
		_ = old.conn.Close()
	}

	return client
}

func (h *Hub) UnregisterAgent(serverID uint, client *AgentClient) {
	h.mu.Lock()
	current := h.agents[serverID]
	if current == client {
		delete(h.agents, serverID)
	}
	h.mu.Unlock()

	client.failPending(errors.New("agent disconnected"))
	_ = client.conn.Close()
}

func (h *Hub) KickServer(serverID uint) {
	h.mu.RLock()
	client := h.agents[serverID]
	h.mu.RUnlock()
	if client == nil {
		return
	}
	client.failPending(errors.New("agent token revoked"))
	_ = client.conn.Close()
}

func (h *Hub) RequestAgent(serverID uint, command string, payload any, timeout time.Duration) (json.RawMessage, error) {
	h.mu.RLock()
	agent := h.agents[serverID]
	h.mu.RUnlock()
	if agent == nil {
		return nil, errors.New("agent is offline")
	}

	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	requestID := uuid.NewString()
	ch := make(chan commandResult, 1)

	agent.pendingMu.Lock()
	agent.pending[requestID] = ch
	agent.pendingMu.Unlock()

	message := map[string]any{
		"type":       "command",
		"command":    command,
		"request_id": requestID,
	}
	if payload != nil {
		message["payload"] = payload
	}

	if err := agent.sendJSON(message); err != nil {
		agent.pendingMu.Lock()
		delete(agent.pending, requestID)
		agent.pendingMu.Unlock()
		return nil, err
	}

	select {
	case result := <-ch:
		if !result.Success {
			if result.Err == "" {
				result.Err = "agent command failed"
			}
			return nil, errors.New(result.Err)
		}
		return result.Data, nil
	case <-time.After(timeout):
		agent.pendingMu.Lock()
		delete(agent.pending, requestID)
		agent.pendingMu.Unlock()
		return nil, fmt.Errorf("agent command timeout")
	}
}

func (h *Hub) SendAgentEvent(serverID uint, payload any) error {
	h.mu.RLock()
	agent := h.agents[serverID]
	h.mu.RUnlock()
	if agent == nil {
		return errors.New("agent is offline")
	}
	return agent.sendJSON(payload)
}

func (a *AgentClient) completeRequest(requestID string, result commandResult) {
	a.pendingMu.Lock()
	ch, ok := a.pending[requestID]
	if ok {
		delete(a.pending, requestID)
	}
	a.pendingMu.Unlock()

	if ok {
		ch <- result
	}
}

func (a *AgentClient) failPending(err error) {
	a.pendingMu.Lock()
	pending := a.pending
	a.pending = make(map[string]chan commandResult)
	a.pendingMu.Unlock()

	for _, ch := range pending {
		ch <- commandResult{Success: false, Err: err.Error()}
	}
}

func (h *Hub) SubscribeMetrics(site *SiteClient, serverID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.metricSubscribers[serverID] == nil {
		h.metricSubscribers[serverID] = make(map[*SiteClient]struct{})
	}
	h.metricSubscribers[serverID][site] = struct{}{}
	site.metricSubs[serverID] = struct{}{}
}

func (h *Hub) UnsubscribeMetrics(site *SiteClient, serverID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.metricSubscribers[serverID]; ok {
		delete(subs, site)
		if len(subs) == 0 {
			delete(h.metricSubscribers, serverID)
		}
	}
	delete(site.metricSubs, serverID)
}

func (h *Hub) BroadcastMetrics(serverID uint, data json.RawMessage) {
	var decoded any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &decoded); err != nil {
			decoded = map[string]any{}
		}
	} else {
		decoded = map[string]any{}
	}

	h.mu.RLock()
	subMap := h.metricSubscribers[serverID]
	sites := make([]*SiteClient, 0, len(subMap))
	for site := range subMap {
		sites = append(sites, site)
	}
	h.mu.RUnlock()

	message := map[string]any{
		"type":      "metrics",
		"server_id": serverID,
		"data":      decoded,
	}
	for _, site := range sites {
		_ = site.sendJSON(message)
	}
}

func (h *Hub) SubscribeDeploy(site *SiteClient, deployID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.deploySubscribers[deployID] == nil {
		h.deploySubscribers[deployID] = make(map[*SiteClient]struct{})
	}
	h.deploySubscribers[deployID][site] = struct{}{}
	site.deploySubs[deployID] = struct{}{}
}

func (h *Hub) UnsubscribeDeploy(site *SiteClient, deployID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.deploySubscribers[deployID]; ok {
		delete(subs, site)
		if len(subs) == 0 {
			delete(h.deploySubscribers, deployID)
		}
	}
	delete(site.deploySubs, deployID)
}

func (h *Hub) BroadcastDeployLog(deployID uint, line string, isError bool) {
	h.mu.RLock()
	subMap := h.deploySubscribers[deployID]
	sites := make([]*SiteClient, 0, len(subMap))
	for site := range subMap {
		sites = append(sites, site)
	}
	h.mu.RUnlock()

	message := map[string]any{
		"type":      "deploy_log",
		"deploy_id": deployID,
		"line":      line,
		"is_error":  isError,
	}
	for _, site := range sites {
		_ = site.sendJSON(message)
	}
}

func (h *Hub) BroadcastDeployComplete(deployID uint, success bool, url, errText string) {
	h.mu.RLock()
	subMap := h.deploySubscribers[deployID]
	sites := make([]*SiteClient, 0, len(subMap))
	for site := range subMap {
		sites = append(sites, site)
	}
	h.mu.RUnlock()

	message := map[string]any{
		"type":      "deploy_complete",
		"deploy_id": deployID,
		"success":   success,
		"url":       url,
	}
	if errText != "" {
		message["error"] = errText
	}

	for _, site := range sites {
		_ = site.sendJSON(message)
	}
}

func (h *Hub) OpenTerminal(site *SiteClient, serverID uint, rows, cols int) (string, error) {
	sessionID := uuid.NewString()
	_, err := h.RequestAgent(serverID, "run_terminal", map[string]any{
		"session_id": sessionID,
		"rows":       rows,
		"cols":       cols,
	}, 15*time.Second)
	if err != nil {
		return "", err
	}

	var previous string
	h.mu.Lock()
	previous = site.activeTerminals[serverID]
	if previous != "" {
		delete(h.terminals, previous)
	}
	h.terminals[sessionID] = &TerminalSession{ServerID: serverID, Site: site}
	site.activeTerminals[serverID] = sessionID
	h.mu.Unlock()

	if previous != "" {
		_ = h.SendAgentEvent(serverID, map[string]any{
			"type":       "terminal_close",
			"session_id": previous,
		})
	}

	return sessionID, nil
}

func (h *Hub) TerminalInput(site *SiteClient, serverID uint, sessionID, data string) error {
	h.mu.RLock()
	if sessionID == "" {
		if serverID != 0 {
			sessionID = site.activeTerminals[serverID]
		} else if len(site.activeTerminals) == 1 {
			for srvID, sid := range site.activeTerminals {
				serverID = srvID
				sessionID = sid
			}
		}
	}
	if serverID == 0 && sessionID != "" {
		if session := h.terminals[sessionID]; session != nil {
			serverID = session.ServerID
		}
	}
	h.mu.RUnlock()

	if sessionID == "" || serverID == 0 {
		return errors.New("terminal session not found")
	}

	return h.SendAgentEvent(serverID, map[string]any{
		"type":       "terminal_input",
		"session_id": sessionID,
		"data":       data,
	})
}

func (h *Hub) CloseTerminal(site *SiteClient, serverID uint) {
	var sessionID string
	h.mu.Lock()
	sessionID = site.activeTerminals[serverID]
	if sessionID != "" {
		delete(site.activeTerminals, serverID)
		delete(h.terminals, sessionID)
	}
	h.mu.Unlock()

	if sessionID != "" {
		_ = h.SendAgentEvent(serverID, map[string]any{
			"type":       "terminal_close",
			"session_id": sessionID,
		})
	}
}

func (h *Hub) ForwardTerminalOutput(sessionID string, serverID uint, data string) {
	h.mu.RLock()
	session := h.terminals[sessionID]
	h.mu.RUnlock()
	if session == nil {
		return
	}

	_ = session.Site.sendJSON(map[string]any{
		"type":       "terminal_output",
		"server_id":  serverID,
		"session_id": sessionID,
		"data":       data,
	})
}

func (h *Hub) RemoveSite(site *SiteClient) {
	type terminalToClose struct {
		serverID  uint
		sessionID string
	}

	toClose := make([]terminalToClose, 0)
	h.mu.Lock()
	for serverID := range site.metricSubs {
		if subs := h.metricSubscribers[serverID]; subs != nil {
			delete(subs, site)
			if len(subs) == 0 {
				delete(h.metricSubscribers, serverID)
			}
		}
	}
	for deployID := range site.deploySubs {
		if subs := h.deploySubscribers[deployID]; subs != nil {
			delete(subs, site)
			if len(subs) == 0 {
				delete(h.deploySubscribers, deployID)
			}
		}
	}
	for serverID, sessionID := range site.activeTerminals {
		delete(h.terminals, sessionID)
		toClose = append(toClose, terminalToClose{serverID: serverID, sessionID: sessionID})
	}
	site.metricSubs = make(map[uint]struct{})
	site.deploySubs = make(map[uint]struct{})
	site.activeTerminals = make(map[uint]string)
	h.mu.Unlock()

	for _, session := range toClose {
		_ = h.SendAgentEvent(session.serverID, map[string]any{
			"type":       "terminal_close",
			"session_id": session.sessionID,
		})
	}

	_ = site.conn.Close()
}
