package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type agentConfig struct {
	BackendURL      string
	Token           string
	Name            string
	MetricsInterval time.Duration
	InsecureTLS     bool
}

type storedAgentConfig struct {
	BackendURL      string `json:"backend_url"`
	Token           string `json:"token"`
	Name            string `json:"name"`
	MetricsInterval string `json:"metrics_interval"`
	InsecureTLS     bool   `json:"insecure_tls"`
}

const (
	defaultBackendURL      = "ws://localhost:8380"
	defaultMetricsInterval = 2 * time.Second
	agentWSPongWait        = 90 * time.Second
	agentWSPingInterval    = 30 * time.Second

	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

var colorLogsEnabled bool

type Agent struct {
	cfg agentConfig

	conn   *websocket.Conn
	sendMu sync.Mutex

	termMu    sync.RWMutex
	terminals map[string]*terminalSession

	netMu     sync.Mutex
	lastRX    uint64
	lastTX    uint64
	lastNetAt time.Time

	diskMu        sync.Mutex
	lastDiskRead  uint64
	lastDiskWrite uint64
	lastDiskAt    time.Time

	deployMu      sync.Mutex
	activeDeploys map[uint]deployRuntime
}

type deployRuntime struct {
	ContainerName string
}

type terminalSession struct {
	cmd     *exec.Cmd
	ptyFile *os.File
}

type commandMessage struct {
	Type      string          `json:"type"`
	Command   string          `json:"command"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type deployPayload struct {
	DeployID             uint           `json:"deploy_id"`
	DeployIDCamel        uint           `json:"deployId"`
	Source               string         `json:"source"`
	RepoURL              string         `json:"repo_url"`
	RepoURLCamel         string         `json:"repoUrl"`
	Branch               string         `json:"branch"`
	Type                 string         `json:"type"`
	ProjectType          string         `json:"project_type"`
	ProjectTypeCamel     string         `json:"projectType"`
	Subdirectory         string         `json:"subdirectory"`
	SubdirectoryAlt      string         `json:"sub_directory"`
	SubdirectoryCamel    string         `json:"subDirectory"`
	BuildCommand         string         `json:"build_command"`
	BuildCommandAlt      string         `json:"buildCommand"`
	OutputDir            string         `json:"output_dir"`
	OutputDirAlt         string         `json:"outputDir"`
	ZipData              string         `json:"zip_data"`
	Env                  map[string]any `json:"env"`
	EnvCamel             map[string]any `json:"envVars"`
	Environment          map[string]any `json:"environment"`
	EnvironmentCamel     map[string]any `json:"environmentVars"`
	ContainerEnvironment map[string]any `json:"container_env"`
	ContainerEnvCamel    map[string]any `json:"containerEnv"`
}

func (p *deployPayload) normalize() {
	if p.DeployID == 0 {
		p.DeployID = p.DeployIDCamel
	}
	if strings.TrimSpace(p.RepoURL) == "" {
		p.RepoURL = strings.TrimSpace(p.RepoURLCamel)
	}
	if strings.TrimSpace(p.ProjectType) == "" {
		p.ProjectType = strings.TrimSpace(p.ProjectTypeCamel)
	}
	if strings.TrimSpace(p.ProjectType) == "" {
		p.ProjectType = strings.TrimSpace(p.Type)
	}
	if strings.TrimSpace(p.Subdirectory) == "" {
		p.Subdirectory = strings.TrimSpace(p.SubdirectoryAlt)
	}
	if strings.TrimSpace(p.Subdirectory) == "" {
		p.Subdirectory = strings.TrimSpace(p.SubdirectoryCamel)
	}
	if strings.TrimSpace(p.BuildCommand) == "" {
		p.BuildCommand = strings.TrimSpace(p.BuildCommandAlt)
	}
	if strings.TrimSpace(p.OutputDir) == "" {
		p.OutputDir = strings.TrimSpace(p.OutputDirAlt)
	}
	if strings.TrimSpace(p.Source) == "" {
		p.Source = "github"
	}
	if strings.TrimSpace(p.Branch) == "" {
		p.Branch = "main"
	}
}

func (p *deployPayload) normalizedEnvVars() map[string]string {
	envVars := make(map[string]string)

	mergeMap := func(raw map[string]any) {
		for key, value := range raw {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" || value == nil {
				continue
			}
			envVars[trimmedKey] = strings.TrimSpace(fmt.Sprintf("%v", value))
		}
	}

	mergeMap(p.Env)
	mergeMap(p.EnvCamel)
	mergeMap(p.Environment)
	mergeMap(p.EnvironmentCamel)
	mergeMap(p.ContainerEnvironment)
	mergeMap(p.ContainerEnvCamel)

	return envVars
}

func main() {
	initLogger()

	cliCfg := agentConfig{}
	flag.StringVar(&cliCfg.BackendURL, "backend", defaultBackendURL, "Backend WebSocket base URL")
	flag.StringVar(&cliCfg.Token, "token", "", "Agent token generated by backend")
	flag.StringVar(&cliCfg.Name, "name", "", "Optional server name shown in panel")
	flag.DurationVar(&cliCfg.MetricsInterval, "metrics-interval", defaultMetricsInterval, "Metrics push interval")
	flag.BoolVar(&cliCfg.InsecureTLS, "insecure-tls", false, "Skip TLS cert verification (dev only)")
	flag.Parse()
	setFlags := collectSetFlags()

	cfg, cfgPath, err := resolveAgentConfig(cliCfg, setFlags)
	if err != nil {
		logFatal("resolve agent config: %v", err)
	}

	normalizedURL, err := normalizeBackendURL(cfg.BackendURL)
	if err != nil {
		logFatal("invalid backend URL: %v", err)
	}
	cfg.BackendURL = normalizedURL

	if err := saveAgentConfig(cfgPath, cfg); err != nil {
		logWarn("failed to save config to %s: %v", cfgPath, err)
	} else {
		logSuccess("config saved to %s", cfgPath)
	}

	logInfo("agent started: backend=%s, name=%q, metrics_interval=%s", cfg.BackendURL, cfg.Name, cfg.MetricsInterval)

	agent := &Agent{
		cfg:           cfg,
		terminals:     make(map[string]*terminalSession),
		activeDeploys: make(map[uint]deployRuntime),
	}
	agent.runForever()
}

func initLogger() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags)
	colorLogsEnabled = isTerminal(os.Stdout) && strings.TrimSpace(os.Getenv("NO_COLOR")) == ""
}

func isTerminal(stream *os.File) bool {
	if stream == nil {
		return false
	}
	info, err := stream.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func colorize(color, text string) string {
	if !colorLogsEnabled {
		return text
	}
	return color + text + colorReset
}

func logInfo(format string, args ...any) {
	log.Printf("%s %s", colorize(colorBlue, "[INFO]"), fmt.Sprintf(format, args...))
}

func logWarn(format string, args ...any) {
	log.Printf("%s %s", colorize(colorYellow, "[WARN]"), fmt.Sprintf(format, args...))
}

func logError(format string, args ...any) {
	log.Printf("%s %s", colorize(colorRed, "[ERROR]"), fmt.Sprintf(format, args...))
}

func logSuccess(format string, args ...any) {
	log.Printf("%s %s", colorize(colorGreen, "[OK]"), fmt.Sprintf(format, args...))
}

func logFatal(format string, args ...any) {
	log.Fatalf("%s %s", colorize(colorRed, "[FATAL]"), fmt.Sprintf(format, args...))
}

func collectSetFlags() map[string]bool {
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})
	return setFlags
}

func resolveAgentConfig(cliCfg agentConfig, setFlags map[string]bool) (agentConfig, string, error) {
	configPath, err := defaultConfigPath()
	if err != nil {
		return agentConfig{}, "", err
	}

	cfg := agentConfig{
		BackendURL:      defaultBackendURL,
		MetricsInterval: defaultMetricsInterval,
	}

	storedCfg, hasStoredConfig, err := loadAgentConfig(configPath)
	if err != nil {
		return agentConfig{}, "", err
	}
	if hasStoredConfig {
		cfg = mergeConfig(cfg, storedCfg)
		logInfo("loaded config from %s", configPath)
	}

	applyCLIOverrides(&cfg, cliCfg, setFlags)

	if cfg.MetricsInterval <= 0 {
		cfg.MetricsInterval = defaultMetricsInterval
	}

	interactive := isTerminal(os.Stdin)
	reader := bufio.NewReader(os.Stdin)

	if strings.TrimSpace(cfg.Token) == "" {
		if !interactive {
			return agentConfig{}, "", fmt.Errorf("agent token is empty; pass -token or run in interactive mode")
		}
		logInfo("interactive setup: token is required")
		token, err := promptRequired(reader, "Enter agent token: ")
		if err != nil {
			return agentConfig{}, "", err
		}
		cfg.Token = token
	}

	if strings.TrimSpace(cfg.Name) == "" {
		if interactive {
			name, err := promptInput(reader, "Enter server name (empty = system hostname): ")
			if err != nil {
				return agentConfig{}, "", err
			}
			cfg.Name = name
		}
		if strings.TrimSpace(cfg.Name) == "" {
			cfg.Name = fallbackSystemName()
			logInfo("server name is empty, using system hostname: %s", cfg.Name)
		}
	}

	return cfg, configPath, nil
}

func defaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			if err != nil {
				return "", err
			}
			return "", homeErr
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "novex", "agent.json"), nil
}

func loadAgentConfig(path string) (agentConfig, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return agentConfig{}, false, nil
		}
		return agentConfig{}, false, err
	}

	var stored storedAgentConfig
	if err := json.Unmarshal(content, &stored); err != nil {
		return agentConfig{}, false, fmt.Errorf("decode config: %w", err)
	}

	cfg := agentConfig{
		BackendURL:  strings.TrimSpace(stored.BackendURL),
		Token:       strings.TrimSpace(stored.Token),
		Name:        strings.TrimSpace(stored.Name),
		InsecureTLS: stored.InsecureTLS,
	}
	if intervalText := strings.TrimSpace(stored.MetricsInterval); intervalText != "" {
		if duration, err := time.ParseDuration(intervalText); err == nil {
			cfg.MetricsInterval = duration
		}
	}

	return cfg, true, nil
}

func mergeConfig(base, extra agentConfig) agentConfig {
	if v := strings.TrimSpace(extra.BackendURL); v != "" {
		base.BackendURL = v
	}
	if v := strings.TrimSpace(extra.Token); v != "" {
		base.Token = v
	}
	if v := strings.TrimSpace(extra.Name); v != "" {
		base.Name = v
	}
	if extra.MetricsInterval > 0 {
		base.MetricsInterval = extra.MetricsInterval
	}
	base.InsecureTLS = extra.InsecureTLS
	return base
}

func applyCLIOverrides(cfg *agentConfig, cliCfg agentConfig, setFlags map[string]bool) {
	if setFlags["backend"] {
		cfg.BackendURL = strings.TrimSpace(cliCfg.BackendURL)
	}
	if setFlags["token"] {
		cfg.Token = strings.TrimSpace(cliCfg.Token)
	}
	if setFlags["name"] {
		cfg.Name = strings.TrimSpace(cliCfg.Name)
	}
	if setFlags["metrics-interval"] {
		cfg.MetricsInterval = cliCfg.MetricsInterval
	}
	if setFlags["insecure-tls"] {
		cfg.InsecureTLS = cliCfg.InsecureTLS
	}
}

func promptInput(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(colorize(colorCyan, prompt))
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if errors.Is(err, io.EOF) && line == "" {
		return "", io.EOF
	}
	return line, nil
}

func promptRequired(reader *bufio.Reader, prompt string) (string, error) {
	for {
		value, err := promptInput(reader, prompt)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
		logWarn("value cannot be empty")
	}
}

func fallbackSystemName() string {
	hostName, err := os.Hostname()
	if err != nil {
		return "server"
	}
	hostName = strings.TrimSpace(hostName)
	if hostName == "" {
		return "server"
	}
	return hostName
}

func saveAgentConfig(path string, cfg agentConfig) error {
	stored := storedAgentConfig{
		BackendURL:      strings.TrimSpace(cfg.BackendURL),
		Token:           strings.TrimSpace(cfg.Token),
		Name:            strings.TrimSpace(cfg.Name),
		MetricsInterval: cfg.MetricsInterval.String(),
		InsecureTLS:     cfg.InsecureTLS,
	}

	content, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func normalizeBackendURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return "", fmt.Errorf("scheme must be ws/wss/http/https")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	return u.String(), nil
}

func (a *Agent) runForever() {
	backoff := 1 * time.Second
	for {
		err := a.connectAndRun()
		if err != nil {
			logWarn("agent disconnected: %v", err)
		}
		logInfo("reconnect in %s", backoff)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (a *Agent) connectAndRun() error {
	wsURL := strings.TrimRight(a.cfg.BackendURL, "/") + "/agent/ws?token=" + url.QueryEscape(a.cfg.Token)
	if name := strings.TrimSpace(a.cfg.Name); name != "" {
		wsURL += "&name=" + url.QueryEscape(name)
	}
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	if a.cfg.InsecureTLS {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	conn.SetReadLimit(512 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(agentWSPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(agentWSPongWait))
	})
	a.conn = conn
	defer func() {
		a.closeAllTerminals()
		_ = conn.Close()
		a.conn = nil
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.metricsLoop(ctx)
	go a.pingLoop(ctx)
	logSuccess("connected to %s", wsURL)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		a.handleMessage(payload)
	}
}

func (a *Agent) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(agentWSPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.sendPing(); err != nil {
				return
			}
		}
	}
}

func (a *Agent) sendPing() error {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()

	if a.conn == nil {
		return errors.New("connection is not established")
	}
	if err := a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return a.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(10*time.Second))
}

func (a *Agent) handleMessage(payload []byte) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		logWarn("agent message unmarshal failed: %v", err)
		return
	}
	logInfo("agent received message: type=%q", strings.TrimSpace(envelope.Type))

	switch envelope.Type {
	case "command":
		var msg commandMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			logWarn("agent command envelope unmarshal failed: %v", err)
			return
		}
		logInfo("agent received command envelope: command=%s request_id=%s", msg.Command, msg.RequestID)
		go a.handleCommand(msg)
	case "deploy":
		var deployMsg deployPayload
		if err := json.Unmarshal(payload, &deployMsg); err != nil {
			logWarn("agent deploy event unmarshal failed: %v", err)
			return
		}
		deployMsg.normalize()
		if deployMsg.DeployID == 0 {
			logWarn("agent deploy event ignored: missing deploy_id")
			return
		}
		logInfo("agent received deploy event: deploy_id=%d repo=%q branch=%q", deployMsg.DeployID, deployMsg.RepoURL, deployMsg.Branch)
		go a.runDeploy(deployMsg)
	case "stop_deploy":
		var msg struct {
			DeployID      uint `json:"deploy_id"`
			DeployIDCamel uint `json:"deployId"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			logWarn("agent stop_deploy event unmarshal failed: %v", err)
			return
		}
		if msg.DeployID == 0 {
			msg.DeployID = msg.DeployIDCamel
		}
		if msg.DeployID == 0 {
			logWarn("agent stop_deploy event ignored: missing deploy_id")
			return
		}
		logInfo("agent received stop_deploy event: deploy_id=%d", msg.DeployID)
		if err := a.stopDeploy(msg.DeployID); err != nil {
			logWarn("stop_deploy failed (deploy_id=%d): %v", msg.DeployID, err)
		}
	case "terminal_input":
		var msg struct {
			SessionID string `json:"session_id"`
			Data      string `json:"data"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		_ = a.writeTerminalInput(msg.SessionID, msg.Data)
	case "terminal_resize":
		var msg struct {
			SessionID string `json:"session_id"`
			Rows      int    `json:"rows"`
			Cols      int    `json:"cols"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		_ = a.resizeTerminal(msg.SessionID, msg.Rows, msg.Cols)
	case "terminal_close":
		var msg struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		a.closeTerminal(msg.SessionID)
	case "ping":
		_ = a.sendJSON(map[string]any{"type": "pong"})
	default:
		logWarn("agent received unknown message type: %q", envelope.Type)
	}
}

func (a *Agent) handleCommand(msg commandMessage) {
	logInfo("agent handling command: command=%s request_id=%s", msg.Command, msg.RequestID)

	respondError := func(err error) {
		a.sendCommandResponse(msg.RequestID, false, nil, err.Error())
	}

	switch msg.Command {
	case "ping":
		a.sendCommandResponse(msg.RequestID, true, map[string]any{"pong": true}, "")
	case "get_metrics":
		metrics, err := a.collectMetrics()
		if err != nil {
			respondError(err)
			return
		}
		a.sendCommandResponse(msg.RequestID, true, metrics, "")
	case "run_command":
		var payload struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			respondError(fmt.Errorf("invalid payload"))
			return
		}
		stdout, stderr, exitCode, runErr := runShellCommand(payload.Command)
		resp := map[string]any{
			"stdout":    stdout,
			"stderr":    stderr,
			"exit_code": exitCode,
		}
		if runErr != nil && exitCode < 0 {
			respondError(runErr)
			return
		}
		a.sendCommandResponse(msg.RequestID, true, resp, "")
	case "get_processes":
		processes, err := a.collectProcesses(30)
		if err != nil {
			respondError(err)
			return
		}
		a.sendCommandResponse(msg.RequestID, true, map[string]any{"processes": processes}, "")
	case "kill_process":
		var payload struct {
			PID int `json:"pid"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			respondError(fmt.Errorf("invalid payload"))
			return
		}
		if payload.PID <= 0 {
			respondError(fmt.Errorf("invalid pid"))
			return
		}
		if err := killProcess(payload.PID); err != nil {
			respondError(err)
			return
		}
		a.sendCommandResponse(msg.RequestID, true, map[string]any{"success": true}, "")
	case "run_terminal":
		var payload struct {
			SessionID string `json:"session_id"`
			Rows      int    `json:"rows"`
			Cols      int    `json:"cols"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			respondError(fmt.Errorf("invalid payload"))
			return
		}
		if payload.SessionID == "" {
			respondError(fmt.Errorf("session_id is required"))
			return
		}
		if err := a.openTerminal(payload.SessionID, payload.Rows, payload.Cols); err != nil {
			respondError(err)
			return
		}
		a.sendCommandResponse(msg.RequestID, true, map[string]any{"session_id": payload.SessionID}, "")
	case "deploy":
		var payload deployPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			respondError(fmt.Errorf("invalid payload"))
			return
		}
		payload.normalize()
		if payload.DeployID == 0 {
			respondError(fmt.Errorf("deploy_id is required"))
			return
		}
		logInfo("agent received deploy command: deploy_id=%d repo=%q branch=%q", payload.DeployID, payload.RepoURL, payload.Branch)
		go a.runDeploy(payload)
		a.sendCommandResponse(msg.RequestID, true, map[string]any{"accepted": true}, "")
	case "stop_deploy":
		var payload struct {
			DeployID      uint `json:"deploy_id"`
			DeployIDCamel uint `json:"deployId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			respondError(fmt.Errorf("invalid payload"))
			return
		}
		if payload.DeployID == 0 {
			payload.DeployID = payload.DeployIDCamel
		}
		if payload.DeployID == 0 {
			respondError(fmt.Errorf("deploy_id is required"))
			return
		}
		logInfo("agent received stop_deploy command: deploy_id=%d", payload.DeployID)
		if err := a.stopDeploy(payload.DeployID); err != nil {
			respondError(err)
			return
		}
		a.sendCommandResponse(msg.RequestID, true, map[string]any{"accepted": true}, "")
	default:
		logError("unknown command received: %s", msg.Command)
		respondError(fmt.Errorf("unknown command: %s", msg.Command))
	}
}

func runShellCommand(command string) (stdout, stderr string, exitCode int, err error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", "", -1, errors.New("command is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	exitCode = 0

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			return stdout, stderr, exitCode, nil
		}
		exitCode = -1
		return stdout, stderr, exitCode, err
	}

	return stdout, stderr, exitCode, nil
}

func (a *Agent) sendJSON(v any) error {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()

	if a.conn == nil {
		return errors.New("connection is not established")
	}
	if err := a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return a.conn.WriteJSON(v)
}

func (a *Agent) sendCommandResponse(requestID string, success bool, data any, errText string) {
	message := map[string]any{
		"type":       "command_response",
		"request_id": requestID,
		"success":    success,
	}
	if data != nil {
		message["data"] = data
	}
	if errText != "" {
		message["error"] = errText
	}
	_ = a.sendJSON(message)
}

func truncateLogPayload(payload []byte, maxLen int) string {
	text := strings.TrimSpace(string(payload))
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func (a *Agent) metricsLoop(ctx context.Context) {
	send := func() {
		metrics, err := a.collectMetrics()
		if err != nil {
			logWarn("collect metrics error: %v", err)
			return
		}
		if err := a.sendJSON(map[string]any{"type": "metrics", "data": metrics}); err != nil {
			logWarn("send metrics error: %v", err)
		}
	}

	send()
	ticker := time.NewTicker(a.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

func (a *Agent) collectMetrics() (map[string]any, error) {
	cpuUsage := 0.0
	cpuValues, err := cpu.Percent(0, false)
	if err == nil && len(cpuValues) > 0 {
		cpuUsage = cpuValues[0]
	}

	cores, _ := cpu.Counts(true)
	loadAvg, _ := load.Avg()
	vmem, _ := mem.VirtualMemory()
	diskUsage, _ := disk.Usage("/")
	diskCounters, _ := disk.IOCounters()
	uptime, _ := host.Uptime()
	netCounters, _ := gnet.IOCounters(false)

	topProcesses, _ := a.collectProcesses(5)
	temperature := readTemperature()

	rxBytes := uint64(0)
	txBytes := uint64(0)
	if len(netCounters) > 0 {
		rxBytes = netCounters[0].BytesRecv
		txBytes = netCounters[0].BytesSent
	}
	rxSpeed, txSpeed := a.calculateNetSpeed(rxBytes, txBytes)

	diskReadBytes := uint64(0)
	diskWriteBytes := uint64(0)
	for _, counter := range diskCounters {
		diskReadBytes += counter.ReadBytes
		diskWriteBytes += counter.WriteBytes
	}
	diskReadSpeed, diskWriteSpeed := a.calculateDiskSpeed(diskReadBytes, diskWriteBytes)

	result := map[string]any{
		"cpu": map[string]any{
			"usage":    cpuUsage,
			"cores":    cores,
			"load_avg": []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15},
		},
		"ram": map[string]any{
			"total":   vmem.Total,
			"used":    vmem.Used,
			"free":    vmem.Free,
			"percent": vmem.UsedPercent,
		},
		"disk": map[string]any{
			"total":       diskUsage.Total,
			"used":        diskUsage.Used,
			"free":        diskUsage.Free,
			"percent":     diskUsage.UsedPercent,
			"read_bytes":  diskReadBytes,
			"write_bytes": diskWriteBytes,
			"read_speed":  diskReadSpeed,
			"write_speed": diskWriteSpeed,
		},
		"network": map[string]any{
			"rx_bytes": rxBytes,
			"tx_bytes": txBytes,
			"rx_speed": rxSpeed,
			"tx_speed": txSpeed,
		},
		"uptime":        uptime,
		"temperature":   temperature,
		"top_processes": topProcesses,
	}
	return result, nil
}

func readTemperature() float64 {
	temps, err := host.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return 0
	}
	for _, t := range temps {
		if t.Temperature > 0 {
			return t.Temperature
		}
	}
	return 0
}

func (a *Agent) calculateNetSpeed(rx, tx uint64) (float64, float64) {
	a.netMu.Lock()
	defer a.netMu.Unlock()

	now := time.Now()
	if a.lastNetAt.IsZero() {
		a.lastNetAt = now
		a.lastRX = rx
		a.lastTX = tx
		return 0, 0
	}

	seconds := now.Sub(a.lastNetAt).Seconds()
	if seconds <= 0 {
		return 0, 0
	}
	if rx < a.lastRX || tx < a.lastTX {
		a.lastNetAt = now
		a.lastRX = rx
		a.lastTX = tx
		return 0, 0
	}

	rxSpeed := float64(rx-a.lastRX) / seconds
	txSpeed := float64(tx-a.lastTX) / seconds

	a.lastNetAt = now
	a.lastRX = rx
	a.lastTX = tx

	return rxSpeed, txSpeed
}

func (a *Agent) calculateDiskSpeed(readBytes, writeBytes uint64) (float64, float64) {
	a.diskMu.Lock()
	defer a.diskMu.Unlock()

	now := time.Now()
	if a.lastDiskAt.IsZero() {
		a.lastDiskAt = now
		a.lastDiskRead = readBytes
		a.lastDiskWrite = writeBytes
		return 0, 0
	}

	seconds := now.Sub(a.lastDiskAt).Seconds()
	if seconds <= 0 {
		return 0, 0
	}
	if readBytes < a.lastDiskRead || writeBytes < a.lastDiskWrite {
		a.lastDiskAt = now
		a.lastDiskRead = readBytes
		a.lastDiskWrite = writeBytes
		return 0, 0
	}

	readSpeed := float64(readBytes-a.lastDiskRead) / seconds
	writeSpeed := float64(writeBytes-a.lastDiskWrite) / seconds

	a.lastDiskAt = now
	a.lastDiskRead = readBytes
	a.lastDiskWrite = writeBytes

	return readSpeed, writeSpeed
}

func (a *Agent) collectProcesses(limit int) ([]map[string]any, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	type procInfo struct {
		PID  int32
		Name string
		CPU  float64
		Mem  float32
	}

	items := make([]procInfo, 0, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		cpuPercent, _ := p.CPUPercent()
		memPercent, _ := p.MemoryPercent()
		items = append(items, procInfo{
			PID:  p.Pid,
			Name: name,
			CPU:  cpuPercent,
			Mem:  memPercent,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CPU > items[j].CPU
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"pid":  item.PID,
			"name": item.Name,
			"cpu":  item.CPU,
			"mem":  item.Mem,
		})
	}

	return result, nil
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func (a *Agent) openTerminal(sessionID string, rows, cols int) error {
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}

	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "bash"
	}

	cmd := exec.Command(shell)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return err
	}

	a.termMu.Lock()
	if existing := a.terminals[sessionID]; existing != nil {
		_ = existing.ptyFile.Close()
		if existing.cmd.Process != nil {
			_ = existing.cmd.Process.Kill()
		}
	}
	a.terminals[sessionID] = &terminalSession{cmd: cmd, ptyFile: ptmx}
	a.termMu.Unlock()

	go a.streamTerminalOutput(sessionID, ptmx)
	return nil
}

func (a *Agent) streamTerminalOutput(sessionID string, ptmx *os.File) {
	buf := make([]byte, 4096)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			_ = a.sendJSON(map[string]any{
				"type":       "terminal_output",
				"session_id": sessionID,
				"data":       string(buf[:n]),
			})
		}
		if err != nil {
			a.closeTerminal(sessionID)
			return
		}
	}
}

func (a *Agent) writeTerminalInput(sessionID, data string) error {
	a.termMu.RLock()
	session := a.terminals[sessionID]
	a.termMu.RUnlock()
	if session == nil {
		return errors.New("terminal session not found")
	}
	_, err := session.ptyFile.Write([]byte(data))
	return err
}

func (a *Agent) resizeTerminal(sessionID string, rows, cols int) error {
	if rows <= 0 || cols <= 0 {
		return errors.New("invalid resize dimensions")
	}
	if rows > 1000 || cols > 1000 {
		return errors.New("resize dimensions are too large")
	}

	a.termMu.RLock()
	session := a.terminals[sessionID]
	a.termMu.RUnlock()
	if session == nil {
		return errors.New("terminal session not found")
	}

	return pty.Setsize(session.ptyFile, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (a *Agent) closeTerminal(sessionID string) {
	a.termMu.Lock()
	session := a.terminals[sessionID]
	delete(a.terminals, sessionID)
	a.termMu.Unlock()
	if session == nil {
		return
	}

	_ = session.ptyFile.Close()
	if session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}
}

func (a *Agent) closeAllTerminals() {
	a.termMu.Lock()
	all := a.terminals
	a.terminals = make(map[string]*terminalSession)
	a.termMu.Unlock()

	for _, session := range all {
		_ = session.ptyFile.Close()
		if session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}
	}
}

func (a *Agent) setDeployRuntime(deployID uint, runtime deployRuntime) {
	a.deployMu.Lock()
	a.activeDeploys[deployID] = runtime
	a.deployMu.Unlock()
}

func (a *Agent) clearDeployRuntime(deployID uint) {
	a.deployMu.Lock()
	delete(a.activeDeploys, deployID)
	a.deployMu.Unlock()
}

func (a *Agent) stopDeploy(deployID uint) error {
	a.deployMu.Lock()
	runtime, ok := a.activeDeploys[deployID]
	if ok {
		delete(a.activeDeploys, deployID)
	}
	a.deployMu.Unlock()

	candidates := map[string]struct{}{
		fmt.Sprintf("deploy_%d", deployID): {},
	}
	if runtime.ContainerName != "" {
		candidates[runtime.ContainerName] = struct{}{}
	}

	legacyNames, err := queryDeployContainerNames(deployID)
	if err != nil {
		logWarn("stop deploy: container discovery failed deploy_id=%d error=%v", deployID, err)
	} else {
		for _, name := range legacyNames {
			candidates[name] = struct{}{}
		}
	}

	var firstErr error
	for _, name := range mapKeysSorted(candidates) {
		logInfo("stop deploy: stopping container deploy_id=%d name=%s", deployID, name)
		if err := cleanupAndRemoveContainer(context.Background(), 90*time.Second, "", name, nil); err != nil {
			logWarn("stop deploy: failed to cleanup container deploy_id=%d name=%s error=%v", deployID, name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (a *Agent) runDeploy(payload deployPayload) {
	payload.normalize()
	a.clearDeployRuntime(payload.DeployID)

	const stepTimeout = 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	assignedPort := 0
	var deployLogBuilder strings.Builder

	branch := strings.TrimSpace(payload.Branch)
	if branch == "" {
		branch = "main"
	}
	requestedType := strings.ToLower(strings.TrimSpace(payload.ProjectType))
	subdirectory := strings.TrimSpace(payload.Subdirectory)
	envVars := payload.normalizedEnvVars()

	logInfo("deploy started: deploy_id=%d repo=%q branch=%q subdirectory=%q type=%q", payload.DeployID, payload.RepoURL, branch, subdirectory, requestedType)

	logf := func(line string, isErr bool) {
		if strings.TrimSpace(line) == "" {
			return
		}
		if isErr {
			logError("deploy %d: %s", payload.DeployID, line)
		} else {
			logInfo("deploy %d: %s", payload.DeployID, line)
		}
		deployLogBuilder.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			deployLogBuilder.WriteByte('\n')
		}
		a.sendDeployLog(payload.DeployID, line, isErr)
	}
	finish := func(success bool, deployURL, errText string) {
		status := "error"
		if success {
			status = "success"
		} else {
			a.clearDeployRuntime(payload.DeployID)
		}

		if success {
			logSuccess("deploy finished: deploy_id=%d url=%s port=%d", payload.DeployID, deployURL, assignedPort)
		} else {
			logError("deploy failed: deploy_id=%d error=%s", payload.DeployID, strings.TrimSpace(errText))
		}

		_ = a.sendJSON(map[string]any{
			"type":     "deploy_result",
			"deployId": payload.DeployID,
			"status":   status,
			"url":      deployURL,
			"port":     assignedPort,
			"log":      deployLogBuilder.String(),
			"error":    errText,
		})
		_ = a.sendJSON(map[string]any{
			"type":      "deploy_complete",
			"deploy_id": payload.DeployID,
			"success":   success,
			"url":       deployURL,
			"error":     errText,
		})
	}
	fail := func(step string, commandOutput string, stepErr error) {
		reason := step
		if stepErr != nil {
			reason = fmt.Sprintf("%s: %v", step, stepErr)
		}
		finish(false, "", formatDeployError(reason, commandOutput))
	}

	workDir, err := os.MkdirTemp("", fmt.Sprintf("novex-deploy-%d-*", payload.DeployID))
	if err != nil {
		finish(false, "", err.Error())
		return
	}
	defer func() {
		if removeErr := os.RemoveAll(workDir); removeErr != nil {
			logWarn("cleanup deploy workdir failed: deploy_id=%d dir=%s error=%v", payload.DeployID, workDir, removeErr)
		}
	}()

	sourceDir := filepath.Join(workDir, "src")
	switch strings.ToLower(strings.TrimSpace(payload.Source)) {
	case "github":
		if strings.TrimSpace(payload.RepoURL) == "" {
			finish(false, "", "repo_url is required")
			return
		}
		cloneOutput, runErr := runDeployCommand(ctx, stepTimeout, workDir, logf, "git", "clone", "--depth", "1", "--branch", branch, payload.RepoURL, "src")
		if runErr != nil {
			fail("clone failed", cloneOutput, runErr)
			return
		}
	case "zip":
		if strings.TrimSpace(payload.ZipData) == "" {
			finish(false, "", "zip_data is required")
			return
		}
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			finish(false, "", err.Error())
			return
		}
		if err := unzipBase64(payload.ZipData, sourceDir); err != nil {
			finish(false, "", err.Error())
			return
		}
	default:
		finish(false, "", "unsupported source")
		return
	}

	repoDir := normalizeSourceRoot(sourceDir)
	effectiveDir := repoDir
	if subdirectory != "" {
		resolvedDir, resolveErr := resolveDeployWorkDir(repoDir, subdirectory)
		if resolveErr != nil {
			finish(false, "", resolveErr.Error())
			return
		}
		effectiveDir = resolvedDir
		logf(fmt.Sprintf("using subdirectory: %s", effectiveDir), false)
	}

	projectType := resolveDeployProjectType(requestedType, effectiveDir)
	if !isSupportedProjectType(projectType) {
		finish(false, "", fmt.Sprintf("unsupported project type: %s", projectType))
		return
	}
	logf(fmt.Sprintf("detected project type: %s", projectType), false)

	frontendBuildRequired := isFrontendProjectType(projectType)
	if projectType == "static" {
		shouldBuildStatic, detectErr := shouldBuildStaticProject(effectiveDir)
		if detectErr != nil {
			fail("read package.json scripts failed", "", detectErr)
			return
		}
		if shouldBuildStatic {
			frontendBuildRequired = true
			logf("static project has package.json with build script, running npm install and npm run build", false)
		}
	}

	runFrontendBuild := func(contextLabel string) (string, string, error) {
		hasBuild, buildErr := hasBuildScript(effectiveDir)
		if buildErr != nil {
			return "read package.json scripts failed", "", buildErr
		}
		if !hasBuild {
			return "npm run build failed", "", errors.New("package.json is missing scripts.build")
		}

		logf(fmt.Sprintf("%s: installing dependencies (npm install)", contextLabel), false)
		output, runErr := runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "npm", "install")
		if runErr != nil {
			return "npm install failed", output, runErr
		}

		logf(fmt.Sprintf("%s: building static assets (npm run build)", contextLabel), false)
		output, runErr = runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "npm", "run", "build")
		if runErr != nil {
			return "npm run build failed", output, runErr
		}

		return "", "", nil
	}

	customBuild := strings.TrimSpace(payload.BuildCommand)
	if customBuild == "" {
		switch projectType {
		case "go":
			output, runErr := runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "go", "mod", "download")
			if runErr != nil {
				fail("go mod download failed", output, runErr)
				return
			}

			output, runErr = runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "go", "build", "-o", "app", ".")
			if runErr != nil {
				fail("go build failed", output, runErr)
				return
			}
		case "node":
			logf("node project: installing dependencies (npm install)", false)
			output, runErr := runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "npm", "install")
			if runErr != nil {
				fail("npm install failed", output, runErr)
				return
			}

			hasBuild, buildErr := hasBuildScript(effectiveDir)
			if buildErr != nil {
				fail("read package.json scripts failed", "", buildErr)
				return
			}
			if hasBuild {
				logf("node project: build script found, running npm run build", false)
				output, runErr = runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "npm", "run", "build")
				if runErr != nil {
					fail("npm run build failed", output, runErr)
					return
				}
			}
		case "python":
			logf("python project: build step is not required", false)
		case "vite", "react", "vue", "svelte":
			step, output, runErr := runFrontendBuild(fmt.Sprintf("%s project", projectType))
			if runErr != nil {
				fail(step, output, runErr)
				return
			}
		case "static":
			if frontendBuildRequired {
				step, output, runErr := runFrontendBuild("static project with build script")
				if runErr != nil {
					fail(step, output, runErr)
					return
				}
			} else {
				logf("static project: build step is not required", false)
			}
		}
	} else {
		output, runErr := runDeployCommand(ctx, stepTimeout, effectiveDir, logf, "bash", "-lc", customBuild)
		if runErr != nil {
			fail("custom build failed", output, runErr)
			return
		}
	}

	containerProjectType, staticServeDir, resolveErr := resolveContainerProjectType(projectType, effectiveDir, payload.OutputDir)
	if resolveErr != nil {
		finish(false, "", resolveErr.Error())
		return
	}
	logf(fmt.Sprintf("container runtime type: %s", containerProjectType), false)
	if staticServeDir != "" {
		logf(fmt.Sprintf("container static directory: %s", staticServeDir), false)
	}

	appPort := detectContainerAppPort(containerProjectType, effectiveDir, envVars)
	if appPort <= 0 || appPort > 65535 {
		finish(false, "", fmt.Sprintf("invalid app port: %d", appPort))
		return
	}
	if (containerProjectType == "go" || containerProjectType == "node") && strings.TrimSpace(envVars["PORT"]) == "" {
		envVars["PORT"] = strconv.Itoa(appPort)
		logf(fmt.Sprintf("PORT is not provided, injecting PORT=%d", appPort), false)
	}
	logf(fmt.Sprintf("detected container app port: %d", appPort), false)

	port, err := findFreePort()
	if err != nil {
		finish(false, "", err.Error())
		return
	}
	assignedPort = port
	logf(fmt.Sprintf("assigned external port: %d", assignedPort), false)

	imageTag, buildErr := a.buildDockerImage(ctx, stepTimeout, payload.DeployID, effectiveDir, containerProjectType, staticServeDir, appPort, envVars, logf)
	if buildErr != nil {
		fail("docker build failed", "", buildErr)
		return
	}

	containerName, runErr := a.runContainer(ctx, stepTimeout, payload.DeployID, effectiveDir, imageTag, assignedPort, appPort, envVars, logf)
	if runErr != nil {
		fail("docker run failed", "", runErr)
		return
	}

	a.setDeployRuntime(payload.DeployID, deployRuntime{ContainerName: containerName})

	url := fmt.Sprintf("http://%s:%d", detectLocalIP(), port)
	finish(true, url, "")
}

func (a *Agent) buildDockerImage(parentCtx context.Context, stepTimeout time.Duration, deployID uint, workDir, projectType, staticServeDir string, appPort int, envVars map[string]string, logf func(string, bool)) (string, error) {
	imageTag := fmt.Sprintf("deploy_%d:latest", deployID)

	if fileExists(filepath.Join(workDir, "Dockerfile")) {
		if projectType != "static" {
			logf("repository Dockerfile detected, using it for image build", false)
			if _, err := runDeployCommand(parentCtx, stepTimeout, workDir, logf, "docker", "build", "-t", imageTag, "."); err != nil {
				return "", err
			}
			return imageTag, nil
		}
		logf("static deployment: repository Dockerfile ignored, generating nginx:alpine image", false)
	}

	if projectType == "static" {
		if strings.TrimSpace(staticServeDir) == "" {
			return "", errors.New("static output directory is required for nginx image")
		}
		logf(fmt.Sprintf("copying static build artifacts from %s into nginx:alpine image", staticServeDir), false)
	}

	var staticNginxConfRel string
	if projectType == "static" {
		rel, cleanupNginx, prepErr := prepareStaticNginxConfForDocker(workDir, logf)
		if prepErr != nil {
			return "", prepErr
		}
		staticNginxConfRel = rel
		if cleanupNginx != nil {
			defer cleanupNginx()
		}
	}

	nodeBaseImage := resolveNodeBaseImage()
	if projectType == "node" || projectType == "vite" {
		logf(fmt.Sprintf("using Node base image for docker build: %s", nodeBaseImage), false)
	}
	viteBuildArgKeys := collectViteBuildArgKeys(envVars)
	if projectType == "vite" && len(viteBuildArgKeys) > 0 {
		logf(fmt.Sprintf("vite docker build: forwarding build-time env vars: %s", strings.Join(viteBuildArgKeys, ", ")), false)
	}

	dockerfileContent, err := generateDockerfile(projectType, workDir, staticServeDir, appPort, staticNginxConfRel, nodeBaseImage, viteBuildArgKeys)
	if err != nil {
		return "", err
	}

	generatedDockerfilePath := filepath.Join(workDir, ".novex.generated.Dockerfile")
	if err := os.WriteFile(generatedDockerfilePath, []byte(dockerfileContent), 0o644); err != nil {
		return "", err
	}
	defer func() {
		if removeErr := os.Remove(generatedDockerfilePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			logWarn("cleanup generated Dockerfile failed: path=%s error=%v", generatedDockerfilePath, removeErr)
		}
	}()

	buildArgs := []string{"build", "-f", generatedDockerfilePath, "-t", imageTag}
	if projectType == "vite" {
		for _, key := range viteBuildArgKeys {
			buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", key, envVars[key]))
		}
	}
	buildArgs = append(buildArgs, ".")

	logf(fmt.Sprintf("generated Dockerfile: %s", generatedDockerfilePath), false)
	if _, err := runDeployCommand(parentCtx, stepTimeout, workDir, logf, "docker", buildArgs...); err != nil {
		return "", err
	}

	return imageTag, nil
}

func (a *Agent) runContainer(parentCtx context.Context, stepTimeout time.Duration, deployID uint, workDir, imageTag string, hostPort, appPort int, envVars map[string]string, logf func(string, bool)) (string, error) {
	if hostPort <= 0 || hostPort > 65535 {
		return "", fmt.Errorf("invalid host port: %d", hostPort)
	}
	if appPort <= 0 || appPort > 65535 {
		return "", fmt.Errorf("invalid app port: %d", appPort)
	}

	containerName := fmt.Sprintf("deploy_%d", deployID)
	if err := cleanupAndRemoveContainer(parentCtx, stepTimeout, workDir, containerName, logf); err != nil {
		return "", err
	}
	logf(fmt.Sprintf("starting container %s: %d -> %d", containerName, hostPort, appPort), false)
	if appPort == 80 {
		logf("container internal port is 80 (nginx/static runtime)", false)
	}
	memoryLimit := strings.TrimSpace(os.Getenv("NOVEX_DEPLOY_MEMORY_LIMIT"))
	if memoryLimit == "" {
		memoryLimit = "512m"
	}
	cpusLimit := strings.TrimSpace(os.Getenv("NOVEX_DEPLOY_CPUS"))
	if cpusLimit == "" {
		cpusLimit = "1.0"
	}
	pidsLimit := strings.TrimSpace(os.Getenv("NOVEX_DEPLOY_PIDS_LIMIT"))
	if pidsLimit == "" {
		pidsLimit = "256"
	}

	runArgs := []string{
		"run", "-d", "--name", containerName,
		"--security-opt", "no-new-privileges:true",
		"--memory", memoryLimit,
		"--cpus", cpusLimit,
		"--pids-limit", pidsLimit,
		"--cap-drop", "ALL",
		"-p", fmt.Sprintf("%d:%d", hostPort, appPort),
	}
	if appPort <= 1024 {
		runArgs = append(runArgs, "--cap-add", "NET_BIND_SERVICE")
	}
	for _, key := range sortedEnvKeys(envVars) {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", key, envVars[key]))
	}
	runArgs = append(runArgs, imageTag)

	if _, err := runDeployCommand(parentCtx, stepTimeout, workDir, logf, "docker", runArgs...); err != nil {
		return "", err
	}

	return containerName, nil
}

func cleanupAndRemoveContainer(parentCtx context.Context, timeout time.Duration, workDir, containerName string, logf func(string, bool)) error {
	containerName = strings.TrimSpace(containerName)
	if containerName == "" {
		return nil
	}

	for _, action := range []string{"stop", "rm"} {
		commandLine := fmt.Sprintf("docker %s %s", action, containerName)
		if logf != nil {
			logf(fmt.Sprintf("running: %s in %s", commandLine, workDir), false)
		}

		stepCtx, cancel := context.WithTimeout(parentCtx, timeout)
		combinedOutput, stderrOutput, err := runCommandContextDetailed(stepCtx, workDir, "docker", action, containerName)
		cancel()

		if logf != nil {
			logDeployOutput(logf, combinedOutput, err != nil)
		}
		if err != nil {
			errText := strings.ToLower(strings.TrimSpace(stderrOutput + "\n" + combinedOutput))
			if strings.Contains(errText, "no such container") || strings.Contains(errText, "is not running") {
				continue
			}
			return fmt.Errorf("%s failed: %w", commandLine, err)
		}
	}

	return nil
}

func queryDeployContainerNames(deployID uint) ([]string, error) {
	filters := []string{
		fmt.Sprintf("name=deploy_%d", deployID),
		fmt.Sprintf("name=deploy-%d-", deployID),
	}

	names := make(map[string]struct{})
	for _, filter := range filters {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, _, err := runCommandContextDetailed(ctx, "", "docker", "ps", "-a", "--filter", filter, "--format", "{{.Names}}")
		cancel()
		if err != nil {
			return nil, err
		}

		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			names[name] = struct{}{}
		}
	}

	return mapKeysSorted(names), nil
}

func resolveContainerProjectType(projectType, workDir, outputDir string) (string, string, error) {
	switch projectType {
	case "go":
		return "go", "", nil
	case "vite", "react", "vue", "svelte":
		if projectType == "vite" {
			return "vite", "", nil
		}
		staticDir, err := resolveStaticAssetsDir(workDir, outputDir)
		if err != nil {
			return "", "", err
		}
		return "static", staticDir, nil
	case "node":
		if strings.TrimSpace(outputDir) != "" {
			staticDir, err := resolveStaticAssetsDir(workDir, outputDir)
			if err != nil {
				return "", "", err
			}
			return "static", staticDir, nil
		}

		hasStart, err := hasPackageScript(workDir, "start")
		if err != nil {
			return "", "", fmt.Errorf("read package.json scripts failed: %w", err)
		}
		hasBuild, err := hasPackageScript(workDir, "build")
		if err != nil {
			return "", "", fmt.Errorf("read package.json scripts failed: %w", err)
		}

		autoStaticDir := detectDefaultStaticAssetsDir(workDir)
		if !hasStart {
			if autoStaticDir == "" {
				return "", "", errors.New("node project has neither start script nor static output directory")
			}
			return "static", autoStaticDir, nil
		}

		if hasBuild && autoStaticDir != "" && isLikelyFrontendNodeProject(workDir) {
			return "static", autoStaticDir, nil
		}
		return "node", "", nil
	case "python":
		return "python", "", nil
	case "static":
		staticDir, err := resolveStaticAssetsDir(workDir, outputDir)
		if err != nil {
			return "", "", err
		}
		return "static", staticDir, nil
	default:
		return "", "", fmt.Errorf("unsupported project type: %s", projectType)
	}
}

func resolveStaticAssetsDir(workDir, outputDir string) (string, error) {
	if resolved := resolveOutputDir(workDir, outputDir); resolved != "" {
		if !dirExists(resolved) {
			return "", fmt.Errorf("output directory not found: %s", resolved)
		}
		return resolved, nil
	}

	if autoDir := detectDefaultStaticAssetsDir(workDir); autoDir != "" {
		return autoDir, nil
	}

	return "", errors.New("static output directory not found (expected dist, build, .output, out or public)")
}

func detectDefaultStaticAssetsDir(workDir string) string {
	for _, candidate := range []string{"dist", "build", ".output", "out", "public"} {
		candidateDir := filepath.Join(workDir, candidate)
		if dirExists(candidateDir) {
			return candidateDir
		}
	}

	if fileExists(filepath.Join(workDir, "index.html")) {
		return workDir
	}

	return ""
}

func isLikelyFrontendNodeProject(workDir string) bool {
	content, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		return false
	}

	var parsed struct {
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &parsed); err != nil {
		return false
	}

	frontendDeps := []string{"react", "vue", "vite", "@angular/core", "svelte", "solid-js", "preact"}
	for _, dep := range frontendDeps {
		if _, ok := parsed.Dependencies[dep]; ok {
			return true
		}
		if _, ok := parsed.DevDependencies[dep]; ok {
			return true
		}
	}

	return false
}

func detectContainerAppPort(projectType, workDir string, envVars map[string]string) int {
	if port, ok := parseValidPort(envVars["PORT"]); ok {
		return port
	}

	switch projectType {
	case "static":
		return 80
	case "vite":
		return 80
	case "go":
		goPatterns := []*regexp.Regexp{
			regexp.MustCompile(`(?i)ListenAndServe\(\s*"(?:[^":]*:)?(\d{2,5})"`),
			regexp.MustCompile(`(?i)\.Run\(\s*"(?:[^":]*:)?(\d{2,5})"`),
			regexp.MustCompile(`(?i)PORT[^0-9\n]{0,20}(\d{2,5})`),
		}
		if port := detectPortInSourceFiles(workDir, map[string]struct{}{".go": {}}, goPatterns); port > 0 {
			return port
		}
		return 8080
	case "node":
		if port := detectNodePortFromScripts(workDir); port > 0 {
			return port
		}
		nodePatterns := []*regexp.Regexp{
			regexp.MustCompile(`process\.env\.PORT\s*\|\|\s*(\d{2,5})`),
			regexp.MustCompile(`\.listen\(\s*(\d{2,5})`),
			regexp.MustCompile(`(?i)PORT[^0-9\n]{0,20}(\d{2,5})`),
		}
		if port := detectPortInSourceFiles(workDir, map[string]struct{}{".js": {}, ".mjs": {}, ".cjs": {}, ".ts": {}, ".tsx": {}, ".jsx": {}}, nodePatterns); port > 0 {
			return port
		}
		return 3000
	case "python":
		return 8000
	default:
		return 80
	}
}

func detectNodePortFromScripts(workDir string) int {
	content, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		return 0
	}

	var parsed struct {
		Scripts map[string]any `json:"scripts"`
	}
	if err := json.Unmarshal(content, &parsed); err != nil {
		return 0
	}

	flagPattern := regexp.MustCompile(`(?:--port|-p)\s+(\d{2,5})`)
	for _, scriptName := range []string{"start", "serve", "dev"} {
		raw, ok := parsed.Scripts[scriptName]
		if !ok {
			continue
		}
		scriptValue := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if scriptValue == "" {
			continue
		}
		matches := flagPattern.FindStringSubmatch(scriptValue)
		if len(matches) < 2 {
			continue
		}
		if port, ok := parseValidPort(matches[1]); ok {
			return port
		}
	}

	return 0
}

func detectPortInSourceFiles(workDir string, extensions map[string]struct{}, patterns []*regexp.Regexp) int {
	if len(patterns) == 0 {
		return 0
	}

	stopWalkErr := errors.New("stop walk")
	detectedPort := 0
	filesScanned := 0

	walkErr := filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "build", ".next", ".nuxt", "vendor":
				return filepath.SkipDir
			}
			return nil
		}

		if len(extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := extensions[ext]; !ok {
				return nil
			}
		}

		filesScanned++
		if filesScanned > 300 {
			return stopWalkErr
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		text := string(content)
		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(text)
			if len(matches) < 2 {
				continue
			}
			if port, ok := parseValidPort(matches[1]); ok {
				detectedPort = port
				return stopWalkErr
			}
		}

		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, stopWalkErr) {
		return 0
	}

	return detectedPort
}

func parseValidPort(raw string) (int, bool) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	if port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func relativePathWithinBase(baseDir, targetDir string) (string, error) {
	base := filepath.Clean(baseDir)
	target := filepath.Clean(targetDir)

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return ".", nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %s is outside build context %s", targetDir, baseDir)
	}

	return filepath.ToSlash(rel), nil
}

const defaultStaticNginxConf = `server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}
`

const generatedStaticNginxConfFile = ".novex.generated.nginx.conf"

// prepareStaticNginxConfForDocker returns a path relative to workDir suitable for COPY in the build context.
// If nginx.conf exists at the repo root, it is used; otherwise a default SPA-friendly config is written to
// generatedStaticNginxConfFile. When the second return value is non-nil, it removes the generated file.
func prepareStaticNginxConfForDocker(workDir string, logf func(string, bool)) (relPath string, cleanup func(), err error) {
	repoNginx := filepath.Join(workDir, "nginx.conf")
	if fileExists(repoNginx) {
		rel, relErr := relativePathWithinBase(workDir, repoNginx)
		if relErr != nil {
			return "", nil, relErr
		}
		logf("static SPA: using nginx.conf from repository root for nginx:alpine image", false)
		return rel, nil, nil
	}

	genPath := filepath.Join(workDir, generatedStaticNginxConfFile)
	if err := os.WriteFile(genPath, []byte(defaultStaticNginxConf), 0o644); err != nil {
		return "", nil, fmt.Errorf("write generated nginx.conf: %w", err)
	}
	logf("static SPA: generated default nginx.conf with try_files for client-side routing (F5 on deep links)", false)
	return generatedStaticNginxConfFile, func() {
		_ = os.Remove(genPath)
	}, nil
}

func generateDockerfile(projectType, workDir, staticServeDir string, appPort int, staticNginxConfRel, nodeBaseImage string, viteBuildArgKeys []string) (string, error) {
	if appPort <= 0 || appPort > 65535 {
		return "", fmt.Errorf("invalid app port: %d", appPort)
	}

	switch projectType {
	case "go":
		goSumCopy := ""
		if fileExists(filepath.Join(workDir, "go.sum")) {
			goSumCopy = "COPY go.sum ./\n"
		}
		return fmt.Sprintf(`FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod ./
%sRUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app .

FROM alpine:3.20
WORKDIR /app
RUN adduser -D appuser
COPY --from=builder /out/app /app/app
EXPOSE %d
USER appuser
ENTRYPOINT ["/app/app"]
`, goSumCopy, appPort), nil
	case "node":
		if strings.TrimSpace(nodeBaseImage) == "" {
			nodeBaseImage = "node:22-alpine"
		}
		return fmt.Sprintf(`FROM %s
WORKDIR /app
COPY package*.json ./
RUN npm install --omit=dev
COPY . .
RUN node --version
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && chown -R appuser:appgroup /app
USER appuser
ENV PORT=%d
EXPOSE %d
CMD ["npm", "start"]
`, nodeBaseImage, appPort, appPort), nil
	case "vite":
		if strings.TrimSpace(nodeBaseImage) == "" {
			nodeBaseImage = "node:22-alpine"
		}
		var viteArgs strings.Builder
		for _, key := range viteBuildArgKeys {
			cleanKey := strings.TrimSpace(key)
			if cleanKey == "" {
				continue
			}
			viteArgs.WriteString(fmt.Sprintf("ARG %s\n", cleanKey))
			viteArgs.WriteString(fmt.Sprintf("ENV %s=$%s\n", cleanKey, cleanKey))
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY . .
%sRUN node --version
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
`, nodeBaseImage, viteArgs.String()), nil
	case "static":
		if strings.TrimSpace(staticServeDir) == "" {
			return "", errors.New("static output directory is required for static container")
		}
		if strings.TrimSpace(staticNginxConfRel) == "" {
			return "", errors.New("nginx config path is required for static container")
		}
		relStaticDir, err := relativePathWithinBase(workDir, staticServeDir)
		if err != nil {
			return "", err
		}
		relNginxConf, err := relativePathWithinBase(workDir, filepath.Join(workDir, staticNginxConfRel))
		if err != nil {
			return "", err
		}

		copyPath := "./"
		if relStaticDir != "." {
			copyPath = strings.TrimSuffix(relStaticDir, "/") + "/"
		}

		return fmt.Sprintf(`FROM nginx:alpine
COPY %s /etc/nginx/conf.d/default.conf
COPY %s /usr/share/nginx/html/
EXPOSE 80
`, filepath.ToSlash(relNginxConf), filepath.ToSlash(copyPath)), nil
	case "python":
		return fmt.Sprintf(`FROM python:3-alpine
WORKDIR /app
COPY . .
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && chown -R appuser:appgroup /app
USER appuser
EXPOSE %d
CMD ["python", "-m", "http.server", "%d", "--bind", "0.0.0.0"]
`, appPort, appPort), nil
	default:
		return "", fmt.Errorf("unsupported project type for Docker build: %s", projectType)
	}
}

func resolveNodeBaseImage() string {
	if legacy := strings.ToLower(strings.TrimSpace(os.Getenv("NOVEX_USE_LEGACY_NODE_ALPINE"))); legacy == "1" || legacy == "true" || legacy == "yes" {
		return "node:alpine"
	}
	if custom := strings.TrimSpace(os.Getenv("NOVEX_NODE_BASE_IMAGE")); custom != "" {
		return custom
	}
	return "node:22-alpine"
}

func collectViteBuildArgKeys(envVars map[string]string) []string {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		trimmed := strings.TrimSpace(key)
		if strings.HasPrefix(trimmed, "VITE_") {
			keys = append(keys, trimmed)
		}
	}
	sort.Strings(keys)
	return keys
}

func mapKeysSorted(items map[string]struct{}) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedEnvKeys(envVars map[string]string) []string {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (a *Agent) sendDeployLog(deployID uint, line string, isErr bool) {
	stream := "stdout"
	if isErr {
		stream = "stderr"
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)

	_ = a.sendJSON(map[string]any{
		"type":      "deploy_log",
		"deploy_id": deployID,
		"line":      line,
		"stream":    stream,
		"timestamp": timestamp,
		"is_error":  isErr,
	})
}

func (a *Agent) runLoggedCommand(ctx context.Context, deployID uint, dir string, env []string, onLine func(string, bool), name string, args ...string) error {
	commandLine := "$ " + name + " " + strings.Join(args, " ")
	if onLine != nil {
		onLine(commandLine, false)
	} else {
		a.sendDeployLog(deployID, commandLine, false)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	pump := func(reader io.Reader, isErr bool) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if onLine != nil {
				onLine(line, isErr)
			} else {
				a.sendDeployLog(deployID, line, isErr)
			}
		}
	}

	wg.Add(2)
	go pump(stdout, false)
	go pump(stderr, true)

	waitErr := cmd.Wait()
	wg.Wait()
	if waitErr != nil {
		return waitErr
	}
	return nil
}

func runDeployCommand(parentCtx context.Context, timeout time.Duration, dir string, logf func(string, bool), name string, args ...string) (string, error) {
	commandLine := strings.TrimSpace(name + " " + strings.Join(args, " "))
	logf(fmt.Sprintf("running: %s in %s", commandLine, dir), false)

	stepCtx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	combinedOutput, stderrOutput, err := runCommandContextDetailed(stepCtx, dir, name, args...)
	logDeployOutput(logf, combinedOutput, err != nil)
	if err != nil {
		errorOutput := strings.TrimSpace(stderrOutput)
		if errorOutput == "" {
			errorOutput = strings.TrimSpace(combinedOutput)
		}
		return errorOutput, fmt.Errorf("%s failed in %s: %w", commandLine, dir, err)
	}

	return combinedOutput, nil
}

func detectProjectType(workDir string) string {
	if fileExists(filepath.Join(workDir, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(workDir, "package.json")) {
		if frontendType := detectFrontendFrameworkType(workDir); frontendType != "" {
			return frontendType
		}
		return "node"
	}
	if fileExists(filepath.Join(workDir, "requirements.txt")) {
		return "python"
	}
	return "static"
}

func detectFrontendFrameworkType(workDir string) string {
	content, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		return ""
	}

	var parsed struct {
		Scripts         map[string]any `json:"scripts"`
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &parsed); err != nil {
		return ""
	}

	hasScript := func(name string) bool {
		_, ok := parsed.Scripts[name]
		return ok
	}
	scriptContains := func(name, part string) bool {
		raw, ok := parsed.Scripts[name]
		if !ok {
			return false
		}
		value := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
		if value == "" {
			return false
		}
		return strings.Contains(value, strings.ToLower(strings.TrimSpace(part)))
	}
	hasDependency := func(names ...string) bool {
		for _, name := range names {
			if _, ok := parsed.Dependencies[name]; ok {
				return true
			}
			if _, ok := parsed.DevDependencies[name]; ok {
				return true
			}
		}
		return false
	}

	hasBuild := hasScript("build")
	hasStart := hasScript("start")

	if hasDependency("vite") || scriptContains("build", "vite") || scriptContains("dev", "vite") {
		return "vite"
	}

	if hasBuild && !hasStart {
		switch {
		case hasDependency("react", "react-dom"):
			return "react"
		case hasDependency("vue"):
			return "vue"
		case hasDependency("svelte", "@sveltejs/kit"):
			return "svelte"
		}
	}

	return ""
}

func isFrontendProjectType(projectType string) bool {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "vite", "react", "vue", "svelte":
		return true
	default:
		return false
	}
}

func resolveDeployProjectType(requested, workDir string) string {
	projectType := strings.ToLower(strings.TrimSpace(requested))
	if projectType == "" || projectType == "auto" {
		return detectProjectType(workDir)
	}
	switch projectType {
	case "react", "vite", "vue", "svelte":
		return projectType
	case "frontend", "spa":
		return "static"
	case "nodejs", "node.js":
		return "node"
	case "reactjs":
		return "react"
	case "vuejs":
		return "vue"
	case "sveltekit":
		return "svelte"
	}
	if projectType == "python3" {
		return "python"
	}
	return projectType
}

func isSupportedProjectType(projectType string) bool {
	switch projectType {
	case "go", "node", "python", "static", "vite", "react", "vue", "svelte":
		return true
	default:
		return false
	}
}

func resolveDeployWorkDir(repoDir, subdirectory string) (string, error) {
	subdirectory = strings.TrimSpace(subdirectory)
	if subdirectory == "" {
		return repoDir, nil
	}

	normalized := strings.ReplaceAll(subdirectory, "\\", "/")
	normalized = filepath.Clean(normalized)
	if normalized == "." {
		return repoDir, nil
	}
	if filepath.IsAbs(normalized) || normalized == ".." || strings.HasPrefix(normalized, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid subdirectory path: %s", subdirectory)
	}

	workDir := filepath.Join(repoDir, normalized)
	rel, err := filepath.Rel(repoDir, workDir)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("subdirectory escapes repository root: %s", subdirectory)
	}

	info, err := os.Stat(workDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("subdirectory does not exist: %s", subdirectory)
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("subdirectory is not a directory: %s", subdirectory)
	}

	return workDir, nil
}

func runCommand(dir, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return runCommandContext(ctx, dir, name, args...)
}

func runCommandContext(ctx context.Context, dir, name string, args ...string) (string, error) {
	combinedOutput, _, err := runCommandContextDetailed(ctx, dir, name, args...)
	return combinedOutput, err
}

func runCommandContextDetailed(ctx context.Context, dir, name string, args ...string) (string, string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", "", fmt.Errorf("command not found: %s", name)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()
	combinedOutput := strings.TrimSpace(stdoutBuffer.String() + stderrBuffer.String())
	stderrOutput := strings.TrimSpace(stderrBuffer.String())
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if deadline, ok := ctx.Deadline(); ok {
				return combinedOutput, stderrOutput, fmt.Errorf("command timeout at %s", deadline.Format(time.RFC3339))
			}
			return combinedOutput, stderrOutput, fmt.Errorf("command timeout")
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return combinedOutput, stderrOutput, fmt.Errorf("exit status %d", exitErr.ExitCode())
		}
		return combinedOutput, stderrOutput, err
	}

	return combinedOutput, stderrOutput, nil
}

func logDeployOutput(logf func(string, bool), output string, isErr bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		logf(scanner.Text(), isErr)
	}
}

func formatDeployError(reason, output string) string {
	trimmedReason := strings.TrimSpace(reason)
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return trimmedReason
	}
	return fmt.Sprintf("%s. output: %s", trimmedReason, truncateDeployErrorText(trimmedOutput, 500))
}

func truncateDeployErrorText(text string, maxLen int) string {
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func startDetachedChecked(dir string, env []string, name string, args ...string) (int, error) {
	if _, err := exec.LookPath(name); err != nil {
		return 0, fmt.Errorf("command not found: %s", name)
	}
	return startDetached(dir, env, name, args...)
}

func firstAvailableCommand(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("none of the required commands were found: %s", strings.Join(candidates, ", "))
}

func hasBuildScript(sourceDir string) (bool, error) {
	return hasPackageScript(sourceDir, "build")
}

func shouldBuildStaticProject(workDir string) (bool, error) {
	if !fileExists(filepath.Join(workDir, "package.json")) {
		return false, nil
	}
	hasBuild, err := hasBuildScript(workDir)
	if err != nil {
		return false, err
	}
	return hasBuild, nil
}

func hasPackageScript(sourceDir, scriptName string) (bool, error) {
	scriptName = strings.TrimSpace(scriptName)
	if scriptName == "" {
		return false, errors.New("script name is empty")
	}

	content, err := os.ReadFile(filepath.Join(sourceDir, "package.json"))
	if err != nil {
		return false, err
	}
	var parsed struct {
		Scripts map[string]any `json:"scripts"`
	}
	if err := json.Unmarshal(content, &parsed); err != nil {
		return false, err
	}
	_, ok := parsed.Scripts[scriptName]
	return ok, nil
}

func startDetached(dir string, env []string, name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	go cmd.Wait()
	return pid, nil
}

func unzipBase64(zipData, targetDir string) error {
	decoded, err := base64.StdEncoding.DecodeString(zipData)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(zipData)
		if err != nil {
			return err
		}
	}

	zr, err := zip.NewReader(bytes.NewReader(decoded), int64(len(decoded)))
	if err != nil {
		return err
	}

	basePath := filepath.Clean(targetDir) + string(os.PathSeparator)
	for _, f := range zr.File {
		path := filepath.Join(targetDir, f.Name)
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath+string(os.PathSeparator), basePath) && cleanPath != filepath.Clean(targetDir) {
			return fmt.Errorf("invalid zip path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rcCloseErr := rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if rcCloseErr != nil {
			return rcCloseErr
		}
	}
	return nil
}

func normalizeSourceRoot(sourceDir string) string {
	entries, err := os.ReadDir(sourceDir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return sourceDir
	}
	return filepath.Join(sourceDir, entries[0].Name())
}

func resolveOutputDir(sourceDir, outputDir string) string {
	trimmed := strings.TrimSpace(outputDir)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return ""
	}
	return filepath.Clean(filepath.Join(sourceDir, cleaned))
}

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func detectLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ipv4 := ip.To4()
			if ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return "127.0.0.1"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
