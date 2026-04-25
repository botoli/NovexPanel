# WebSocket Protocol

This document describes message contracts for both websocket channels:

- `site/ws`: browser client <-> backend
- `agent/ws`: backend <-> agent

Conventions:

- JSON encoding is used for all messages.
- Field names in payloads are snake_case unless stated otherwise.
- Unknown message types should be ignored.
- Required fields are marked as `required`.

## 1. Endpoints And Auth

### 1.1 Site channel

- URL: `wss://<backend>/site/ws?token=<jwt>`
- Auth: user JWT (same token returned by login)

### 1.2 Agent channel

- URL: `wss://<backend>/agent/ws?token=<agent_token>&name=<optional_server_name>`
- Auth: agent token

## 2. Generic Envelope And Types

Most messages are event-like and include `type`.

Example:

```json
{
  "type": "subscribe_metrics",
  "server_id": 12
}
```

### 2.1 Common scalar types

- `server_id`: integer
- `deploy_id`: integer
- `session_id`: string (uuid)
- `pid`: integer
- `rows`: integer
- `cols`: integer
- `line`: string
- `data`: string

## 3. Site <-> Backend (`site/ws`)

## 3.1 Client -> Server Events

### `subscribe_metrics`

Subscribe current client to metrics stream for a server.

Required fields:

- `type` (string, required): `subscribe_metrics`
- `server_id` (integer, required)

Example:

```json
{
  "type": "subscribe_metrics",
  "server_id": 12
}
```

### `unsubscribe_metrics`

Unsubscribe metrics stream.

Required fields:

- `type` (string, required): `unsubscribe_metrics`
- `server_id` (integer, required)

### `open_terminal`

Open terminal session on server.

Required fields:

- `type` (string, required): `open_terminal`
- `server_id` (integer, required)

Optional fields:

- `rows` (integer, optional, default `24`)
- `cols` (integer, optional, default `80`)

Example:

```json
{
  "type": "open_terminal",
  "server_id": 12,
  "rows": 40,
  "cols": 120
}
```

### `terminal_input`

Send input to active or explicit terminal session.

Required fields:

- `type` (string, required): `terminal_input`
- `data` (string, required)

Optional fields:

- `server_id` (integer, optional if `session_id` provided)
- `session_id` (string, optional if active terminal exists)

Example:

```json
{
  "type": "terminal_input",
  "server_id": 12,
  "session_id": "1f4820c8-fd15-4f54-9e4f-3fec1f16f502",
  "data": "ls -la\n"
}
```

### `terminal_resize`

Resize terminal session.

Required fields:

- `type` (string, required): `terminal_resize`
- `rows` (integer, required)
- `cols` (integer, required)

Optional fields:

- `server_id` (integer, optional if `session_id` provided)
- `session_id` (string, optional if active terminal exists)

### `close_terminal`

Close active terminal session for server.

Required fields:

- `type` (string, required): `close_terminal`
- `server_id` (integer, required)

### `subscribe_deploy_logs`

Subscribe to realtime deploy log stream.

Required fields:

- `type` (string, required): `subscribe_deploy_logs`
- `deploy_id` (integer, required)

Example:

```json
{
  "type": "subscribe_deploy_logs",
  "deploy_id": 452
}
```

Compatibility note:

- `type: "deploy_logs"` is accepted as a legacy alias for subscribe.

### `unsubscribe_deploy_logs`

Unsubscribe from realtime deploy log stream.

Required fields:

- `type` (string, required): `unsubscribe_deploy_logs`
- `deploy_id` (integer, required)

Example:

```json
{
  "type": "unsubscribe_deploy_logs",
  "deploy_id": 452
}
```

## 3.2 Server -> Client Events

### `metrics`

Canonical metrics push event for subscribed server.

Required fields:

- `type` (string, required): `metrics`
- `server_id` (integer, required)
- `data` (object, required)

Example:

```json
{
  "type": "metrics",
  "server_id": 12,
  "data": {
    "cpu": { "usage": 23.4, "cores": 8 },
    "ram": { "percent": 51.7 },
    "disk": { "percent": 67.1, "read_speed": 20480, "write_speed": 10240 },
    "network": { "rx_speed": 145000, "tx_speed": 92000 }
  }
}
```

### `terminal_output`

Terminal stream output chunk.

Required fields:

- `type` (string, required): `terminal_output`
- `server_id` (integer, required)
- `session_id` (string, required)
- `data` (string, required)

### `terminal_closed`

Terminal closure confirmation.

Required fields:

- `type` (string, required): `terminal_closed`
- `server_id` (integer, required)

### `deploy_log_line`

Single realtime deploy log line (new canonical format).

Required fields:

- `type` (string, required): `deploy_log_line`
- `deploy_id` (integer, required)
- `line` (string, required)
- `stream` (string, required): `stdout` or `stderr`
- `timestamp` (string, required): RFC3339 UTC timestamp

Example:

```json
{
  "type": "deploy_log_line",
  "deploy_id": 452,
  "line": "Cloning into 'src'...",
  "stream": "stdout",
  "timestamp": "2026-04-25T12:00:00Z"
}
```

Compatibility note:

- Backend also emits legacy event `deploy_log` with `is_error` for older clients.

### `deploy_log` (legacy)

Single deploy log line (legacy compatibility event).

Required fields:

- `type` (string, required): `deploy_log`
- `deploy_id` (integer, required)
- `line` (string, required)
- `is_error` (boolean, required)

### `deploy_complete`

Deploy terminal event.

Required fields:

- `type` (string, required): `deploy_complete`
- `deploy_id` (integer, required)
- `success` (boolean, required)
- `url` (string, required for success)

Optional fields:

- `error` (string, optional)

Success example:

```json
{
  "type": "deploy_complete",
  "deploy_id": 452,
  "success": true,
  "url": "http://10.0.0.11:32789"
}
```

Error example:

```json
{
  "type": "deploy_complete",
  "deploy_id": 452,
  "success": false,
  "url": "",
  "error": "npm run build failed: exit status 1"
}
```

### `error`

Validation or routing error.

Required fields:

- `type` (string, required): `error`
- `error` (string, required)

### `subscribed_deploy_logs`

Subscribe confirmation from backend.

Required fields:

- `type` (string, required): `subscribed_deploy_logs`
- `deploy_id` (integer, required)

### `unsubscribed_deploy_logs`

Unsubscribe confirmation from backend.

Required fields:

- `type` (string, required): `unsubscribed_deploy_logs`
- `deploy_id` (integer, required)

## 4. Backend <-> Agent (`agent/ws`)

Backend can send direct event payloads and command envelopes. Agent must support both.

## 4.1 Server -> Agent Events

### `deploy`

Start deploy flow.

Required fields:

- `type` (string, required): `deploy`
- `deploy_id` (integer, required)
- `repo_url` (string, required for github source)

Optional fields:

- `source` (string, optional, default `github`, values: `github`, `zip`)
- `zip_data` (string, optional, required when `source=zip`)
- `branch` (string, optional, default `main`)
- `project_type` / `type` (string, optional)
- `subdirectory` (string, optional)
- `build_command` (string, optional)
- `output_dir` (string, optional)
- `env` / `envVars` (object, optional)

Example:

```json
{
  "type": "deploy",
  "deploy_id": 452,
  "source": "github",
  "repo_url": "https://github.com/acme/marketing-site.git",
  "branch": "main",
  "project_type": "vite",
  "subdirectory": "web",
  "output_dir": "dist",
  "env": {
    "NODE_ENV": "production"
  }
}
```

### `stop_deploy`

Stop and cleanup deploy runtime.

Required fields:

- `type` (string, required): `stop_deploy`
- `deploy_id` (integer, required)

### `kill_process`

Kill process by PID.

Required fields:

- `type` (string, required): `kill_process`
- `pid` (integer, required)

### `run_terminal`

Open terminal session on agent.

Required fields:

- `type` (string, required): `run_terminal`
- `session_id` (string, required)

Optional fields:

- `rows` (integer, optional)
- `cols` (integer, optional)

### `terminal_resize`

Resize existing terminal session.

Required fields:

- `type` (string, required): `terminal_resize`
- `session_id` (string, required)
- `rows` (integer, required)
- `cols` (integer, required)

### `terminal_close`

Close terminal session.

Required fields:

- `type` (string, required): `terminal_close`
- `session_id` (string, required)

## 4.2 Command Envelope (Backend -> Agent)

In request/response flows backend may wrap commands:

```json
{
  "type": "command",
  "command": "deploy",
  "request_id": "5f5f8bb7-f363-4c18-a4a0-14e12ee15e88",
  "payload": {
    "deploy_id": 452,
    "repo_url": "https://github.com/acme/marketing-site.git"
  }
}
```

Required envelope fields:

- `type` (string, required): `command`
- `command` (string, required)
- `request_id` (string, required)
- `payload` (object, optional for commands without body)

## 4.3 Agent -> Server Events

### `deploy_log`

Single deploy log line.

Required fields:

- `type` (string, required): `deploy_log`
- `deploy_id` (integer, required)
- `line` (string, required)
- `stream` (string, required): `stdout` or `stderr`
- `timestamp` (string, required): RFC3339 UTC timestamp

Compatibility note:

- Agent may also send `is_error` (or `isError`) instead of `stream`; backend maps it automatically.

Example:

```json
{
  "type": "deploy_log",
  "deploy_id": 452,
  "line": "npm run build",
  "stream": "stderr",
  "timestamp": "2026-04-25T12:00:00Z"
}
```

### `deploy_result`

Detailed deploy completion event.

Required fields:

- `type` (string, required): `deploy_result`
- `deployId` or `deploy_id` (integer, required)
- `status` (string, required; `success` or `error`)
- `url` (string, required, empty on failure)
- `port` (integer, optional)
- `log` (string, optional)
- `error` (string, optional)

Success example:

```json
{
  "type": "deploy_result",
  "deployId": 452,
  "status": "success",
  "url": "http://10.0.0.11:32789",
  "port": 32789,
  "log": "...",
  "error": ""
}
```

Error example:

```json
{
  "type": "deploy_result",
  "deployId": 452,
  "status": "error",
  "url": "",
  "port": 0,
  "log": "npm run build",
  "error": "package.json is missing scripts.build"
}
```

### `metrics`

Periodic metrics push from agent.

Required fields:

- `type` (string, required): `metrics`
- `data` (object, required)

### `terminal_output`

Terminal output chunk.

Required fields:

- `type` (string, required): `terminal_output`
- `session_id` (string, required)
- `data` (string, required)

## 4.4 Command Response (Agent -> Server)

When backend sends command envelope, agent responds with:

```json
{
  "type": "command_response",
  "request_id": "5f5f8bb7-f363-4c18-a4a0-14e12ee15e88",
  "success": true,
  "data": {
    "accepted": true
  }
}
```

Error response example:

```json
{
  "type": "command_response",
  "request_id": "5f5f8bb7-f363-4c18-a4a0-14e12ee15e88",
  "success": false,
  "error": "deploy_id is required"
}
```

Required fields:

- `type` (string, required): `command_response`
- `request_id` (string, required)
- `success` (boolean, required)

Optional fields:

- `data` (object)
- `error` (string)
