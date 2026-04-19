package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"novexpanel/backend/internal/auth"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

const (
	defaultTerminalCols = 80
	defaultTerminalRows = 24
)

type terminalEnvelope struct {
	Type string `json:"type"`
}

type terminalInputMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type terminalResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// handleTerminalWS adapts a standard net/http handler to Gin routing.
func (a *App) handleTerminalWS(c *gin.Context) {
	req := c.Request.Clone(c.Request.Context())
	req.SetPathValue("server_id", c.Param("id"))
	a.terminalWSHandler(c.Writer, req)
}

// terminalWSHandler upgrades the HTTP connection to WebSocket and proxies data
// between client and PTY-backed shell session.
func (a *App) terminalWSHandler(w http.ResponseWriter, r *http.Request) {
	debug := strings.TrimSpace(os.Getenv("NOVEXPANEL_TERMINAL_DEBUG")) == "1"

	tokenRaw := strings.TrimSpace(r.URL.Query().Get("token"))
	if tokenRaw == "" {
		writeTerminalError(w, http.StatusUnauthorized, "missing token")
		return
	}

	claims, err := auth.ParseUserToken(a.cfg.JWTSecret, tokenRaw)
	if err != nil {
		writeTerminalError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	serverID, err := parseTerminalServerID(r)
	if err != nil {
		writeTerminalError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	if _, err := a.requireServerForUser(claims.UserID, serverID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeTerminalError(w, http.StatusNotFound, "server not found")
			return
		}
		writeTerminalError(w, http.StatusInternalServerError, "unable to load server")
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("terminal ws upgrade error (server_id=%d): %v", serverID, err)
		return
	}
	defer conn.Close()
	if debug {
		log.Printf("terminal ws upgrade ok (server_id=%d)", serverID)
	}

	conn.SetReadLimit(1 << 20)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, selectShell())
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: defaultTerminalCols, Rows: defaultTerminalRows})
	if err != nil {
		log.Printf("terminal pty start error (server_id=%d): %v", serverID, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","error":"unable to start shell"}`))
		return
	}
	defer ptyFile.Close()
	if debug {
		pid := 0
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}
		log.Printf("terminal started shell (server_id=%d pid=%d shell=%q)", serverID, pid, cmd.Path)
	}

	processDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(processDone)
	}()
	defer stopTerminalProcess(cmd, processDone)

	ptyReadErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := ptyFile.Read(buf)
			if n > 0 {
				if debug {
					snippet := buf[:n]
					if len(snippet) > 80 {
						snippet = snippet[:80]
					}
					log.Printf("terminal pty read (server_id=%d n=%d data=%q)", serverID, n, snippet)
				}

				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, append([]byte(nil), buf[:n]...)); writeErr != nil {
					ptyReadErr <- writeErr
					return
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					ptyReadErr <- nil
					return
				}
				ptyReadErr <- readErr
				return
			}
		}
	}()

	for {
		select {
		case <-processDone:
			return
		case readErr := <-ptyReadErr:
			if readErr != nil && !isExpectedWSClose(readErr) {
				log.Printf("terminal pty->ws error (server_id=%d): %v", serverID, readErr)
			}
			return
		default:
		}

		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			if !isExpectedWSClose(err) {
				log.Printf("terminal ws read error (server_id=%d): %v", serverID, err)
			}
			return
		}
		if debug {
			log.Printf("terminal ws recv (server_id=%d type=%d bytes=%d)", serverID, msgType, len(payload))
		}

		switch msgType {
		case websocket.TextMessage:
			handled, handleErr := handleTerminalTextPayload(payload, ptyFile)
			if handleErr != nil {
				log.Printf("terminal text payload error (server_id=%d): %v", serverID, handleErr)
				return
			}
			if handled {
				continue
			}
			if _, writeErr := ptyFile.Write(payload); writeErr != nil {
				log.Printf("terminal ws->pty write error (server_id=%d): %v", serverID, writeErr)
				return
			}
			if debug {
				log.Printf("terminal ws->pty wrote (server_id=%d bytes=%d)", serverID, len(payload))
			}
		case websocket.BinaryMessage:
			if _, writeErr := ptyFile.Write(payload); writeErr != nil {
				log.Printf("terminal ws->pty write error (server_id=%d): %v", serverID, writeErr)
				return
			}
			if debug {
				log.Printf("terminal ws->pty wrote (server_id=%d bytes=%d)", serverID, len(payload))
			}
		case websocket.CloseMessage:
			return
		}
	}
}

func handleTerminalTextPayload(payload []byte, ptyFile *os.File) (bool, error) {
	if len(payload) == 0 {
		return true, nil
	}

	first := firstNonSpaceByteIndex(payload)
	if first < 0 || payload[first] != '{' {
		return false, nil
	}

	var envelope terminalEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false, nil
	}

	if envelope.Type == "" {
		return false, nil
	}

	switch envelope.Type {
	case "resize":
		var msg terminalResizeMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return true, err
		}
		if msg.Cols <= 0 || msg.Rows <= 0 {
			return true, errors.New("invalid resize dimensions")
		}
		if msg.Cols > 1000 || msg.Rows > 1000 {
			return true, errors.New("resize dimensions are too large")
		}
		return true, pty.Setsize(ptyFile, &pty.Winsize{Cols: uint16(msg.Cols), Rows: uint16(msg.Rows)})
	case "input":
		var msg terminalInputMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return true, err
		}
		if msg.Data == "" {
			return true, nil
		}
		_, err := ptyFile.Write([]byte(msg.Data))
		return true, err
	default:
		return true, nil
	}
}

func firstNonSpaceByteIndex(b []byte) int {
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			return i
		}
	}
	return -1
}

func parseTerminalServerID(r *http.Request) (uint, error) {
	rawID := strings.TrimSpace(r.PathValue("server_id"))
	if rawID == "" {
		rawID = strings.TrimSpace(r.PathValue("id"))
	}
	if rawID == "" {
		rawID = strings.TrimPrefix(strings.TrimSpace(r.URL.Path), "/terminal/")
		if idx := strings.IndexByte(rawID, '/'); idx >= 0 {
			rawID = rawID[:idx]
		}
	}

	parsed, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("invalid server id")
	}
	return uint(parsed), nil
}

func selectShell() string {
	envShell := strings.TrimSpace(os.Getenv("SHELL"))
	if envShell != "" {
		if _, err := os.Stat(envShell); err == nil {
			return envShell
		}
	}
	if _, err := os.Stat("/bin/bash"); err == nil {
		return "/bin/bash"
	}
	return "/bin/sh"
}

func stopTerminalProcess(cmd *exec.Cmd, done <-chan struct{}) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	select {
	case <-done:
		return
	default:
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func isExpectedWSClose(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived)
}

func writeTerminalError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
