# WebSocket Protocol

## 1) Agent <-> Backend

Connection:

`wss://<backend>/agent/ws?token=<agent_token>&name=<server_name_optional>`

### Messages Backend -> Agent

Command envelope:

```json
{
  "type": "command",
  "command": "run_command",
  "request_id": "uuid",
  "payload": { "command": "ls -la" }
}
```

Supported `command` values:

- `ping`
- `get_metrics`
- `run_command`
- `run_terminal`
- `get_processes`
- `kill_process`
- `deploy`

Terminal stream:

```json
{ "type": "terminal_input", "session_id": "uuid", "data": "ls\n" }
{ "type": "terminal_close", "session_id": "uuid" }
```

### Messages Agent -> Backend

Command response:

```json
{
  "type": "command_response",
  "request_id": "uuid",
  "success": true,
  "data": { "stdout": "...", "stderr": "..." }
}
```

Metrics push every 2s:

```json
{ "type": "metrics", "data": { "cpu": { "usage": 12.5 } } }
```

Terminal output:

```json
{ "type": "terminal_output", "session_id": "uuid", "data": "user@server:~$ " }
```

Deploy log and complete:

```json
{ "type": "deploy_log", "deploy_id": 10, "line": "Cloning repository...", "is_error": false }
{ "type": "deploy_complete", "deploy_id": 10, "success": true, "url": "http://1.2.3.4:3000", "error": "" }
```

## 2) Site <-> Backend

Connection:

`wss://<backend>/site/ws?token=<JWT>`

### Messages Site -> Backend

Metrics subscriptions:

```json
{ "type": "subscribe_metrics", "server_id": 123 }
{ "type": "unsubscribe_metrics", "server_id": 123 }
```

Terminal:

```json
{ "type": "open_terminal", "server_id": 123, "rows": 24, "cols": 80 }
{ "type": "terminal_input", "server_id": 123, "data": "ls\n" }
{ "type": "close_terminal", "server_id": 123 }
```

Deploy logs:

```json
{ "type": "deploy_logs", "deploy_id": 456 }
```

### Messages Backend -> Site

```json
{ "type": "metrics", "server_id": 123, "data": { "cpu": { "usage": 45 } } }
{ "type": "terminal_output", "server_id": 123, "session_id": "uuid", "data": "..." }
{ "type": "deploy_log", "deploy_id": 456, "line": "Installing deps", "is_error": false }
{ "type": "deploy_complete", "deploy_id": 456, "success": true, "url": "http://1.2.3.4:3000" }
{ "type": "error", "error": "server not found" }
```
